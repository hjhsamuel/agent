package service

import (
	"github.com/hjhsamuel/agent/internal/service/adapter"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/runner"
)

func (s *Service) getAgent(cfg *AgentConfig) (agent.Agent, error) {
	llmModel, err := s.randomModel(schema.ChatModel)
	if err != nil {
		return nil, err
	}

	return llmagent.New(llmagent.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
		Instruction: cfg.Prompt,
		SubAgents:   cfg.SubAgents,
		Tools:       cfg.Tools,
		Toolsets:    cfg.Toolsets,
		Model:       llmModel,
	})
}

func (s *Service) getRunner(agt agent.Agent) (*runner.Runner, error) {
	return runner.New(runner.Config{
		AppName:           "root",
		Agent:             agt,
		SessionService:    adapter.NewSessionService(s.store),
		AutoCreateSession: true,
	})
}
