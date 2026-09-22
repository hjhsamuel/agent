package resolver

import (
	"fmt"
	"strings"

	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/tool"
)

func BuildMessages(messages []*provider.Message) string {
	var summary string
	parts := make([]string, 0)
	for _, message := range messages {
		if message == nil {
			continue
		}
		switch message.Role {
		case provider.RoleSystem:
			summary = message.Content
		case provider.RoleUser:
			parts = append(parts, "[User]: "+message.Content)
		case provider.RoleAssistant:
			if message.Content != "" {
				parts = append(parts, "[Assistant]: "+message.Content)
			}
			if len(message.ToolCalls) != 0 {
				toolCalls := make([]string, 0)
				for _, toolCall := range message.ToolCalls {
					if toolCall == nil {
						continue
					}
					toolCalls = append(toolCalls,
						fmt.Sprintf("(tool_call_id: %s) %s(%s)", toolCall.ID, toolCall.Name, toolCall.Arguments),
					)
				}
				parts = append(parts, "[Assistant tool calls]: "+strings.Join(toolCalls, " ;"))
			}
		case provider.RoleTool:
			result := tool.Truncate(message.Content)
			parts = append(parts, fmt.Sprintf("[Tool result (tool_call_id: %s)]: %s", message.ToolCallId,
				result.Content))
		}
	}

	var content string
	if summary != "" {
		content = "<previous-summary>\n" + summary + "\n</previous-summary>\n\n>"
	}
	content += "<conversation>\n" + strings.Join(parts, "\n\n") + "\n</conversation>\n\n"

	return content
}
