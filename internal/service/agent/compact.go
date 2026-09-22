package agent

import (
	"errors"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/service/agent/compact"
	"github.com/hjhsamuel/agent/internal/service/agent/prompts"
	"github.com/hjhsamuel/agent/pkg/provider"
)

func (a *Agent) compact() error {
	if len(a.runtime.OldMessages) == 0 {
		return nil
	}

	var preSummary string
	if a.runtime.OldMessages[0].Role == provider.RoleSystem {
		preSummary = a.runtime.OldMessages[0].Content
	}
	messages := compact.ConvertMessages(preSummary, a.runtime.OldMessages)
	response, err := a.base.Compact.Chat(
		a.ctx,
		prompts.CompactSystemPrompt,
		messages,
		&provider.ChatConfig{
			Temperature: 0.1,
		},
	)
	if err != nil {
		return err
	}
	if response.Content == "" {
		return errors.New("model returned an empty conversation summary")
	}
	err = a.base.Store.AddConversationCompaction(&schema.Compaction{
		Conversation: a.id,
		Message:      a.runtime.oldId,
		Content:      response.Content,
	})
	if err != nil {
		return err
	}

	a.runtime.OldMessages = []*provider.Message{{Role: provider.RoleSystem, Content: response.Content}}
	return nil
}
