package resolver

import (
	"encoding/json"

	"github.com/hjhsamuel/agent/pkg/provider"
)

type InputRequiredAction string

const (
	ProvideInput InputRequiredAction = "provide_input"
	AskUser      InputRequiredAction = "ask_user"
)

type InputRequiredResult struct {
	Results []*InputRequiredDecision `json:"results"`
}

type InputRequiredDecision struct {
	ToolCallId string              `json:"tool_call_id"`
	Action     InputRequiredAction `json:"action"`
	Input      map[string]any      `json:"input"`
	Question   string              `json:"question"`
}

type InputRequiredPayload struct {
	Status      string         `json:"status"`
	Message     string         `json:"message"`
	InputSchema map[string]any `json:"input_schema"`
}

func buildInputRequiredContent(
	message string,
	inputSchema map[string]any,
) (string, error) {
	payload := InputRequiredPayload{
		Status:      "input_required",
		Message:     message,
		InputSchema: inputSchema,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func BuildInputRequiredMessage(
	toolCallId string,
	message string,
	inputSchema map[string]any,
) (*provider.Message, error) {
	content, err := buildInputRequiredContent(message, inputSchema)
	if err != nil {
		return nil, err
	}
	return &provider.Message{
		Role:       provider.RoleTool,
		Content:    content,
		ToolCallId: toolCallId,
	}, nil
}

func ParseInputRequiredDecision(content string) ([]*InputRequiredDecision, error) {
	out, err := provider.ExtractJson(content)
	if err != nil {
		return nil, err
	}

	var result InputRequiredResult
	if err = json.Unmarshal([]byte(out), &result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

type InputRequiredItem struct {
	ContextId  string
	TaskId     string
	ToolCallId string
	ToolName   string

	Message *provider.Message
}
