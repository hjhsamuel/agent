package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hjhsamuel/agent/pkg/tool"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

func init() {
	Register(NewLoadSkill())
}

type LoadSkill struct {
	root      string
	skillFile string
}

func (l *LoadSkill) Type() tool.ToolType {
	return tool.LocalTool
}

func (l *LoadSkill) Define() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Type: "function",
			Function: shared.FunctionDefinitionParam{
				Name:        string(tool.LocalTool + ":load_skill"),
				Description: openai.String("Load the complete instructions or one text resource reference for one available skill. Call this before using a matching skill."),
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "Skill name",
						},
						"path": map[string]any{
							"type":        "string",
							"description": "Resource path. Only used in loading text resource reference.",
						},
					},
					"required":             []string{"name"},
					"additionalProperties": false,
				},
			},
		},
	}
}

func (l *LoadSkill) Execute(ctx context.Context, token, content string) (*tool.ToolResult, error) {
	var params map[string]string
	if err := json.Unmarshal([]byte(content), &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %v", err)
	}

	if params["name"] == "" {
		return nil, errors.New("invalid arguments: name is required")
	}

	var path string
	if params["path"] == "" {
		// load SKILL.md
		path = filepath.Join(l.root, params["name"], l.skillFile)
	} else {
		// load reference
		path = filepath.Join(l.root, params["name"], params["path"])
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading skill error: %v", err)
	}
	return &tool.ToolResult{Content: string(body)}, nil
}

func (l *LoadSkill) Resume(ctx context.Context, token, contextId, taskId string, content string) (*tool.ToolResult, error) {
	return nil, errors.New("not supported")
}

func (l *LoadSkill) Check(ctx context.Context, token, contextId, taskId string) (*tool.ToolResult, error) {
	return nil, errors.New("not supported")
}

func NewLoadSkill() tool.Tool {
	return &LoadSkill{
		root:      "skills",
		skillFile: "SKILL.md",
	}
}
