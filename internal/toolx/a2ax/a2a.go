package a2ax

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/remoteagent/v2"
	"google.golang.org/adk/v2/auth"
	"google.golang.org/adk/v2/server/adka2a/v2"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool/toolconfirmation"
	"google.golang.org/genai"
)

func NewA2A(path string, cred auth.CredentialProvider) (agent.Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	// 默认使用不鉴权接口获取 AgentCard
	card, err := agentcard.DefaultResolver.Resolve(ctx, path)
	if err != nil {
		return nil, err
	}

	// 将 skill 描述配置在 agent 描述中，尽量保证具备完整的输入信息
	var b strings.Builder
	b.WriteString(card.Description)
	b.WriteString("\n\nAvailable skills:\n")
	for _, item := range card.Skills {
		_, _ = fmt.Fprintf(&b, "- %s: %s\n", item.Name, item.Description)
	}

	// 由于 remoteagent 内部没有实现 GetTask 轮询，因此目前强制使用 SSE 方式
	return remoteagent.NewA2A(remoteagent.A2AConfig{
		Name:                   card.Name,
		Description:            b.String(),
		AgentCard:              card,
		Auth:                   cred,
		GenAIPartConverter:     compactContextPart,
		BeforeRequestCallbacks: []remoteagent.BeforeA2ARequestCallback{resumeTextInput},
		AfterRequestCallbacks:  []remoteagent.AfterA2ARequestCallback{requestTextInput},
		MessageSendConfig: &a2a.SendMessageConfig{
			ReturnImmediately: true,
		},
	})
}

// ADK wraps other agents' events as user context before converting parts.
// Keep their useful output, but omit internal routing calls and boilerplate.
func compactContextPart(_ context.Context, event *session.Event, part *genai.Part) (*a2a.Part, error) {
	if event.Content != nil && len(event.Content.Parts) > 0 && event.Content.Parts[0] != nil && event.Content.Parts[0].Text == "For context:" {
		if part == event.Content.Parts[0] {
			return nil, nil
		}
		if strings.HasPrefix(part.Text, "[") {
			if end := strings.Index(part.Text, "] "); end >= 0 {
				body := part.Text[end+2:]
				if strings.HasPrefix(body, "called tool transfer_to_agent with parameters:") {
					return nil, nil
				}
				if strings.HasPrefix(body, "transfer_to_agent tool returned result:") {
					return nil, nil
				}
				if text, ok := strings.CutPrefix(body, "said: "); ok {
					return a2a.NewTextPart(part.Text[:end+1] + " " + text), nil
				}
			}
		}
	}
	return adka2a.ToA2APart(part, event.LongRunningToolIDs)
}

// Register a real pending call before the runner persists the remote event.
// This lets the runner route a later FunctionResponse back to this agent.
func requestTextInput(_ agent.Context, _ *a2a.SendMessageRequest, event *session.Event, err error) (*session.Event, error) {
	if err != nil || event == nil {
		return nil, err
	}
	response, _ := event.CustomMetadata[adka2a.ToADKMetaKey("response")].(map[string]any)
	status, _ := response["status"].(map[string]any)
	if status["state"] != string(a2a.TaskStateInputRequired) {
		return nil, nil
	}
	var hint strings.Builder
	if event.Content != nil {
		for _, part := range event.Content.Parts {
			if part == nil {
				continue
			}
			if part.FunctionCall != nil {
				return nil, nil
			}
			if !part.Thought {
				hint.WriteString(part.Text)
			}
		}
	} else {
		event.Content = &genai.Content{Role: "model"}
	}
	id := "a2a-text:" + event.ID
	event.Content.Parts = append(event.Content.Parts, &genai.Part{FunctionCall: &genai.FunctionCall{
		ID: id, Name: toolconfirmation.FunctionCallName,
		Args: map[string]any{"toolConfirmation": map[string]any{"hint": hint.String()}},
	}})
	event.LongRunningToolIDs = append(event.LongRunningToolIDs, id)
	return event, nil
}

// ADK resolves the pending call to its peer and restores TaskID/contextID.
// Only translate our synthetic confirmation back to the server's text protocol.
func resumeTextInput(ctx agent.Context, req *a2a.SendMessageRequest) (*session.Event, error) {
	content := ctx.UserContent()
	if content == nil || req.Message.TaskID == "" {
		return nil, nil
	}
	for _, part := range content.Parts {
		if part == nil || part.FunctionResponse == nil {
			continue
		}
		fr := part.FunctionResponse
		if !strings.HasPrefix(fr.ID, "a2a-text:") {
			continue
		}
		answer, ok := fr.Response["payload"].(string)
		if !ok {
			return nil, fmt.Errorf("A2A input request %s has no text answer", fr.ID)
		}
		req.Message.Parts = []*a2a.Part{a2a.NewTextPart(answer)}
		return nil, nil
	}
	return nil, nil
}
