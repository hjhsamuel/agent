package service

import (
	"errors"
	"time"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) AddAgent(obj *schema.Agent) (string, error) {
	var (
		id  bson.ObjectID
		err error
	)
	switch obj.Type {
	case schema.AgentTypeMain:
		id, err = s.addMainAgent(obj)
	case schema.AgentTypeGroup:
		id, err = s.addAgentGroup(obj)
	case schema.AgentTypeRunner:
		id, err = s.addSubAgent(obj)
	default:
		return "", errors.New("unknown agent type")
	}
	if err != nil {
		return "", err
	}
	return id.Hex(), nil
}

func (s *Service) addMainAgent(obj *schema.Agent) (bson.ObjectID, error) {
	info := &schema.Agent{
		Name:        obj.Name,
		Description: obj.Description,
		Prompt:      obj.Prompt,
		Type:        schema.AgentTypeMain,
		Level:       schema.AgentLevelMedium,
		CreatedAt:   time.Now(),
	}

	if info.Name == "" {
		info.Name = schema.DefaultMainAgentName
	}
	if info.Description == "" {
		info.Description = schema.DefaultMainAgentDescription
	}
	if info.Prompt == "" {
		info.Prompt = schema.DefaultMainAgentPrompt
	}

	return s.store.AddAgent(info)
}

func (s *Service) addAgentGroup(obj *schema.Agent) (bson.ObjectID, error) {
	if obj.Name == "" || obj.Description == "" || obj.Prompt == "" {
		return bson.ObjectID{}, errors.New("name, description, prompt are required")
	}

	agents, err := s.store.GetAgents(schema.AgentTypeMain)
	if err != nil {
		return bson.ObjectID{}, err
	}
	if len(agents) == 0 {
		return bson.ObjectID{}, errors.New("no main agent")
	}

	info := &schema.Agent{
		Name:        obj.Name,
		Description: obj.Description,
		Prompt:      obj.Prompt,
		Type:        schema.AgentTypeGroup,
		Parent:      &(agents[0].ID),
		Level:       obj.Level,
		CreatedAt:   time.Now(),
	}

	return s.store.AddAgent(info)
}

func (s *Service) addSubAgent(obj *schema.Agent) (bson.ObjectID, error) {
	if obj.Name == "" || obj.Description == "" || obj.Prompt == "" {
		return bson.ObjectID{}, errors.New("name, description, prompt are required")
	}
	if obj.Parent == nil {
		return bson.ObjectID{}, errors.New("parent is required")
	}

	agents, err := s.store.FindAgents(bson.M{"_id": *obj.Parent})
	if err != nil {
		return bson.ObjectID{}, err
	}
	if len(agents) == 0 {
		return bson.ObjectID{}, errors.New("parent agent not exists")
	}
	if agents[0].Type == schema.AgentTypeRunner {
		// TODO
		// 暂时仅支持 group 形式，后续支持多层级 subagent
		return bson.ObjectID{}, errors.New("can only add to main agent or agent group")
	}

	info := &schema.Agent{
		Name:        obj.Name,
		Description: obj.Description,
		Prompt:      obj.Prompt,
		Type:        schema.AgentTypeRunner,
		Parent:      &(agents[0].ID),
		Level:       obj.Level,
		CreatedAt:   time.Now(),
	}

	return s.store.AddAgent(info)
}
