package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/provider"
	"github.com/vmOrbit/backend/pkg/logger"
)

type consoleService struct {
	consoleSessions port.ConsoleSessionRepository
	vms             port.VMRepository
	hypervisors     port.HypervisorRepository
	registry        *provider.Registry
	audit           port.AuditService
	log             logger.Logger
}

// NewConsoleService creates a new ConsoleService.
func NewConsoleService(
	consoleSessions port.ConsoleSessionRepository,
	vms port.VMRepository,
	hypervisors port.HypervisorRepository,
	registry *provider.Registry,
	audit port.AuditService,
	log logger.Logger,
) port.ConsoleService {
	return &consoleService{
		consoleSessions: consoleSessions,
		vms:             vms,
		hypervisors:     hypervisors,
		registry:        registry,
		audit:           audit,
		log:             log,
	}
}

// RequestSession acquires a provider console ticket, persists a session record,
// and returns the session ready for the frontend to use.
func (s *consoleService) RequestSession(ctx context.Context, vmID string, opts port.ConsoleOptions) (*model.ConsoleSession, error) {
	vm, err := s.vms.GetByID(ctx, vmID)
	if err != nil {
		return nil, fmt.Errorf("vm not found: %w", err)
	}

	h, err := s.hypervisors.GetByID(ctx, vm.HypervisorID.String())
	if err != nil {
		return nil, fmt.Errorf("hypervisor not found: %w", err)
	}

	p, err := s.registry.Get(h.Provider)
	if err != nil {
		return nil, err
	}

	if !p.Capabilities().Console {
		return nil, fmt.Errorf("provider %q does not support console sessions", h.Provider)
	}

	cp, ok := p.(port.ConsoleProvider)
	if !ok {
		return nil, fmt.Errorf("provider %q advertises Console capability but does not implement ConsoleProvider", h.Provider)
	}

	creds, err := buildHypervisorCredentials(h)
	if err != nil {
		return nil, fmt.Errorf("building credentials: %w", err)
	}
	if err := p.Connect(ctx, creds); err != nil {
		if isTLSError(err) {
			return nil, fmt.Errorf("TLS certificate verification failed — set tls_verify=false for self-signed certificates: %w", err)
		}
		return nil, fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(ctx) //nolint:errcheck

	providerSession, err := cp.GetConsoleSession(ctx, vm.ProviderVMID, opts)
	if err != nil {
		return nil, fmt.Errorf("provider GetConsoleSession: %w", err)
	}
	if providerSession == nil {
		return nil, fmt.Errorf("provider returned nil console session")
	}
	if providerSession.URL == "" {
		return nil, fmt.Errorf("provider returned console session with empty URL (type=%s)", providerSession.Type)
	}

	s.log.Info("console: provider session acquired",
		logger.String("vm_id", vmID),
		logger.String("provider_vm_id", vm.ProviderVMID),
		logger.String("console_type", string(providerSession.Type)),
		logger.String("url", providerSession.URL),
		logger.String("ticket_len", fmt.Sprintf("%d", len(providerSession.Ticket))),
	)

	// Generate a random opaque session token for the frontend to reference.
	token, err := generateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("generating session token: %w", err)
	}

	expiresAt := providerSession.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(5 * time.Minute)
	}

	userID := callerUUIDPtr(ctx)
	hypervisorUUID := vm.HypervisorID

	// Merge provider host/port into Extra so the proxy handler can use them.
	extra := model.JSONMap{}
	for k, v := range providerSession.Extra {
		extra[k] = v
	}
	if providerSession.Host != "" {
		extra["host"] = providerSession.Host
	}
	if providerSession.Port != 0 {
		extra["port"] = providerSession.Port
	}
	if providerSession.Ticket != "" {
		extra["ticket"] = providerSession.Ticket
	}

	session := &model.ConsoleSession{
		ID:             uuid.New(),
		VMID:           vm.ID,
		HypervisorID:   hypervisorUUID,
		UserID:         userID,
		Provider:       h.Provider,
		ConsoleType:    string(providerSession.Type),
		SessionToken:   token,
		ProviderTicket: providerSession.Ticket,
		ConsoleURL:     providerSession.URL,
		Status:         model.ConsoleSessionStatusActive,
		ExpiresAt:      expiresAt,
		CreatedAt:      time.Now().UTC(),
		Extra:          extra,
	}

	// Store in memory immediately — the proxy WS connection arrives before the
	// async DB write completes, so the cache is the primary lookup path.
	globalConsoleCache.set(session)

	// Persist the session record asynchronously — a DB failure must never
	// prevent the user from getting their console URL.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Warn("panic persisting console session (table may not exist — run migrations)",
					logger.String("vm_id", vmID),
					logger.String("panic", fmt.Sprintf("%v", r)),
				)
			}
		}()
		if err := s.consoleSessions.Create(context.Background(), session); err != nil {
			s.log.Warn("failed to persist console session record",
				logger.String("vm_id", vmID),
				logger.String("error", err.Error()),
			)
		}
	}()
	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionExecute,
		Resource:    "vm",
		ResourceID:  vmID,
		Description: fmt.Sprintf("console session requested (type=%s)", providerSession.Type),
		Success:     true,
	})

	s.log.Info("console session created",
		logger.String("vm_id", vmID),
		logger.String("provider", string(h.Provider)),
		logger.String("console_type", string(providerSession.Type)),
		logger.String("session_id", session.ID.String()),
	)

	return session, nil
}

// GetSession looks up an active session by its opaque token.
// Checks the in-memory cache first (for freshly-created sessions), then DB.
func (s *consoleService) GetSession(ctx context.Context, token string) (*model.ConsoleSession, error) {
	// 1. Check in-memory cache first — covers the case where the proxy WS
	//    arrives before the async DB write completes.
	if cached, ok := globalConsoleCache.get(token); ok {
		if cached.IsExpired() {
			return nil, fmt.Errorf("console session has expired")
		}
		if cached.Status != model.ConsoleSessionStatusActive {
			return nil, fmt.Errorf("console session is %s", cached.Status)
		}
		return cached, nil
	}

	// 2. Fall back to DB.
	session, err := s.consoleSessions.GetByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	if session.IsExpired() {
		_ = s.consoleSessions.UpdateStatus(ctx, session.ID.String(), model.ConsoleSessionStatusExpired)
		return nil, fmt.Errorf("console session has expired")
	}

	if session.Status != model.ConsoleSessionStatusActive {
		return nil, fmt.Errorf("console session is %s", session.Status)
	}

	return session, nil
}

// generateSessionToken returns a 32-byte cryptographically random hex string.
func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// callerUUIDPtr returns the caller's UUID from context, or nil if not present.
// This is an alias for callerUUID which already returns *uuid.UUID.
func callerUUIDPtr(ctx context.Context) *uuid.UUID {
	return callerUUID(ctx)
}
