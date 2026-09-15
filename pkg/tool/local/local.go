package local

import (
	"github.com/hjhsamuel/agent/pkg/tool"
)

var localTools = make(map[string]tool.Tool)

func Register(t tool.Tool) {
	name := t.Define().OfFunction.Function.Name
	// 覆盖
	localTools[name] = t
}

func GetLocalTools() []tool.Tool {
	out := make([]tool.Tool, 0, len(localTools))
	for _, t := range localTools {
		out = append(out, t)
	}
	return out
}
