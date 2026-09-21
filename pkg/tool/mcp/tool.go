package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hjhsamuel/agent/pkg/tool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

// ErrNotSupported indicates that this synchronous adapter cannot resume or poll tasks.
var ErrNotSupported = errors.New("MCP tool does not support task resume or status checks")

type MCP struct {
	id          string
	name        string
	description string
	addr        string
	parameters  shared.FunctionParameters

	client *mcp.Client
}

var _ tool.Tool = (*MCP)(nil)

// NewMCP wraps a remote tool definition while preserving its input schema.
func NewMCP(client *mcp.Client, id, addr string, definition *mcp.Tool) (tool.Tool, error) {
	if definition == nil || definition.Name == "" {
		return nil, errors.New("MCP tool definition requires a name")
	}

	data, err := json.Marshal(definition.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("encode MCP tool %q schema: %w", definition.Name, err)
	}

	var parameters shared.FunctionParameters
	if err = json.Unmarshal(data, &parameters); err != nil {
		return nil, fmt.Errorf("decode MCP tool %q schema: %w", definition.Name, err)
	}
	if parameters == nil || parameters["type"] != "object" {
		return nil, fmt.Errorf("MCP tool %q input schema must describe an object", definition.Name)
	}
	return &MCP{
		id:          id,
		name:        definition.Name,
		description: definition.Description,
		addr:        addr,
		parameters:  parameters,
		client:      client,
	}, nil
}

func (m *MCP) Type() tool.ToolType { return tool.McpTool }

func (m *MCP) Define() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Type: "function",
			Function: shared.FunctionDefinitionParam{
				Name:        m.id,
				Description: openai.String(m.description),
				Parameters:  m.parameters,
			},
		},
	}
}

// Execute accepts the JSON object arguments described by Define.
// Each call owns and closes its session, isolating concurrent callers' credentials.
func (m *MCP) Execute(ctx context.Context, token, content string) (*tool.ToolResult, error) {
	if strings.TrimSpace(content) == "" {
		content = "{}"
	}
	var arguments map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &arguments); err != nil {
		return nil, fmt.Errorf("invalid MCP tool arguments: %w", err)
	}
	if arguments == nil {
		return nil, errors.New("MCP tool arguments must be a JSON object")
	}

	session, err := connect(ctx, m.client, m.addr, token)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	result, err := session.CallTool(
		ctx,
		&mcp.CallToolParams{
			Name:      m.name,
			Arguments: arguments,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("call MCP tool %q: %w", m.name, err)
	}
	if result.NeedsInput() {
		return nil, fmt.Errorf("MCP tool %q requires an unsupported input round trip: %w", m.name, ErrNotSupported)
	}
	// Preserve all content types, structured output, and tool error details.
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode MCP tool %q result: %w", m.name, err)
	}
	status := tool.TaskCompleted
	if result.IsError {
		status = tool.TaskFailed
	}
	return &tool.ToolResult{
		Status:  status,
		Content: string(data),
	}, nil
}

func (m *MCP) Resume(ctx context.Context, token, contextId, taskId, content string) (*tool.ToolResult, error) {
	return nil, ErrNotSupported
}

func (m *MCP) Check(ctx context.Context, token, contextId, taskId string) (*tool.ToolResult, error) {
	return nil, ErrNotSupported
}
