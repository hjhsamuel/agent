package local

import (
	"errors"
	"fmt"

	"github.com/hjhsamuel/agent/pkg/skill"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

func init() {
	Register(&LoadSkill{})
}

type LoadSkill struct{}

func (t *LoadSkill) Name() string {
	return "cagent_load_skill"
}

func (t *LoadSkill) Description() string {
	return "Loads the SKILL.md instructions for a given skill"
}

func (t *LoadSkill) Declaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"name": {
					Type:        genai.TypeString,
					Description: "The name of the skill",
				},
			},
			Required: []string{"name"},
		},
	}
}

func (t *LoadSkill) IsLongRunning() bool {
	return false
}

func (t *LoadSkill) ProcessRequest(ctx agent.Context, req *model.LLMRequest) error {
	return PackTool(req, t)
}

func (t *LoadSkill) Run(ctx agent.Context, args any) (map[string]any, error) {
	m, ok := args.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected args type: %T", args)
	}

	nameRaw, ok := m["name"]
	if !ok {
		return nil, errors.New("missing required argument: name")
	}
	name, ok := nameRaw.(string)
	if !ok {
		return nil, fmt.Errorf("name must be a string, got: %T", nameRaw)
	}

	_, content, err := skill.ReadSkill(name, true)
	if err != nil {
		return nil, fmt.Errorf("load instructions for skill %q: %w", name, err)
	}

	return map[string]any{
		"content": content,
	}, nil
}
