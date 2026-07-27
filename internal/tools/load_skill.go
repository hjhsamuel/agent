package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hjhsamuel/agent/internal/skills"
)

type LoadSkill struct{}

func (l *LoadSkill) Basic() *ToolDefine {
	return &ToolDefine{
		Name:        "load_skill",
		Description: "Load the complete instructions for one available skill. Call this before using a matching skill.",
		Parameter: schema(map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Skill name from the system prompt",
			},
		}, "name"),
	}
}

func (l *LoadSkill) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var req *LoadSkillReq
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("invalid arguments: %v", err)
	}
	skill, err := skills.LoadSkill(req.Name)
	if err != nil {
		return nil, fmt.Errorf("load skill failed: %v", err)
	}
	content := "# Skill: " + skill.Name + "\n\n" + skill.Body
	if len(skill.Reference) != 0 {
		content += "\n\nAvailable references (load only when needed with **load_skill_reference**):\n- " +
			strings.Join(skill.Reference, "\n- ")
	}
	return &ToolResult{
		Content: content,
		Details: map[string]any{
			"skill":      skill.Name,
			"references": len(skill.Reference),
		},
	}, nil
}

func (l *LoadSkill) RequireApproval() bool {
	return false
}

func NewLoadSkill() Tool {
	return &LoadSkill{}
}

type LoadSkillReq struct {
	Name string `json:"name"`
}
