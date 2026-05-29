package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/vmOrbit/backend/internal/config"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/infrastructure/cache"
	"github.com/vmOrbit/backend/pkg/logger"
)

const refreshTokenCachePrefix = "refresh_token:"

type authService struct {
	users  port.UserRepository
	cache  cache.Cache
	cfg    config.JWTConfig
	log    logger.Logger
}

// NewAuthService creates a new authentication service.
func NewAuthService(users port.UserRepository, perms port.PermissionRepository, c cache.Cache, cfg config.JWTConfig, log logger.Logger) port.AuthService {
	return &authService{users: users, cache: c, cfg: cfg, log: log}
}

func (s *authService) Login(ctx context.Context, email, password string) (*port.TokenPair, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if !user.IsActive {
		return nil, fmt.Errorf("account is disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	now := time.Now().UTC()
	user.LastLoginAt = &now
	_ = s.users.Update(ctx, user)

	return s.generateTokenPair(ctx, user)
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	return s.cache.Delete(ctx, refreshTokenCachePrefix+refreshToken)
}

func (s *authService) RefreshTokens(ctx context.Context, refreshToken string) (*port.TokenPair, error) {
	var userID string
	if err := s.cache.Get(ctx, refreshTokenCachePrefix+refreshToken, &userID); err != nil {
		return nil, fmt.Errorf("invalid or expired refresh token")
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Rotate: invalidate old token
	_ = s.cache.Delete(ctx, refreshTokenCachePrefix+refreshToken)

	return s.generateTokenPair(ctx, user)
}

func (s *authService) ValidateAccessToken(_ context.Context, tokenStr string) (*port.Claims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.Secret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	roles, _ := claims["roles"].([]interface{})
	roleStrs := make([]string, 0, len(roles))
	for _, r := range roles {
		if rs, ok := r.(string); ok {
			roleStrs = append(roleStrs, rs)
		}
	}

	perms, _ := claims["permissions"].([]interface{})
	permStrs := make([]string, 0, len(perms))
	for _, p := range perms {
		if ps, ok := p.(string); ok {
			permStrs = append(permStrs, ps)
		}
	}

	return &port.Claims{
		UserID:      fmt.Sprintf("%v", claims["sub"]),
		Username:    fmt.Sprintf("%v", claims["username"]),
		Roles:       roleStrs,
		Permissions: permStrs,
	}, nil
}

func (s *authService) generateTokenPair(ctx context.Context, user *model.User) (*port.TokenPair, error) {
	// Build role list
	roles := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		roles = append(roles, r.Name)
	}

	// Build permission list ("resource:action") from DB for accurate claims
	userPerms, _ := s.users.GetPermissions(ctx, user.ID.String())
	permStrs := make([]string, 0, len(userPerms))
	for _, p := range userPerms {
		permStrs = append(permStrs, p.Resource+":"+p.Action)
	}

	// Access token
	accessClaims := jwt.MapClaims{
		"sub":         user.ID.String(),
		"username":    user.Username,
		"email":       user.Email,
		"roles":       roles,
		"permissions": permStrs,
		"iss":         s.cfg.Issuer,
		"iat":         time.Now().Unix(),
		"exp":         time.Now().Add(s.cfg.AccessTokenTTL).Unix(),
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return nil, fmt.Errorf("signing access token: %w", err)
	}

	// Refresh token (opaque, stored in Redis)
	refreshToken := uuid.New().String()
	if err := s.cache.Set(ctx, refreshTokenCachePrefix+refreshToken, user.ID.String(), s.cfg.RefreshTokenTTL); err != nil {
		return nil, fmt.Errorf("storing refresh token: %w", err)
	}

	return &port.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
