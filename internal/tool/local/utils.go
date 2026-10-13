package local

import (
	"fmt"
	"strings"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

type Tool interface {
	Name() string
	Declaration() *genai.FunctionDeclaration
}

func PackTool(req *model.LLMRequest, t Tool) error {
	if req.Tools == nil {
		req.Tools = make(map[string]any)
	}

	name := t.Name()

	if _, ok := req.Tools[name]; ok {
		return fmt.Errorf("duplicate tool: %q", name)
	}
	req.Tools[name] = t

	if req.Config == nil {
		req.Config = &genai.GenerateContentConfig{}
	}
	if decl := t.Declaration(); decl == nil {
		return nil
	}
	// Find an existing genai.Tool with FunctionDeclarations
	var funcTool *genai.Tool
	for _, tool := range req.Config.Tools {
		if tool != nil && tool.FunctionDeclarations != nil {
			funcTool = tool
			break
		}
	}
	if funcTool == nil {
		req.Config.Tools = append(req.Config.Tools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{t.Declaration()},
		})
	} else {
		funcTool.FunctionDeclarations = append(funcTool.FunctionDeclarations, t.Declaration())
	}
	return nil
}

func AppendInstructions(r *model.LLMRequest, instructions ...string) {
	if len(instructions) == 0 {
		return
	}

	inst := strings.Join(instructions, "\n\n")

	if r.Config == nil {
		r.Config = &genai.GenerateContentConfig{}
	}

	if r.Config.SystemInstruction == nil {
		r.Config.SystemInstruction = genai.NewContentFromText(inst, genai.RoleUser)
		return
	}
	if len(r.Config.SystemInstruction.Parts) > 0 && r.Config.SystemInstruction.Parts[len(r.Config.SystemInstruction.Parts)-1].Text != "" {
		r.Config.SystemInstruction.Parts[len(r.Config.SystemInstruction.Parts)-1].Text += "\n\n" + inst
		return
	}
	r.Config.SystemInstruction.Parts = append(r.Config.SystemInstruction.Parts, genai.NewPartFromText(inst))
}
