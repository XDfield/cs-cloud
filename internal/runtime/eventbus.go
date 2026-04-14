package runtime

import (
	"sync"

	"cs-cloud/internal/agent"
)

type SubFilter struct {
	Backend string
}

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[chan agent.Event]*SubFilter
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[chan agent.Event]*SubFilter),
	}
}

func (b *EventBus) Subscribe(filter *SubFilter) chan agent.Event {
	ch := make(chan agent.Event, 64)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[ch] = filter
	return ch
}

func (b *EventBus) Unsubscribe(ch chan agent.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscribers, ch)
	close(ch)
}

func (b *EventBus) Emit(event agent.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch, filter := range b.subscribers {
		if filter != nil && filter.Backend != "" && event.Backend != filter.Backend {
			continue
		}
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *EventBus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
