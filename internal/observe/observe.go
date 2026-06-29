package observe

import (
	"sync"
	"time"
)

type SessionEvent struct {
	SessionID string      `json:"session_id"`
	Type      string      `json:"type"` // "action", "observation", "status"
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan SessionEvent
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan SessionEvent),
	}
}

func (b *EventBus) Publish(event SessionEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	chans := b.subscribers[event.SessionID]
	for _, ch := range chans {
		select {
		case ch <- event:
		default:
			// drop if full
		}
	}
}

func (b *EventBus) Subscribe(sessionID string) (<-chan SessionEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan SessionEvent, 32)
	b.subscribers[sessionID] = append(b.subscribers[sessionID], ch)

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		chans := b.subscribers[sessionID]
		for i, c := range chans {
			if c == ch {
				b.subscribers[sessionID] = append(chans[:i], chans[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, unsubscribe
}

func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, chans := range b.subscribers {
		for _, ch := range chans {
			close(ch)
		}
	}
	b.subscribers = make(map[string][]chan SessionEvent)
}
