package resolver

import (
	"strings"
	"testing"

	"github.com/hjhsamuel/agent/pkg/provider"
)

func TestBuildMessagesPreservesCallIDsWithOutOfOrderResults(t *testing.T) {
	content := BuildMessages([]*provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []*provider.ToolCall{
			{ID: "call-a", Name: "lookup", Arguments: `{"order":42}`},
			{ID: "call-b", Name: "lookup", Arguments: `{"order":43}`},
		}},
		{Role: provider.RoleTool, ToolCallId: "call-b", Content: "pending"},
		{Role: provider.RoleTool, ToolCallId: "call-a", Content: "approved"},
	})
	for _, want := range []string{
		`(tool_call_id: call-a) lookup({"order":42})`,
		`(tool_call_id: call-b) lookup({"order":43})`,
		`[Tool result (tool_call_id: call-b)]: pending`,
		`[Tool result (tool_call_id: call-a)]: approved`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("missing tool association %q in %s", want, content)
		}
	}
}
