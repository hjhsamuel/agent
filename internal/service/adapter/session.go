package adapter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hjhsamuel/agent/internal/store"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/adk/v2/platform"
	"google.golang.org/adk/v2/session"
)

type localSession struct {
	appendMu sync.Mutex
	mu       sync.RWMutex
	doc      schema.ADKSession
	state    *localState
	events   *localEvents
}

func newLocalSession(doc *schema.ADKSession, appState, userState schema.ADKState) (*localSession, error) {
	sess := new(localSession)
	if err := sess.restore(doc, appState, userState); err != nil {
		return nil, err
	}
	return sess, nil
}

// restore initializes an unpublished session. State and events own their data
// and synchronization; callers interact with them through methods only.
func (s *localSession) restore(doc *schema.ADKSession, appState, userState schema.ADKState) error {
	state, err := newLocalState(doc.State, appState, userState)
	if err != nil {
		return err
	}
	s.doc = *doc
	s.doc.State = nil // Only the version and identity are needed for writes.
	s.state = state
	s.events = new(localEvents)
	return nil
}

func (s *localSession) create(ctx context.Context, db *store.Store, req *session.CreateRequest) error {
	appDelta, userDelta, state, err := stateDelta(req.State).encode()
	if err != nil {
		return err
	}
	id := req.SessionID
	if id == "" {
		id = platform.NewUUID(ctx)
	}
	now := platform.Now(ctx).UTC().Truncate(time.Millisecond)
	doc := &schema.ADKSession{
		ID: bson.NewObjectID(), AppName: req.AppName, UserID: req.UserID, SessionID: id,
		State: state, CreatedAt: now, UpdatedAt: now,
	}
	appState, userState, err := db.CreateADKSession(ctx, doc, appDelta, userDelta)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return s.restore(doc, appState, userState)
}

func (s *localSession) load(ctx context.Context, db *store.Store, req *session.GetRequest) error {
	doc, err := db.GetADKSession(ctx, req.AppName, req.UserID, req.SessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}
	appState, err := db.GetADKAppState(ctx, req.AppName)
	if err != nil {
		return fmt.Errorf("get app state: %w", err)
	}
	userStates, err := db.GetADKUserStates(ctx, req.AppName, []string{req.UserID})
	if err != nil {
		return fmt.Errorf("get user state: %w", err)
	}
	if err := s.restore(doc, appState, userStates[req.UserID]); err != nil {
		return err
	}
	if doc.Version == 0 {
		return nil
	}
	storedEvents, err := db.GetADKSessionEvents(ctx, doc, req.After, req.NumRecentEvents)
	if err != nil {
		return fmt.Errorf("get session events: %w", err)
	}
	return s.events.restore(storedEvents)
}

func (s *localSession) ID() string             { return s.doc.SessionID }
func (s *localSession) AppName() string        { return s.doc.AppName }
func (s *localSession) UserID() string         { return s.doc.UserID }
func (s *localSession) State() session.State   { return s.state }
func (s *localSession) Events() session.Events { return s.events.snapshot() }

func (s *localSession) LastUpdateTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.doc.UpdatedAt
}

func (s *localSession) storageSnapshot() schema.ADKSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.doc
}

func (s *localSession) appendEvent(ctx context.Context, db *store.Store, event *session.Event) error {
	// Serialize appends without holding the metadata/state locks during I/O.
	s.appendMu.Lock()
	defer s.appendMu.Unlock()
	prepared := newSessionEvent(ctx, event)
	appDelta, userDelta, sessionDelta, err := prepared.stateDelta().encode()
	if err != nil {
		return err
	}
	doc := s.storageSnapshot()
	stored, err := prepared.storageRecord(&doc)
	if err != nil {
		return err
	}
	if err := db.AppendADKSessionEvent(ctx, &doc, stored, appDelta, userDelta, sessionDelta); err != nil {
		return fmt.Errorf("append session event: %w", err)
	}
	s.commitEvent(prepared, stored)
	return nil
}

func (s *localSession) commitEvent(event *sessionEvent, stored *schema.ADKSessionEvent) {
	// Only mutate the local session and caller event after the DB commit.
	s.mu.Lock()
	s.state.apply(event.stateDelta())
	s.events.append(event.value())
	s.doc.Version = stored.Sequence
	s.doc.UpdatedAt = stored.Timestamp
	s.mu.Unlock()
	event.commit()
}

var _ session.Session = (*localSession)(nil)
