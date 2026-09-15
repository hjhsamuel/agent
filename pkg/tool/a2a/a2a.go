package a2a

import (
	"context"
	"fmt"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"github.com/hjhsamuel/agent/pkg/tool"
)

func GetA2ATools(addr string) ([]tool.Tool, error) {
	card, err := agentcard.DefaultResolver.Resolve(context.Background(), addr)
	if err != nil {
		return nil, err
	}

	client, err := a2aclient.NewFromCard(context.Background(), card)
	if err != nil {
		return nil, err
	}

	name := formatAgentName(card.Name)

	out := make([]tool.Tool, 0)
	for _, skill := range card.Skills {
		out = append(out, NewA2A(
			fmt.Sprintf("%s:%s:%s", tool.A2ATool, name, skill.ID),
			skill.Description,
			client,
		))
	}
	return out, nil
}

func formatAgentName(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] != ' ' && name[i] != '-' {
			continue
		}
		var b strings.Builder
		b.Grow(len(name))
		b.WriteString(name[:i])

		for ; i < len(name); i++ {
			switch name[i] {
			case ' ', '-':
				b.WriteByte('_')
			default:
				b.WriteByte(name[i])
			}
		}
		return b.String()
	}
	return name
}
