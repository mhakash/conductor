// Package events fans run events out to live subscribers (SSE clients).
package events

import (
	"sync"

	"conductor/internal/run"
)

// bufferSize is how many events a slow subscriber may lag behind before it is
// dropped. A dropped client reconnects and starts again from a snapshot.
const bufferSize = 512

type subscriber struct {
	ch chan run.Event
}

// Broker is an in-process pub/sub keyed by run ID. Publishing never blocks.
type Broker struct {
	mu   sync.Mutex
	subs map[string]map[*subscriber]struct{}
}

func NewBroker() *Broker {
	return &Broker{subs: map[string]map[*subscriber]struct{}{}}
}

var _ run.Publisher = (*Broker)(nil)

// Subscribe returns a channel of events for runID. The channel is closed when
// the run ends, when the subscriber falls too far behind, or when cancel is
// called.
func (b *Broker) Subscribe(runID string) (<-chan run.Event, func()) {
	s := &subscriber{ch: make(chan run.Event, bufferSize)}
	b.mu.Lock()
	if b.subs[runID] == nil {
		b.subs[runID] = map[*subscriber]struct{}{}
	}
	b.subs[runID][s] = struct{}{}
	b.mu.Unlock()

	return s.ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.remove(runID, s)
	}
}

func (b *Broker) Publish(runID string, e run.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs[runID] {
		select {
		case s.ch <- e:
		default:
			b.remove(runID, s)
		}
	}
}

func (b *Broker) End(runID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs[runID] {
		select {
		case s.ch <- run.Event{Type: run.EventEnd}:
		default:
		}
		b.remove(runID, s)
	}
}

// remove must be called with b.mu held. It is idempotent.
func (b *Broker) remove(runID string, s *subscriber) {
	subs := b.subs[runID]
	if _, ok := subs[s]; !ok {
		return
	}
	delete(subs, s)
	close(s.ch)
	if len(subs) == 0 {
		delete(b.subs, runID)
	}
}

// Nop discards events; for code paths without live subscribers.
type Nop struct{}

func (Nop) Publish(string, run.Event) {}
func (Nop) End(string)                {}
