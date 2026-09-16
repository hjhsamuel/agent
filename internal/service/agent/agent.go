package agent

import (
	"context"

	"github.com/hjhsamuel/agent/internal/db"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/provider"
	"github.com/hjhsamuel/agent/pkg/skill"
	"github.com/hjhsamuel/agent/pkg/tool"
)

type Agent struct {
	ctx context.Context

	prompt string
	skills []*skill.Skill

	tools    map[string]tool.Tool
	provider *provider.LLM
	store    *db.Dao
	notify   chan<- *notify.ChannelEvent
}
