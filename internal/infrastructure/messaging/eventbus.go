package messaging

import (
	"context"
	"sync"
)

// EventType identifies the kind of event.
type EventType string

const (
	EventVMStatusChanged        EventType = "vm.status_changed"
	EventTaskStatusChanged      EventType = "task.status_changed"
	EventTaskProgress           EventType = "task.progress"
	EventTaskLogAppended        EventType = "task.log_appended"
	EventTaskCancelled          EventType = "task.cancelled"
	EventHypervisorConnected    EventType = "hypervisor.connected"
	EventHypervisorDisconnected EventType = "hypervisor.disconnected"
	EventInventorySynced        EventType = "inventory.synced"
	// EventPlatformEvent is published whenever a PlatformEvent is dispatched.
	// The WebSocket hub routes it to the "events" room.
	EventPlatformEvent EventType = "platform.event"
)

// Event is a message published on the event bus.
type Event struct {
	Type    EventType
	Payload interface{}
}

// Handler is a function that processes an event.
type Handler func(ctx context.Context, event Event)

// EventBus is the publish/subscribe contract.
type EventBus interface {
	Publish(ctx context.Context, event Event)
	Subscribe(eventType EventType, handler Handler) (unsubscribe func())
}

// InMemoryEventBus is a simple in-process fan-out event bus.
// For multi-instance deployments, replace with a Redis Pub/Sub or NATS adapter.
type InMemoryEventBus struct {
	mu       sync.RWMutex
	handlers map[EventType]map[int]Handler
	nextID   int
}

// NewInMemoryEventBus creates a new in-memory event bus.
func NewInMemoryEventBus() *InMemoryEventBus {
	return &InMemoryEventBus{
		handlers: make(map[EventType]map[int]Handler),
	}
}

// Publish fans out the event to all registered handlers asynchronously.
func (b *InMemoryEventBus) Publish(ctx context.Context, event Event) {
	b.mu.RLock()
	handlers := make([]Handler, 0)
	if hs, ok := b.handlers[event.Type]; ok {
		for _, h := range hs {
			handlers = append(handlers, h)
		}
	}
	b.mu.RUnlock()

	for _, h := range handlers {
		h := h
		go h(ctx, event)
	}
}

// Subscribe registers a handler and returns an unsubscribe function.
func (b *InMemoryEventBus) Subscribe(eventType EventType, handler Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.handlers[eventType] == nil {
		b.handlers[eventType] = make(map[int]Handler)
	}
	id := b.nextID
	b.nextID++
	b.handlers[eventType][id] = handler

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.handlers[eventType], id)
	}
}
