package local

import "google.golang.org/adk/v2/tool"

var Tools map[string]tool.Tool

func Register(t tool.Tool) {
	Tools[t.Name()] = t
}

func GetTools() []tool.Tool {
	tools := make([]tool.Tool, 0, len(Tools))
	for _, t := range Tools {
		tools = append(tools, t)
	}
	return tools
}
