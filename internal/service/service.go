package service

import (
	"sync"

	"github.com/hjhsamuel/agent/internal/db"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/provider"
	"github.com/hjhsamuel/agent/internal/service/agent"
	"github.com/hjhsamuel/agent/pkg/skill"
	"github.com/hjhsamuel/agent/pkg/tool"
)

type Service struct {
	tools     *tool.Manager
	providers *provider.Manager
	notify    *notify.Manager
	store     *db.Dao

	skills []*skill.Skill

	lock     sync.RWMutex
	agentMap map[string]agent.Agent

	event chan *notify.ChannelEvent
}
