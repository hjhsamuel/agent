package service

import (
	"errors"
	"time"

	"github.com/hjhsamuel/agent/internal/store/request"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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
	case schema.AgentTypeSub:
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

	// TODO
	// 暂不支持 group 嵌套
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
	if agents[0].Type == schema.AgentTypeSub {
		// TODO
		// 暂时仅支持 group 形式，后续支持多层级 subagent
		return bson.ObjectID{}, errors.New("can only add to main agent or agent group")
	}

	info := &schema.Agent{
		Name:        obj.Name,
		Description: obj.Description,
		Prompt:      obj.Prompt,
		Type:        schema.AgentTypeSub,
		Parent:      &(agents[0].ID),
		Level:       obj.Level,
		CreatedAt:   time.Now(),
	}

	return s.store.AddAgent(info)
}

func (s *Service) UpdateAgent(id string, req *request.UpdateAgentReq) error {
	agentId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	filter := bson.M{"_id": agentId}

	update := bson.M{}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if req.Prompt != nil {
		update["prompt"] = *req.Prompt
	}
	if req.Level != nil {
		level := schema.AgentLevel(*req.Level)
		if level < schema.AgentLevelLow || level > schema.AgentLevelHigh {
			return errors.New("invalid agent level")
		}
		update["level"] = level
	}

	return s.store.UpdateAgent(filter, bson.M{"$set": update})
}

func (s *Service) DeleteAgent(id string) error {
	agentId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	return s.store.DeleteAgent(agentId)
}

func (s *Service) GetAgents(req *request.GetAgentsReq) ([]*schema.Agent, error) {
	filter := bson.M{}
	if req.Offset != "" {
		id, err := bson.ObjectIDFromHex(req.Offset)
		if err != nil {
			return nil, err
		}
		filter["_id"] = bson.M{"$gt": id}
	}

	switch schema.AgentType(req.Type) {
	case schema.AgentTypeMain:
		return s.store.GetAgents(schema.AgentTypeMain)
	case schema.AgentTypeGroup:
		filter["type"] = schema.AgentTypeGroup
		return s.store.FindAgents(
			filter,
			options.Find().SetSort(bson.M{"_id": 1}).SetLimit(req.Limit),
		)
	case schema.AgentTypeSub:
		filter["type"] = schema.AgentTypeSub
		return s.store.FindAgents(
			filter,
			options.Find().SetSort(bson.M{"_id": 1}).SetLimit(req.Limit),
		)
	default:
		return nil, errors.New("invalid agent type")
	}
}
