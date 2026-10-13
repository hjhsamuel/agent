package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"slices"
	"sync"
	"time"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/adk/v2/platform"
	"google.golang.org/adk/v2/session"
)

// sessionEvent prepares a persistent copy and updates the source only on commit.
type sessionEvent struct {
	source    *session.Event
	processed session.Event
}

func newSessionEvent(ctx context.Context, source *session.Event) *sessionEvent {
	e := &sessionEvent{source: source, processed: *source}
	e.processed.Actions.StateDelta = e.stateDelta().persistent()
	e.processed.Timestamp = source.Timestamp.UTC().Truncate(time.Millisecond)
	if e.processed.Timestamp.IsZero() {
		e.processed.Timestamp = platform.Now(ctx).UTC().Truncate(time.Millisecond)
	}
	if e.processed.ID == "" {
		e.processed.ID = platform.NewUUID(ctx)
	}
	return e
}

func (e *sessionEvent) stateDelta() stateDelta { return stateDelta(e.source.Actions.StateDelta) }
func (e *sessionEvent) value() *session.Event  { return &e.processed }

func (e *sessionEvent) storageRecord(doc *schema.ADKSession) (*schema.ADKSessionEvent, error) {
	payload, err := json.Marshal(&e.processed)
	if err != nil {
		return nil, fmt.Errorf("encode session event: %w", err)
	}
	return &schema.ADKSessionEvent{
		ID: bson.NewObjectID(), Session: doc.ID, EventID: e.processed.ID,
		Sequence: doc.Version + 1, Timestamp: e.processed.Timestamp, Payload: payload,
	}, nil
}

func (e *sessionEvent) commit() {
	e.source.ID = e.processed.ID
	e.source.Timestamp = e.processed.Timestamp
	e.source.Actions.StateDelta = e.processed.Actions.StateDelta
}

type localEvents struct {
	mu     sync.RWMutex
	values []*session.Event
}

func (e *localEvents) restore(stored []schema.ADKSessionEvent) error {
	values := make([]*session.Event, 0, len(stored))
	// MongoDB returns recent events first; restore chronological order.
	for i := len(stored) - 1; i >= 0; i-- {
		var event session.Event
		if err := json.Unmarshal(stored[i].Payload, &event); err != nil {
			return fmt.Errorf("decode session event: %w", err)
		}
		values = append(values, &event)
	}
	e.mu.Lock()
	e.values = values
	e.mu.Unlock()
	return nil
}

func (e *localEvents) append(event *session.Event) {
	e.mu.Lock()
	e.values = append(e.values, event)
	e.mu.Unlock()
}

func (e *localEvents) snapshot() eventSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return eventSnapshot(slices.Clone(e.values))
}

// eventSnapshot has its own immutable slice, so iteration needs no locks or copies.
type eventSnapshot []*session.Event

func (e eventSnapshot) All() iter.Seq[*session.Event] { return slices.Values(e) }
func (e eventSnapshot) Len() int                      { return len(e) }
func (e eventSnapshot) At(i int) *session.Event {
	if i < 0 || i >= len(e) {
		return nil
	}
	return e[i]
}

var _ session.Events = eventSnapshot(nil)
