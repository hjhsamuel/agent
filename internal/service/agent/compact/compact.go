package compact

import (
	"fmt"
	"strings"

	"github.com/hjhsamuel/agent/internal/service/agent/prompts"
	"github.com/hjhsamuel/agent/pkg/provider"
)

// ConvertMessages
//
// 由于 tool_call_id 未被填充，因此需要保证参与压缩的消息中，不存在未完成的 tool call
func ConvertMessages(preSummary string, messages []*provider.Message) []*provider.Message {
	userMessage := convertMessages(preSummary, messages)
	return []*provider.Message{
		{Role: provider.RoleSystem, Content: prompts.CompactSystemPrompt},
		{Role: provider.RoleUser, Content: userMessage},
	}
}

func convertMessages(preSummary string, messages []*provider.Message) string {
	parts := make([]string, 0)
	for _, message := range messages {
		switch message.Role {
		case provider.RoleUser:
			parts = append(parts, "[User]: "+message.Content)
		case provider.RoleAssistant:
			if message.Content != "" {
				parts = append(parts, "[Assistant]: "+message.Content)
			}
			if len(message.ToolCalls) != 0 {
				toolCalls := make([]string, 0)
				for _, toolCall := range message.ToolCalls {
					toolCalls = append(toolCalls,
						fmt.Sprintf("%s(%s)", toolCall.Name, toolCall.Arguments),
					)
				}
				parts = append(parts, "[Assistant tool calls]: "+strings.Join(toolCalls, " ;"))
			}
		case provider.RoleTool:
			result := Truncate(message.Content)
			parts = append(parts, "[Tool result]: "+result.Content)
		}
	}

	content := "<conversation>\n" + strings.Join(parts, "\n\n") + "\n</conversation>\n\n"
	if preSummary != "" {
		content += "<previous-summary>\n" + preSummary + "\n</previous-summary>\n\n>" + prompts.CompactUpdatePrompt
	} else {
		content += prompts.CompactSummaryPrompt
	}

	return content
}
