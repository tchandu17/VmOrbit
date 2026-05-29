package database

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/vmOrbit/backend/internal/config"
	"github.com/vmOrbit/backend/internal/domain/model"
)

// NewPostgresDB opens a GORM connection to PostgreSQL and configures the
// connection pool according to cfg.
func NewPostgresDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		// Disable automatic plural table names — we control names via GORM tags.
		// NamingStrategy is left at default (snake_case plural) which matches the SQL migration.
	})
	if err != nil {
		return nil, fmt.Errorf("opening postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("getting sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return db, nil
}

// AutoMigrate runs GORM auto-migration for all domain models.
//
// Ordering matters: tables with foreign keys must come after the tables they
// reference. GORM will CREATE TABLE IF NOT EXISTS and ADD COLUMN IF NOT EXISTS,
// but it will NOT drop columns or change column types — use the versioned SQL
// migration in migrations/001_initial_schema.sql for structural changes.
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		// Auth & RBAC
		&model.User{},
		&model.Role{},
		&model.Permission{},
		&model.RefreshToken{},

		// Hypervisor hierarchy
		&model.HypervisorGroup{},
		&model.Hypervisor{},
		&model.DataStore{},
		&model.Network{},

		// VM inventory
		&model.VMTemplate{},
		&model.VM{},
		&model.Snapshot{},

		// Tagging
		&model.Tag{},
		&model.VMTag{},

		// Async operations
		&model.Task{},
		&model.TaskLog{},

		// Observability (append-only — no soft-delete)
		&model.AuditLog{},
		&model.WebSocketEvent{},

		// Provider health monitoring
		&model.ProviderHealth{},
		&model.ProviderHealthHistory{},

		// Console sessions
		&model.ConsoleSession{},

		// Event & notification engine
		&model.PlatformEvent{},
		&model.NotificationChannel{},
		&model.NotificationRule{},
		&model.NotificationHistory{},

		// Scheduling & automation
		&model.Schedule{},
		&model.ScheduleExecution{},
		&model.Workflow{},
		&model.WorkflowAction{},
		&model.WorkflowRun{},

		// Provisioning & cloning
		&model.ProvisioningJob{},

		// Environment & stack orchestration
		&model.Environment{},
		&model.EnvironmentVM{},
		&model.EnvironmentTag{},
		&model.VMDependency{},
		&model.OrchestrationRun{},
		&model.OrchestrationStep{},

		// Analytics & capacity planning
		&model.InfrastructureMetrics{},
		&model.CapacityHistory{},
		&model.ProviderCapacity{},
		&model.VMUsageStats{},
		&model.OptimizationRecommendation{},
		&model.RecommendationHistory{},

		// Policy & Approval governance
		&model.Policy{},
		&model.PolicyCondition{},
		&model.PolicyAssignment{},
		&model.PolicyViolation{},
		&model.ApprovalRequest{},
		&model.ApprovalStep{},
		&model.ApprovalHistory{},

		// Infrastructure hierarchy
		&model.Cluster{},
		&model.Host{},
	); err != nil {
		return err
	}

	if err := runDataPatches(db); err != nil {
		return err
	}

	// Create composite and partial performance indexes that GORM cannot
	// express via struct tags. These are idempotent (IF NOT EXISTS).
	return CreatePerformanceIndexes(db)
}

// runDataPatches applies one-time idempotent data fixes after schema migration.
func runDataPatches(db *gorm.DB) error {
	// Fix: hypervisors registered with tls_verify=true (old default) that have
	// never successfully connected. Self-signed certs are the norm for on-prem
	// hypervisors — flip them to false automatically so they can connect.
	return db.Exec(`
		UPDATE hypervisors
		SET    tls_verify = false
		WHERE  provider IN ('proxmox', 'vmware', 'esxi', 'kvm')
		  AND  tls_verify = true
		  AND  connection_status IN ('unknown', 'error')
		  AND  deleted_at IS NULL
	`).Error
}
