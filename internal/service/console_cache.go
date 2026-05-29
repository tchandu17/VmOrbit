package service

import (
	"sync"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// consoleSessionCache is a short-lived in-memory store for console sessions.
// It is the primary lookup path — the DB is secondary (for persistence across restarts).
// Entries are evicted after their ExpiresAt time plus a small grace period.
type consoleSessionCache struct {
	mu      sync.RWMutex
	entries map[string]*model.ConsoleSession // keyed by SessionToken
}

var globalConsoleCache = &consoleSessionCache{
	entries: make(map[string]*model.ConsoleSession),
}

func (c *consoleSessionCache) set(session *model.ConsoleSession) {
	c.mu.Lock()
	c.entries[session.SessionToken] = session
	c.mu.Unlock()

	// Schedule eviction after expiry + 30s grace
	ttl := time.Until(session.ExpiresAt) + 30*time.Second
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	go func() {
		time.Sleep(ttl)
		c.mu.Lock()
		delete(c.entries, session.SessionToken)
		c.mu.Unlock()
	}()
}

func (c *consoleSessionCache) get(token string) (*model.ConsoleSession, bool) {
	c.mu.RLock()
	s, ok := c.entries[token]
	c.mu.RUnlock()
	return s, ok
}
