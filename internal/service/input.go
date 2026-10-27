package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/plugin"
	"google.golang.org/genai"
)

func waitInputsReady(inputs []*schema.WaitInput) bool {
	if len(inputs) == 0 {
		return false
	}
	for _, input := range inputs {
		if input == nil || input.ID == "" || input.Content == "" {
			return false
		}
	}
	return true
}

// recoveryInput uses only business state; the runner owns ADK history.
func (s *Service) recoveryInput(ctx context.Context, obj *schema.Session) (*genai.Content, error) {
	if obj.State != schema.SessionInputReady {
		return nil, nil
	}
	if len(obj.Wait) != 0 {
		if !waitInputsReady(obj.Wait) {
			return nil, errors.New("incomplete submitted confirmation input")
		}
		return buildInputRequiredMessage(obj.Wait), nil
	}
	message, err := s.store.GetTurnUserMessage(obj.ID, obj.TurnSeq-1)
	if err != nil {
		return nil, err
	}
	if message.Content == "" {
		return nil, errors.New("missing user input")
	}
	return genai.NewContentFromText(message.Content, genai.RoleUser), nil
}

// BeforeAgent runs after input persistence, even when the agent emits no output.
// Business persistence stays outside SessionService.
func inputStatePlugin(markRunning func() error) (*plugin.Plugin, error) {
	var mu sync.Mutex
	var marked atomic.Bool
	return plugin.New(plugin.Config{
		Name: "business_input_state",
		BeforeAgentCallback: func(ctx agent.Context) (*genai.Content, error) {
			// Most callbacks arrive after the first agent has committed input.
			if marked.Load() || ctx.UserContent() == nil {
				return nil, nil
			}
			mu.Lock()
			defer mu.Unlock()
			// Another callback may have committed while we waited for the lock.
			if marked.Load() {
				return nil, nil
			}
			if err := markRunning(); err != nil {
				return nil, err
			}
			marked.Store(true)
			return nil, nil
		},
	})
}
