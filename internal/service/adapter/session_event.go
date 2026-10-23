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
	"google.golang.org/genai"
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
	event := &e.processed
	record := &schema.ADKSessionEvent{
		ID:                      bson.NewObjectID(),
		Session:                 doc.ID,
		EventID:                 event.ID,
		Sequence:                doc.Version + 1,
		Timestamp:               event.Timestamp,
		InvocationID:            event.InvocationID,
		Author:                  event.Author,
		Branch:                  event.Branch,
		IsolationScope:          event.IsolationScope,
		LongRunningToolIDs:      slices.Clone(event.LongRunningToolIDs),
		Routes:                  slices.Clone(event.Routes),
		Partial:                 event.Partial,
		TurnComplete:            event.TurnComplete,
		Interrupted:             event.Interrupted,
		ErrorCode:               event.ErrorCode,
		ErrorMessage:            event.ErrorMessage,
		ModelVersion:            event.ModelVersion,
		SessionResumptionHandle: event.SessionResumptionHandle,
		FinishReason:            string(event.FinishReason),
		AvgLogprobs:             event.AvgLogprobs,
	}
	for _, field := range eventJSONFields(event, record) {
		data, err := json.Marshal(field.value)
		if err != nil {
			return nil, fmt.Errorf("encode session event %s: %w", field.name, err)
		}
		if string(data) != "null" {
			*field.storage = data
		}
	}
	return record, nil
}

// Share the complex-field mapping between encoding and decoding so both paths
// preserve the same data, including Actions' custom JSON representation.
func eventJSONFields(event *session.Event, record *schema.ADKSessionEvent) []struct {
	name    string
	value   any
	storage *[]byte
} {
	return []struct {
		name    string
		value   any
		storage *[]byte
	}{
		{"actions", &event.Actions, &record.Actions},
		{"content", &event.Content, &record.Content},
		{"grounding_metadata", &event.GroundingMetadata, &record.GroundingMetadata},
		{"custom_metadata", &event.CustomMetadata, &record.CustomMetadata},
		{"usage_metadata", &event.UsageMetadata, &record.UsageMetadata},
		{"citation_metadata", &event.CitationMetadata, &record.CitationMetadata},
		{"logprobs_result", &event.LogprobsResult, &record.LogprobsResult},
		{"input_transcription", &event.InputTranscription, &record.InputTranscription},
		{"output_transcription", &event.OutputTranscription, &record.OutputTranscription},
		{"output", &event.Output, &record.Output},
		{"node_info", &event.NodeInfo, &record.NodeInfo},
		{"requested_input", &event.RequestedInput, &record.RequestedInput},
	}
}

func eventFromStorage(record *schema.ADKSessionEvent) (*session.Event, error) {
	event := &session.Event{}
	event.ID, event.Timestamp = record.EventID, record.Timestamp
	event.InvocationID, event.Author = record.InvocationID, record.Author
	event.Branch, event.IsolationScope = record.Branch, record.IsolationScope
	event.LongRunningToolIDs, event.Routes = slices.Clone(record.LongRunningToolIDs), slices.Clone(record.Routes)
	event.Partial, event.TurnComplete, event.Interrupted = record.Partial, record.TurnComplete, record.Interrupted
	event.ErrorCode, event.ErrorMessage = record.ErrorCode, record.ErrorMessage
	event.ModelVersion, event.SessionResumptionHandle = record.ModelVersion, record.SessionResumptionHandle
	event.FinishReason, event.AvgLogprobs = genai.FinishReason(record.FinishReason), record.AvgLogprobs
	for _, field := range eventJSONFields(event, record) {
		if len(*field.storage) == 0 {
			continue
		}
		if err := json.Unmarshal(*field.storage, field.value); err != nil {
			return nil, fmt.Errorf("decode session event %s: %w", field.name, err)
		}
	}
	return event, nil
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
		event, err := eventFromStorage(&stored[i])
		if err != nil {
			return err
		}
		values = append(values, event)
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
