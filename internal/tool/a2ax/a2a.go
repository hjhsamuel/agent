package a2ax

import (
	"context"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/remoteagent/v2"
	"google.golang.org/adk/v2/session"
)

func NewA2A(path string) (agent.Agent, error) {
	card, err := agentcard.DefaultResolver.Resolve(context.Background(), path)
	if err != nil {
		return nil, err
	}

	return remoteagent.NewA2A(remoteagent.A2AConfig{
		Name:        card.Name,
		Description: card.Description,
		AgentCard:   card,
		BeforeRequestCallbacks: []remoteagent.BeforeA2ARequestCallback{
			func(ctx agent.Context, req *a2a.SendMessageRequest) (*session.Event, error) {
				req.Message.SetMeta("User", ctx.UserID())
				return nil, nil
			},
		},
	})
}
