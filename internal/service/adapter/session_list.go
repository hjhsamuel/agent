package adapter

import (
	"context"
	"fmt"

	"github.com/hjhsamuel/agent/internal/store"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"google.golang.org/adk/v2/session"
)

type sessionList struct {
	records []schema.ADKSession
}

func (l *sessionList) userIDs() []string {
	seen := make(map[string]bool)
	var ids []string
	for _, doc := range l.records {
		if !seen[doc.UserID] {
			seen[doc.UserID] = true
			ids = append(ids, doc.UserID)
		}
	}
	return ids
}

func (l *sessionList) load(ctx context.Context, db *store.Store, req *session.ListRequest) ([]session.Session, error) {
	docs, err := db.ListADKSessions(ctx, req.AppName, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	l.records = docs
	sessions := make([]session.Session, 0, len(l.records))
	if len(l.records) == 0 {
		return sessions, nil
	}
	appState, err := db.GetADKAppState(ctx, req.AppName)
	if err != nil {
		return nil, fmt.Errorf("get app state: %w", err)
	}
	userStates, err := db.GetADKUserStates(ctx, req.AppName, l.userIDs())
	if err != nil {
		return nil, fmt.Errorf("get user states: %w", err)
	}
	for _, doc := range l.records {
		sess, err := newLocalSession(&doc, appState, userStates[doc.UserID])
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}
