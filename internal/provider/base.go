package provider

import (
	"context"
	"sync"

	"github.com/vmOrbit/backend/internal/domain/port"
)

// BaseProvider provides shared connection-state management that concrete
// providers can embed to avoid boilerplate.
type BaseProvider struct {
	mu          sync.RWMutex
	connected   bool
	credentials port.Credentials
}

// IsConnected returns true if the provider has an active connection.
func (b *BaseProvider) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// SetConnected updates the connection state.
func (b *BaseProvider) SetConnected(v bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.connected = v
}

// Credentials returns the stored credentials (read-only copy).
func (b *BaseProvider) Credentials() port.Credentials {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.credentials
}

// StoreCredentials saves credentials after a successful Connect.
func (b *BaseProvider) StoreCredentials(creds port.Credentials) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.credentials = creds
}

// EnsureConnected returns an error if the provider is not connected.
func (b *BaseProvider) EnsureConnected(_ context.Context) error {
	if !b.IsConnected() {
		return ErrNotConnected
	}
	return nil
}
