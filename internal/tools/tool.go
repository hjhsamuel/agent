package tools

import (
	"context"
	"encoding/json"
)

type Tool interface {
	Basic() *ToolDefine
	Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error)
	RequireApproval() bool
}

type ToolDefine struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameter   map[string]any `json:"parameter"`
}

type ToolResult struct {
	Content string         `json:"content"`
	Details map[string]any `json:"details,omitempty"`
}

func schema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}
