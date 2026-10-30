package a2ax

import (
	"context"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/remoteagent/v2"
	"google.golang.org/adk/v2/auth"
)

func NewA2A(path string, cred auth.CredentialProvider) (agent.Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	card, err := agentcard.DefaultResolver.Resolve(ctx, path)
	if err != nil {
		return nil, err
	}

	return remoteagent.NewA2A(remoteagent.A2AConfig{
		Name:        card.Name,
		Description: card.Description,
		AgentCard:   card,
		Auth:        cred,
		MessageSendConfig: &a2a.SendMessageConfig{
			ReturnImmediately: true,
		},
	})
}
