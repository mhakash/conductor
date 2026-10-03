package events

import (
	"testing"

	"conductor/internal/run"
)

func drain(ch <-chan run.Event) []run.EventType {
	var out []run.EventType
	for e := range ch {
		out = append(out, e.Type)
	}
	return out
}

func TestPublishAndEnd(t *testing.T) {
	b := NewBroker()
	ch, cancel := b.Subscribe("r1")
	defer cancel()
	other, cancelOther := b.Subscribe("r2")
	defer cancelOther()

	b.Publish("r1", run.Event{Type: run.EventStep})
	b.Publish("r1", run.Event{Type: run.EventLogs})
	b.End("r1")

	got := drain(ch)
	if len(got) != 3 || got[0] != run.EventStep || got[2] != run.EventEnd {
		t.Errorf("events = %v", got)
	}
	select {
	case e := <-other:
		t.Errorf("r2 subscriber received %v", e.Type)
	default:
	}
}

func TestSlowSubscriberIsDropped(t *testing.T) {
	b := NewBroker()
	ch, cancel := b.Subscribe("r1")
	defer cancel()

	for range bufferSize + 1 {
		b.Publish("r1", run.Event{Type: run.EventLogs})
	}
	if n := len(drain(ch)); n != bufferSize {
		t.Errorf("received %d events before close, want %d", n, bufferSize)
	}
}

func TestCancelIsIdempotent(t *testing.T) {
	b := NewBroker()
	_, cancel := b.Subscribe("r1")
	cancel()
	cancel()
	b.End("r1") // must not panic on a closed channel
}
