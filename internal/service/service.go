package service

import (
	"github.com/hjhsamuel/agent/internal/db"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/provider"
	"github.com/hjhsamuel/agent/pkg/skill"
	"github.com/hjhsamuel/agent/pkg/tool"
)

type Service struct {
	tools     *tool.Manager
	providers *provider.Manager
	notify    *notify.Manager
	store     *db.Dao

	skills []*skill.Skill
}
