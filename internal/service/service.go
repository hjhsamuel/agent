package service

import (
	"github.com/hjhsamuel/agent/internal/db"
	"github.com/hjhsamuel/agent/internal/llm"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/shard"
	"github.com/hjhsamuel/agent/pkg/skill"
	"github.com/hjhsamuel/agent/pkg/tool"
)

type Service struct {
	tools     *tool.Manager
	providers *llm.Manager
	notify    *notify.Manager
	store     *db.Dao

	skills []*skill.Skill

	agents *shard.Manager // 会话agent
}
