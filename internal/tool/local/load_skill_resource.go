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
	Register(&LoadSkillResource{})
}

type LoadSkillResource struct{}

func (t *LoadSkillResource) Name() string {
	return "cagent_load_skill_resource"
}

func (t *LoadSkillResource) Description() string {
	return "Loads a resource file (e.g., from references/ or assets/) associated with the specified skill."
}

func (t *LoadSkillResource) Declaration() *genai.FunctionDeclaration {
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
				"resource": {
					Type:        genai.TypeString,
					Description: "The relative path to the resource (e.g., 'references/my_doc.md', 'assets/template.txt', or 'scripts/setup.sh').",
				},
			},
			Required: []string{"name", "resource"},
		},
	}
}

func (t *LoadSkillResource) IsLongRunning() bool {
	return false
}

func (t *LoadSkillResource) ProcessRequest(ctx agent.Context, req *model.LLMRequest) error {
	return PackTool(req, t)
}

func (t *LoadSkillResource) Run(ctx agent.Context, args any) (map[string]any, error) {
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

	resourceRaw, ok := m["resource"]
	if !ok {
		return nil, errors.New("missing required argument: resource")
	}
	resource, ok := resourceRaw.(string)
	if !ok {
		return nil, fmt.Errorf("resource must be a string, got: %T", resourceRaw)
	}

	content, err := skill.ReadResource(name, resource)
	if err != nil {
		return nil, fmt.Errorf("read resource '%q' from skill '%q': %w", resource, name, err)
	}

	return map[string]any{
		"content": string(content),
	}, nil
}
