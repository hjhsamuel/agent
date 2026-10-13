package adapter

import (
	"context"

	"github.com/hjhsamuel/agent/internal/store"
	"google.golang.org/adk/v2/session"
)

type SessionService struct {
	store *store.Store
}

func (s *SessionService) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *SessionService) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *SessionService) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *SessionService) Delete(ctx context.Context, req *session.DeleteRequest) error {
	//TODO implement me
	panic("implement me")
}

func (s *SessionService) AppendEvent(ctx context.Context, sess session.Session, event *session.Event) error {
	//TODO implement me
	panic("implement me")
}

func NewSessionService(s *store.Store) session.Service {
	return &SessionService{store: s}
}
