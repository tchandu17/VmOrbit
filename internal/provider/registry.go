package provider

import (
	"fmt"
	"sync"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// Registry holds all registered provider factories.
// New providers are added by calling Register — no other code changes needed.
type Registry struct {
	mu        sync.RWMutex
	providers map[model.ProviderType]port.Provider
	log       logger.Logger
}

// NewRegistry creates an empty provider registry.
func NewRegistry(log logger.Logger) *Registry {
	return &Registry{
		providers: make(map[model.ProviderType]port.Provider),
		log:       log,
	}
}

// Register adds a provider to the registry.
// Panics on duplicate registration to catch wiring mistakes at startup.
func (r *Registry) Register(p port.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[p.Type()]; exists {
		panic(fmt.Sprintf("provider %q already registered", p.Type()))
	}
	r.providers[p.Type()] = p
	r.log.Info("provider registered", logger.String("type", string(p.Type())), logger.String("name", p.Name()))
}

// Get returns the provider for the given type.
func (r *Registry) Get(t model.ProviderType) (port.Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[t]
	if !ok {
		return nil, fmt.Errorf("provider %q not registered", t)
	}
	return p, nil
}

// List returns all registered provider types.
func (r *Registry) List() []model.ProviderType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]model.ProviderType, 0, len(r.providers))
	for t := range r.providers {
		types = append(types, t)
	}
	return types
}
