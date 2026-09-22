package local

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
				Name:        string(tool.LocalTool + "_load_skill"),
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
		return &tool.ToolResult{
			Status:  tool.TaskFailed,
			Content: fmt.Sprintf("invalid arguments: %v", err),
		}, nil
	}

	if params["name"] == "" {
		return &tool.ToolResult{
			Status:  tool.TaskFailed,
			Content: "invalid arguments: name is required",
		}, nil
	}

	var path string
	if params["path"] == "" {
		// load SKILL.md
		path = filepath.Join(params["name"], l.skillFile)
	} else {
		// load reference
		path = filepath.Join(params["name"], params["path"])
	}

	if !filepath.IsLocal(path) {
		return nil, fmt.Errorf("skill path escapes the skill directory")
	}
	root, err := os.OpenRoot(l.root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 128*1024+1))
	if err != nil {
		return &tool.ToolResult{
			Status:  tool.TaskFailed,
			Content: fmt.Sprintf("loading skill error: %v", err),
		}, nil
	}
	if len(body) > 128*1024 {
		return nil, fmt.Errorf("skill resource exceeds 128 KiB")
	}
	return &tool.ToolResult{
		Status:  tool.TaskCompleted,
		Content: string(body),
	}, nil
}

func (l *LoadSkill) Resume(ctx context.Context, token, contextId, taskId string, content string) (*tool.ToolResult, error) {
	return &tool.ToolResult{
		Status:  tool.TaskFailed,
		Content: "local tool not support resume",
	}, nil
}

func (l *LoadSkill) Check(ctx context.Context, token, contextId, taskId string) (*tool.ToolResult, error) {
	return &tool.ToolResult{
		Status:  tool.TaskFailed,
		Content: "tool not has no task state",
	}, nil
}

func NewLoadSkill() tool.Tool {
	return &LoadSkill{
		root:      "skills",
		skillFile: "SKILL.md",
	}
}
