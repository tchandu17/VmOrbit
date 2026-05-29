package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ─────────────────────────────────────────────────────────────────────────────
// ProviderManager
// ─────────────────────────────────────────────────────────────────────────────

// ManagerConfig controls reconnect and health-check behaviour.
type ManagerConfig struct {
	// HealthCheckInterval is how often connected providers are pinged.
	// Zero disables periodic health checks.
	HealthCheckInterval time.Duration
	// ReconnectBackoff is the wait between reconnect attempts.
	ReconnectBackoff time.Duration
	// MaxReconnectAttempts is the number of reconnect tries before giving up.
	// Zero means unlimited.
	MaxReconnectAttempts int
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		HealthCheckInterval:  30 * time.Second,
		ReconnectBackoff:     10 * time.Second,
		MaxReconnectAttempts: 5,
	}
}

// connectionEntry tracks a live provider connection.
type connectionEntry struct {
	provider port.Provider
	creds    port.Credentials
	mu       sync.Mutex
	attempts int
}

// Manager wraps the Registry and adds:
//   - connect-on-demand (lazy connection per hypervisor)
//   - periodic health checks with automatic reconnect
//   - capability-aware operation dispatch
//   - console session routing to ConsoleProvider
type Manager struct {
	registry *Registry
	cfg      ManagerConfig
	log      logger.Logger

	mu          sync.RWMutex
	connections map[string]*connectionEntry // key: hypervisorID
}

// NewManager creates a Manager backed by the given registry.
func NewManager(registry *Registry, cfg ManagerConfig, log logger.Logger) *Manager {
	return &Manager{
		registry:    registry,
		cfg:         cfg,
		log:         log,
		connections: make(map[string]*connectionEntry),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Connection management
// ─────────────────────────────────────────────────────────────────────────────

// Connect establishes a connection for the given hypervisor and stores the
// credentials for future reconnect attempts.
func (m *Manager) Connect(ctx context.Context, h *model.Hypervisor, creds port.Credentials) error {
	p, err := m.registry.Get(h.Provider)
	if err != nil {
		return fmt.Errorf("manager connect: %w", err)
	}

	if err := p.Connect(ctx, creds); err != nil {
		return fmt.Errorf("manager connect [%s]: %w", h.ID, err)
	}

	m.mu.Lock()
	m.connections[h.ID.String()] = &connectionEntry{
		provider: p,
		creds:    creds,
	}
	m.mu.Unlock()

	m.log.Info("provider connected",
		logger.String("hypervisor_id", h.ID.String()),
		logger.String("provider", string(h.Provider)),
		logger.String("host", creds.Host),
	)
	return nil
}

// Disconnect tears down the connection for a hypervisor.
func (m *Manager) Disconnect(ctx context.Context, hypervisorID string) error {
	m.mu.Lock()
	entry, ok := m.connections[hypervisorID]
	if ok {
		delete(m.connections, hypervisorID)
	}
	m.mu.Unlock()

	if !ok {
		return nil
	}

	if err := entry.provider.Disconnect(ctx); err != nil {
		return fmt.Errorf("manager disconnect [%s]: %w", hypervisorID, err)
	}
	m.log.Info("provider disconnected", logger.String("hypervisor_id", hypervisorID))
	return nil
}

// connectable is a narrow interface for checking connection state.
// BaseProvider satisfies this; the manager uses it to avoid depending on the
// concrete type while still being able to check liveness.
type connectable interface {
	IsConnected() bool
}

// GetProvider returns the connected provider for a hypervisor.
// Returns ErrNotConnected if no connection has been established.
func (m *Manager) GetProvider(hypervisorID string) (port.Provider, error) {
	m.mu.RLock()
	entry, ok := m.connections[hypervisorID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("hypervisor %q: %w", hypervisorID, ErrNotConnected)
	}

	// Check liveness if the concrete provider exposes IsConnected.
	if c, ok := entry.provider.(connectable); ok && !c.IsConnected() {
		return nil, fmt.Errorf("hypervisor %q: %w", hypervisorID, ErrNotConnected)
	}

	return entry.provider, nil
}

// GetConsoleProvider returns the ConsoleProvider for a hypervisor, or an error
// if the provider does not support console sessions.
func (m *Manager) GetConsoleProvider(hypervisorID string) (port.ConsoleProvider, error) {
	p, err := m.GetProvider(hypervisorID)
	if err != nil {
		return nil, err
	}

	if !p.Capabilities().Console {
		return nil, fmt.Errorf("hypervisor %q: %w", hypervisorID, ErrUnsupported)
	}

	cp, ok := p.(port.ConsoleProvider)
	if !ok {
		// Capability flag is set but interface not implemented — programming error.
		return nil, fmt.Errorf("hypervisor %q: provider advertises Console capability but does not implement ConsoleProvider", hypervisorID)
	}
	return cp, nil
}

// Capabilities returns the capability set for a connected provider.
func (m *Manager) Capabilities(hypervisorID string) (port.ProviderCapabilities, error) {
	p, err := m.GetProvider(hypervisorID)
	if err != nil {
		return port.ProviderCapabilities{}, err
	}
	return p.Capabilities(), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Health checks & reconnect
// ─────────────────────────────────────────────────────────────────────────────

// StartHealthChecks launches a background goroutine that pings all connected
// providers on the configured interval and reconnects on failure.
// Cancel ctx to stop.
func (m *Manager) StartHealthChecks(ctx context.Context) {
	if m.cfg.HealthCheckInterval == 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(m.cfg.HealthCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.checkAll(ctx)
			}
		}
	}()
}

func (m *Manager) checkAll(ctx context.Context) {
	m.mu.RLock()
	ids := make([]string, 0, len(m.connections))
	for id := range m.connections {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		m.checkOne(ctx, id)
	}
}

func (m *Manager) checkOne(ctx context.Context, hypervisorID string) {
	m.mu.RLock()
	entry, ok := m.connections[hypervisorID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	if err := entry.provider.Ping(ctx); err == nil {
		entry.mu.Lock()
		entry.attempts = 0
		entry.mu.Unlock()
		return
	}

	m.log.Warn("provider health check failed, attempting reconnect",
		logger.String("hypervisor_id", hypervisorID),
	)
	m.reconnect(ctx, hypervisorID, entry)
}

func (m *Manager) reconnect(ctx context.Context, hypervisorID string, entry *connectionEntry) {
	entry.mu.Lock()
	defer entry.mu.Unlock()

	if m.cfg.MaxReconnectAttempts > 0 && entry.attempts >= m.cfg.MaxReconnectAttempts {
		m.log.Error("provider reconnect limit reached, giving up",
			logger.String("hypervisor_id", hypervisorID),
			logger.Int("attempts", entry.attempts),
		)
		return
	}

	entry.attempts++

	select {
	case <-ctx.Done():
		return
	case <-time.After(m.cfg.ReconnectBackoff):
	}

	if err := entry.provider.Connect(ctx, entry.creds); err != nil {
		m.log.Error("provider reconnect failed",
			logger.String("hypervisor_id", hypervisorID),
			logger.Int("attempt", entry.attempts),
			logger.Error(err),
		)
		return
	}

	entry.attempts = 0
	m.log.Info("provider reconnected",
		logger.String("hypervisor_id", hypervisorID),
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Capability-aware dispatch helpers
// ─────────────────────────────────────────────────────────────────────────────

// RequireCapability returns an error if the provider for hypervisorID does not
// have the named capability. Use this before calling optional sub-interfaces.
//
//	if err := mgr.RequireCapability(hID, func(c port.ProviderCapabilities) bool {
//	    return c.Console
//	}, "console"); err != nil { ... }
func (m *Manager) RequireCapability(
	hypervisorID string,
	check func(port.ProviderCapabilities) bool,
	capName string,
) error {
	caps, err := m.Capabilities(hypervisorID)
	if err != nil {
		return err
	}
	if !check(caps) {
		return fmt.Errorf("hypervisor %q does not support %s: %w", hypervisorID, capName, ErrUnsupported)
	}
	return nil
}
