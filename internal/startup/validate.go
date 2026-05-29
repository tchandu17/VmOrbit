// Package startup provides pre-flight validation checks that run before
// the application starts serving traffic. These checks ensure all required
// configuration, dependencies, and migrations are in place.
package startup

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vmOrbit/backend/pkg/logger"
	"gorm.io/gorm"
)

// ValidationResult holds the outcome of a single check.
type ValidationResult struct {
	Name    string
	Passed  bool
	Message string
}

// Validator runs startup validation checks.
type Validator struct {
	log     logger.Logger
	results []ValidationResult
}

// NewValidator creates a new startup validator.
func NewValidator(log logger.Logger) *Validator {
	return &Validator{log: log}
}

// ValidateAll runs all pre-flight checks and returns an error if any critical
// check fails. Warnings are logged but do not prevent startup.
func (v *Validator) ValidateAll(db *gorm.DB, redisClient *redis.Client) error {
	v.log.Info("Running startup validation checks...")

	v.validateEnvironment()
	v.validateDatabase(db)
	v.validateRedis(redisClient)
	v.validateEncryptionKey()
	v.validateJWTSecret()

	// Report results
	var failures []string
	for _, r := range v.results {
		if r.Passed {
			v.log.Info("  ✓ "+r.Name, logger.String("detail", r.Message))
		} else {
			v.log.Error("  ✗ "+r.Name, logger.String("detail", r.Message))
			failures = append(failures, r.Name+": "+r.Message)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("startup validation failed: %s", strings.Join(failures, "; "))
	}

	v.log.Info("All startup validation checks passed")
	return nil
}

func (v *Validator) addResult(name string, passed bool, msg string) {
	v.results = append(v.results, ValidationResult{Name: name, Passed: passed, Message: msg})
}

func (v *Validator) validateEnvironment() {
	// Check required environment variables
	required := []string{
		"VMORBIT_DATABASE_HOST",
		"VMORBIT_REDIS_HOST",
	}

	// These are critical in production mode
	mode := os.Getenv("VMORBIT_SERVER_MODE")
	if mode == "release" {
		required = append(required,
			"VMORBIT_JWT_SECRET",
			"VMORBIT_ENCRYPTION_KEY",
		)
	}

	missing := []string{}
	for _, env := range required {
		if os.Getenv(env) == "" {
			missing = append(missing, env)
		}
	}

	if len(missing) > 0 {
		if mode == "release" {
			v.addResult("Environment variables", false,
				"Missing required vars: "+strings.Join(missing, ", "))
		} else {
			// In development, missing vars are just warnings
			v.addResult("Environment variables", true,
				"Missing vars (dev mode, using defaults): "+strings.Join(missing, ", "))
		}
	} else {
		v.addResult("Environment variables", true, "All required variables set")
	}
}

func (v *Validator) validateDatabase(db *gorm.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.WithContext(ctx).Exec("SELECT 1").Error; err != nil {
		v.addResult("Database connectivity", false, err.Error())
		return
	}

	// Check that migrations have run (look for a known table)
	var count int64
	if err := db.WithContext(ctx).Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'users'").Scan(&count).Error; err != nil {
		v.addResult("Database migrations", false, "Cannot check migration status: "+err.Error())
		return
	}

	if count == 0 {
		v.addResult("Database migrations", false, "Users table not found — migrations may not have run")
		return
	}

	v.addResult("Database connectivity", true, "Connected and migrations verified")
}

func (v *Validator) validateRedis(client *redis.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		v.addResult("Redis connectivity", false, err.Error())
		return
	}

	// Check Redis info for memory usage
	info, err := client.Info(ctx, "memory").Result()
	if err == nil && strings.Contains(info, "used_memory_human") {
		v.addResult("Redis connectivity", true, "Connected")
	} else {
		v.addResult("Redis connectivity", true, "Connected (memory info unavailable)")
	}
}

func (v *Validator) validateEncryptionKey() {
	key := os.Getenv("VMORBIT_ENCRYPTION_KEY")
	mode := os.Getenv("VMORBIT_SERVER_MODE")

	if key == "" && mode == "release" {
		v.addResult("Encryption key", false,
			"VMORBIT_ENCRYPTION_KEY not set — provider credentials cannot be encrypted")
		return
	}

	if key != "" && len(key) != 64 {
		v.addResult("Encryption key", false,
			fmt.Sprintf("VMORBIT_ENCRYPTION_KEY should be 64 hex chars (got %d)", len(key)))
		return
	}

	if key == "" {
		v.addResult("Encryption key", true, "Using development key (non-production)")
	} else {
		v.addResult("Encryption key", true, "Configured (AES-256)")
	}
}

func (v *Validator) validateJWTSecret() {
	secret := os.Getenv("VMORBIT_JWT_SECRET")
	mode := os.Getenv("VMORBIT_SERVER_MODE")

	if secret == "" && mode == "release" {
		v.addResult("JWT secret", false, "VMORBIT_JWT_SECRET not set in production mode")
		return
	}

	if secret != "" && len(secret) < 32 {
		v.addResult("JWT secret", false,
			fmt.Sprintf("JWT secret too short (%d chars) — use at least 64 chars", len(secret)))
		return
	}

	if secret == "" {
		v.addResult("JWT secret", true, "Using default (non-production)")
	} else {
		v.addResult("JWT secret", true, "Configured")
	}
}
