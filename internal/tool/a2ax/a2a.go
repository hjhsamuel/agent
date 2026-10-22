package a2ax

import (
	"context"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/remoteagent/v2"
	"google.golang.org/adk/v2/session"
)

func ResolveCard(ctx context.Context, path string) (*a2a.AgentCard, error) {
	return agentcard.DefaultResolver.Resolve(ctx, path)
}

func NewA2A(card *a2a.AgentCard) (agent.Agent, error) {
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
