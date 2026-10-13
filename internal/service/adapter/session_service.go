package adapter

import (
	"context"
	"fmt"

	"github.com/hjhsamuel/agent/internal/store"
	"google.golang.org/adk/v2/session"
)

type SessionService struct {
	store *store.Store
}

func (s *SessionService) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	if req == nil || req.AppName == "" || req.UserID == "" {
		return nil, fmt.Errorf("app_name and user_id are required")
	}
	sess := new(localSession)
	if err := sess.create(ctx, s.store, req); err != nil {
		return nil, err
	}
	return &session.CreateResponse{Session: sess}, nil
}

func (s *SessionService) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	if req == nil || req.AppName == "" || req.UserID == "" || req.SessionID == "" {
		return nil, fmt.Errorf("app_name, user_id and session_id are required")
	}
	if req.NumRecentEvents < 0 {
		return nil, fmt.Errorf("num_recent_events must not be negative")
	}
	sess := new(localSession)
	if err := sess.load(ctx, s.store, req); err != nil {
		return nil, err
	}
	return &session.GetResponse{Session: sess}, nil
}

func (s *SessionService) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) {
	if req == nil || req.AppName == "" {
		return nil, fmt.Errorf("app_name is required")
	}
	sessions, err := new(sessionList).load(ctx, s.store, req)
	if err != nil {
		return nil, err
	}
	return &session.ListResponse{Sessions: sessions}, nil
}

func (s *SessionService) Delete(ctx context.Context, req *session.DeleteRequest) error {
	if req == nil || req.AppName == "" || req.UserID == "" || req.SessionID == "" {
		return fmt.Errorf("app_name, user_id and session_id are required")
	}
	if err := s.store.DeleteADKSession(ctx, req.AppName, req.UserID, req.SessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *SessionService) AppendEvent(ctx context.Context, sess session.Session, event *session.Event) error {
	if sess == nil || event == nil {
		return fmt.Errorf("session and event are required")
	}
	if event.Partial {
		return nil
	}
	local, ok := sess.(*localSession)
	if !ok || local == nil {
		return fmt.Errorf("unexpected session type %T", sess)
	}
	return local.appendEvent(ctx, s.store, event)
}

func NewSessionService(s *store.Store) session.Service {
	return &SessionService{store: s}
}
