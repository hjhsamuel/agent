package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hjhsamuel/agent/internal/skills"
)

type LoadSkillReference struct{}

func (l *LoadSkillReference) Basic() *ToolDefine {
	return &ToolDefine{
		Name:        "load_skill_reference",
		Description: "Load one text resource referenced by skill.",
		Parameter: schema(map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Skill name",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Resource path returned by **load_skill**",
			},
		}, "name", "path"),
	}
}

func (l *LoadSkillReference) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var req *LoadSkillReferenceReq
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("invalid arguments: %v", err)
	}
	content, err := skills.LoadReference(req.Name, req.Path)
	if err != nil {
		return nil, fmt.Errorf("load reference failed: %v", err)
	}
	return &ToolResult{
		Content: string(content),
		Details: map[string]any{
			"skill": req.Name,
			"path":  req.Path,
		},
	}, nil
}

func (l *LoadSkillReference) RequireApproval() bool {
	return false
}

func NewLoadSkillReferenceTool() Tool {
	return &LoadSkillReference{}
}

type LoadSkillReferenceReq struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
