package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/hjhsamuel/agent/internal/service/ringbuffer"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/hjhsamuel/agent/internal/toolx"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session/compaction"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/toolconfirmation"
	"google.golang.org/genai"
)

func (s *Service) getAgentTree() (*AgentGroup, error) {
	agentObjs, err := s.store.AllAgent()
	if err != nil {
		return nil, err
	}

	var (
		mainAgent  = &AgentGroup{}
		groupAgent = make(map[bson.ObjectID]*AgentGroup)
	)
	for _, item := range agentObjs {
		switch item.Type {
		case schema.AgentTypeMain:
			mainAgent.ID = item.ID
			mainAgent.Name = item.Name
			mainAgent.Description = item.Description
			mainAgent.Prompt = item.Prompt
			mainAgent.Level = item.Level
		case schema.AgentTypeGroup:
			if v, ok := groupAgent[item.ID]; !ok {
				v = &AgentGroup{
					ID:          item.ID,
					Name:        item.Name,
					Description: item.Description,
					Prompt:      item.Prompt,
					Level:       item.Level,
					Agents:      make([]*AgentGroup, 0),
				}
				groupAgent[item.ID] = v
			} else {
				v.ID = item.ID
				v.Name = item.Name
				v.Description = item.Description
				v.Prompt = item.Prompt
				v.Level = item.Level
			}
		case schema.AgentTypeSub:
			if item.Parent != nil {
				info := &AgentGroup{
					ID:          item.ID,
					Name:        item.Name,
					Description: item.Description,
					Prompt:      item.Prompt,
					Level:       item.Level,
				}
				if v, ok := groupAgent[*item.Parent]; !ok {
					v = &AgentGroup{
						Agents: make([]*AgentGroup, 0),
					}
					v.Agents = append(v.Agents, info)
					groupAgent[*item.Parent] = v
				} else {
					v.Agents = append(v.Agents, info)
				}
			} else {
				mainAgent.Agents = append(mainAgent.Agents, &AgentGroup{
					ID:          item.ID,
					Name:        item.Name,
					Description: item.Description,
					Prompt:      item.Prompt,
					Level:       item.Level,
				})
			}
		default:
			continue
		}
	}

	if mainAgent.Name == "" {
		return nil, errors.New("has no main agent")
	}

	for _, item := range groupAgent {
		if item.Name == "" {
			continue
		}
		mainAgent.Agents = append(mainAgent.Agents, item)
	}

	return mainAgent, nil
}

func (s *Service) buildAgent(
	user string,
	cfg *AgentGroup,
	agentTool map[bson.ObjectID]map[schema.ToolType][]string,
	llmModel model.LLM,
) (agent.Agent, error) {
	// Stable ordering preserves the prompt prefix across runner rebuilds.
	sort.Slice(cfg.Agents, func(i, j int) bool { return cfg.Agents[i].Name < cfg.Agents[j].Name })
	subAgents := make([]agent.Agent, 0, len(cfg.Agents))
	for _, child := range cfg.Agents {
		agt, err := s.buildAgent(user, child, agentTool, llmModel)
		if err != nil {
			return nil, err
		}
		subAgents = append(subAgents, agt)
	}

	var (
		mcps  = make([]tool.Toolset, 0)
		a2as  = make([]agent.Agent, 0)
		skill tool.Toolset
	)
	if v, ok := agentTool[cfg.ID]; ok {
		for kind, paths := range v {
			switch kind {
			case schema.ToolMcp:
				mcps = toolx.GetMcp(user, paths...)
			case schema.ToolA2A:
				a2as = toolx.GetA2A(paths...)
			case schema.ToolSkill:
				skill = toolx.GetSkills(paths...)
			default:
				continue
			}
		}
	}
	if skill != nil {
		mcps = append(mcps, skill)
	}

	return llmagent.New(llmagent.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
		Instruction: cfg.Prompt,
		SubAgents:   append(subAgents, a2as...),
		Toolsets:    mcps,
		Model:       llmModel,
	})
}

func (s *Service) buildRunner(obj *schema.Session) (*Runner, error) {
	agt, err := s.getAgentTree()
	if err != nil {
		return nil, err
	}

	// Select once for the entire tree, including the compaction summarizer.
	m, err := s.selectModel(obj.LastModel, schema.ChatModel, reflectModelLevel(agt.Level)...)
	if err != nil {
		return nil, err
	}
	llmModel, err := s.newModel(m)
	if err != nil {
		return nil, err
	}

	tools, err := s.store.FindTools(bson.M{"state": schema.ToolStatusOnline})
	if err != nil {
		return nil, err
	}
	agentTool := make(map[bson.ObjectID]map[schema.ToolType][]string)
	for _, item := range tools {
		if item.Agent == nil {
			continue
		}
		if _, ok := agentTool[*item.Agent]; !ok {
			agentTool[*item.Agent] = make(map[schema.ToolType][]string)
		}
		if _, ok := agentTool[*item.Agent][item.Type]; !ok {
			agentTool[*item.Agent][item.Type] = make([]string, 0)
		}

		var path string
		switch item.Type {
		case schema.ToolMcp, schema.ToolA2A:
			path = item.Url
		case schema.ToolSkill:
			path = item.Name
		default:
			continue
		}
		agentTool[*item.Agent][item.Type] = append(agentTool[*item.Agent][item.Type], path)
	}

	mainAgt, err := s.buildAgent(obj.User, agt, agentTool, llmModel)
	if err != nil {
		return nil, err
	}
	inputState, err := inputStatePlugin(func() error {
		return s.store.UpdateSession(
			bson.M{"_id": obj.ID, "state": schema.SessionInputReady},
			bson.M{"$set": bson.M{"state": schema.SessionRunning, "updated_at": time.Now()}},
		)
	})
	if err != nil {
		return nil, err
	}
	r, err := runner.New(runner.Config{
		AppName:           schema.RunnerName,
		Agent:             mainAgt,
		SessionService:    s.adkSession,
		AutoCreateSession: true,
		PluginConfig:      runner.PluginConfig{Plugins: []*plugin.Plugin{inputState}},
		Compaction: &compaction.Config{
			TokenThreshold:     int(float64(m.Options.ContextLimit) * CompactRatio),
			EventRetentionSize: 3,
		},
	})
	if err != nil {
		return nil, err
	}

	selection := &schema.SessionModel{
		ModelID:  m.ID,
		APIKeyID: m.ApiKeys[0].ID,
		UsedAt:   time.Now(),
	}
	if err := s.store.UpdateSession(
		bson.M{"_id": obj.ID},
		bson.M{"$set": bson.M{"last_model": selection}},
	); err != nil {
		return nil, err
	}
	obj.LastModel = selection
	return &Runner{
		Runner: r,
		Name:   agt.Name,
	}, nil
}

func (s *Service) acquireProducer(id string) (*sessionLife, *ringbuffer.RingBufferProducer, error) {
	s.lifecycleMu.Lock()
	if s.stopping.Load() {
		s.lifecycleMu.Unlock()
		return nil, nil, errors.New("service is shutting down")
	}
	ctx, cancel := context.WithCancel(context.Background())
	life := &sessionLife{Context: ctx, cancel: cancel}
	if s.setLife(id, life) {
		s.lifecycleMu.Unlock()
		cancel()
		return nil, nil, errors.New("session already running")
	}
	// Register before Close can start waiting, including synchronous setup work.
	s.wg.Add(1)
	s.lifecycleMu.Unlock()

	producer, err := s.rbs.AcquireProducer(ctx, id)
	if err != nil {
		s.finishLife(id, life)
		return nil, nil, err
	}

	return life, producer, nil
}

func (s *Service) Run(user, id string, content string) error {
	sessionId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid session")
	}

	ctx, producer, err := s.acquireProducer(sessionId.Hex())
	if err != nil {
		return err
	}

	obj, err := s.GetSession(user, id)
	if err != nil {
		s.finishLife(sessionId.Hex(), ctx)
		return err
	}
	if obj == nil || (obj.State != schema.SessionTemp && obj.State != schema.SessionDone) {
		s.finishLife(sessionId.Hex(), ctx)
		return errors.New("session is not available for a new turn")
	}
	r, err := s.buildRunner(obj)
	if err != nil {
		s.finishLife(sessionId.Hex(), ctx)
		return err
	}

	// 多轮对话、首次对话
	message := &schema.Message{
		Session:   sessionId,
		Role:      schema.RoleUser,
		Content:   content,
		CreatedAt: time.Now(),
	}
	state, turnId, err := s.store.CreateNewTurn(message)
	if err != nil {
		s.finishLife(sessionId.Hex(), ctx)
		return err
	}

	msg := genai.NewContentFromText(content, genai.RoleUser)
	go s.newTurn(ctx, producer, user, sessionId, turnId, r, msg, state == schema.SessionTemp)

	return nil
}

func (s *Service) newTurn(
	ctx *sessionLife,
	producer *ringbuffer.RingBufferProducer,
	user string,
	sessionId bson.ObjectID,
	turnId int64,
	r *Runner,
	msg *genai.Content,
	isFirst bool,
) {
	defer s.finishLife(sessionId.Hex(), ctx)
	if ctx.Err() != nil {
		return
	}

	if isFirst {
		if msg != nil && len(msg.Parts) != 0 && msg.Parts[0].Text != "" {
			name, err := s.ExtractSessionTopic(ctx, msg.Parts[0].Text)
			if err != nil {
				name = "新对话"
			}
			if ctx.Err() != nil {
				return
			}
			_ = s.store.UpdateSession(
				bson.M{"_id": sessionId},
				bson.M{"$set": bson.M{"name": name}},
			)
		}
	}

	waitInputs, err := s.run(ctx, user, sessionId, turnId, r, producer, msg)
	// Shutdown is an interruption, not a completed/failed turn. Preserve the
	// persisted running state so Start can resume it, even if the runner ended
	// without returning a cancellation error.
	if s.stopping.Load() && ctx.Err() != nil {
		return
	}
	if err != nil {
		goto Error
	}
	goto Success

Error:
	_ = s.store.AddMessage(&schema.Message{
		Session:   sessionId,
		Turn:      turnId,
		Role:      schema.RoleError,
		Content:   err.Error(),
		CreatedAt: time.Now(),
	})
	_ = s.store.UpdateSession(
		bson.M{
			"_id":   sessionId,
			"state": bson.M{"$in": []schema.SessionStatus{schema.SessionRunning, schema.SessionInputReady}},
		},
		bson.M{"$set": bson.M{"state": schema.SessionDone, "wait": []*schema.WaitInput{}}},
	)

	s.releaseLife(sessionId.Hex(), ctx)
	_, _ = s.rbs.Push(producer, ringbuffer.SSEChunk{
		Type: ringbuffer.SSEChunkTypeError,
		Data: err.Error(),
	})
	_, _ = s.rbs.Push(producer, ringbuffer.SSEChunk{
		Type: ringbuffer.SSEChunkTypeEnd,
	})

	return

Success:
	if len(waitInputs) != 0 {
		err = s.store.UpdateSession(
			bson.M{"_id": sessionId, "state": schema.SessionRunning},
			bson.M{"$set": bson.M{
				"state":              schema.SessionWait,
				"wait":               waitInputs,
				"last_model.used_at": time.Now(),
			}},
		)
		if err != nil {
			goto Error
		}

		s.releaseLife(sessionId.Hex(), ctx)
		for _, item := range waitInputs {
			_, _ = s.rbs.Push(producer, ringbuffer.SSEChunk{
				Type: ringbuffer.SSEChunkTypeHumanInTheLoop,
				Data: ringbuffer.HITLEvent{ID: item.ID, Content: item.Hint}.ToData(),
			})
		}
	} else {
		err = s.store.UpdateSession(
			bson.M{"_id": sessionId},
			bson.M{"$set": bson.M{
				"state":              schema.SessionDone,
				"wait":               []*schema.WaitInput{},
				"last_model.used_at": time.Now(),
			}},
		)
		if err != nil {
			goto Error
		}
		s.releaseLife(sessionId.Hex(), ctx)
		_, _ = s.rbs.Push(producer, ringbuffer.SSEChunk{Type: ringbuffer.SSEChunkTypeEnd})
	}
	return
}

func (s *Service) run(
	ctx context.Context,
	user string,
	sessionId bson.ObjectID,
	turnId int64,
	r *Runner,
	producer *ringbuffer.RingBufferProducer,
	msg *genai.Content,
) ([]*schema.WaitInput, error) {
	inputRequiredReqs := make([]*schema.WaitInput, 0)

	events := r.Run(ctx, user, sessionId.Hex(), msg, agent.RunConfig{StreamingMode: agent.StreamingModeSSE})
	for event, err := range events {
		if err != nil {
			return nil, err
		}

		if event == nil {
			continue
		}

		// stream
		if event.Partial {
			if event.Content == nil {
				continue
			}
			for _, part := range event.Content.Parts {
				if part == nil || part.Text == "" {
					continue
				}

				chunk := ringbuffer.SSEChunk{Data: part.Text}
				if part.Thought {
					chunk.Type = ringbuffer.SSEChunkTypeReasoning
				} else {
					chunk.Type = ringbuffer.SSEChunkTypeData
				}
				_, _ = s.rbs.Push(producer, chunk)
			}
			continue
		}

		if event.Content == nil {
			continue
		}

		var content strings.Builder
		for _, part := range event.Content.Parts {
			if part == nil {
				continue
			}

			// 只将来自主 agent 的模型信息进行保存
			if !part.Thought && event.Content.Role == "model" && part.Text != "" && event.Author == r.Name {
				content.WriteString(part.Text)
			}

			// 处理 input_required
			if part.FunctionCall != nil && part.FunctionCall.Name == toolconfirmation.FunctionCallName {
				value, ok := part.FunctionCall.Args["toolConfirmation"]
				if !ok || value == nil {
					continue
				}
				data, err := json.Marshal(value)
				if err != nil {
					continue
				}
				var confirmation toolconfirmation.ToolConfirmation
				if err = json.Unmarshal(data, &confirmation); err != nil {
					continue
				}
				inputRequiredReqs = append(inputRequiredReqs, &schema.WaitInput{
					ID:        part.FunctionCall.ID,
					Hint:      confirmation.Hint,
					CreatedAt: time.Now(),
				})
			}
		}
		// 添加展示信息
		if content.Len() != 0 {
			err = s.store.AddMessage(&schema.Message{
				Session:   sessionId,
				Turn:      turnId,
				Role:      schema.RoleAssistant,
				Content:   content.String(),
				CreatedAt: time.Now(),
			})
			if err != nil {
				return nil, err
			}
		}
	}

	return inputRequiredReqs, nil
}

func (s *Service) SetInputRequiredInfo(user, id string, toolCallId string, content string) error {
	sessionId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	if strings.TrimSpace(content) == "" {
		return errors.New("input is empty")
	}
	obj, err := s.store.SubmitWaitInput(user, sessionId, toolCallId, content)
	if err != nil {
		return err
	}
	if obj.State != schema.SessionInputReady {
		return nil
	}

	ctx, producer, err := s.acquireProducer(sessionId.Hex())
	if err != nil {
		return err
	}

	r, err := s.buildRunner(obj)
	if err != nil {
		s.finishLife(sessionId.Hex(), ctx)
		return err
	}

	go s.newTurn(ctx, producer, user, sessionId, obj.TurnSeq-1, r, buildInputRequiredMessage(obj.Wait), false)

	return nil
}
