package service

import (
	"context"
	"errors"
	"time"

	"github.com/hjhsamuel/agent/internal/toolx/local"

	"github.com/hjhsamuel/agent/internal/store/request"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/hjhsamuel/agent/pkg/skillx"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Service) AddTool(
	user string,
	name string,
	kind int,
	baseUrl string,
	agentId string,
	cred *schema.Credential,
) (string, error) {
	if schema.ToolType(kind) == schema.ToolLocal {
		if err := local.ValidateName(name); err != nil {
			return "", err
		}
	}
	if schema.ToolType(kind) == schema.ToolSkill {
		if err := skillx.ValidateName(name); err != nil {
			return "", err
		}
	}
	obj := &schema.Tool{
		Name:      name,
		Url:       baseUrl,
		CreatedAt: time.Now(),
		Creator:   user,
	}
	if agentId != "" {
		id, err := bson.ObjectIDFromHex(agentId)
		if err != nil {
			return "", err
		}
		obj.Agent = &id
		obj.State = schema.ToolStatusOnline
	} else {
		obj.State = schema.ToolStatusOffline
	}

	if cred != nil {
		switch cred.Type {
		case schema.CredentialHeaderApi:
			obj.Credential = &schema.Credential{
				Type:  schema.CredentialHeaderApi,
				Key:   cred.Key,
				Value: cred.Value,
			}
		case schema.CredentialBearer:
			obj.Credential = &schema.Credential{
				Type:  schema.CredentialBearer,
				Value: cred.Value,
			}
		default:
			return "", errors.New("invalid credential type")
		}
	}

	withAsync := false
	switch schema.ToolType(kind) {
	case schema.ToolA2A:
		obj.Type = schema.ToolA2A
	case schema.ToolMcp:
		obj.Type = schema.ToolMcp
	case schema.ToolLocal:
		obj.Type = schema.ToolLocal
		obj.State = schema.ToolStatusLoading
		withAsync = true
	case schema.ToolSkill:
		obj.Type = schema.ToolSkill
		obj.State = schema.ToolStatusLoading
		withAsync = true
	default:
		return "", errors.New("unknown tool type")
	}

	id, err := s.store.AddTool(obj)
	if err != nil {
		return "", err
	}

	if withAsync {
		obj.ID = id
		go s.asyncToolLoader(obj)
	}

	return id.Hex(), nil
}

func (s *Service) asyncToolLoader(obj *schema.Tool) {
	var state schema.ToolStatus
	switch obj.Type {
	case schema.ToolLocal, schema.ToolSkill:
		version, err := s.store.NextToolVersion(obj.Name)
		if err == nil {
			err = installToolVersion(obj.Type, obj.Name, obj.Url, version)
		}
		if err != nil {
			state = schema.ToolStatusOffline
		} else {
			_ = s.store.UpdateTool(
				bson.M{"_id": obj.ID, "state": schema.ToolStatusLoading},
				bson.M{"$set": bson.M{"state": schema.ToolStatusOnline, "version": version}},
			)
			return
		}

	default:
		return
	}

	_ = s.store.UpdateTool(
		bson.M{"_id": obj.ID, "state": schema.ToolStatusLoading},
		bson.M{"$set": bson.M{"state": state}},
	)
}

func installToolVersion(kind schema.ToolType, name, url string, version int64) error {
	var err error
	if kind == schema.ToolSkill {
		_, err = skillx.Install(context.Background(), name, url, version)
	} else {
		_, err = local.Install(context.Background(), name, url, version)
	}
	return err
}

func (s *Service) GetTools(req *request.GetToolsReq) ([]*schema.Tool, error) {
	filter := bson.M{}

	if req.Offset != "" {
		id, err := bson.ObjectIDFromHex(req.Offset)
		if err != nil {
			return nil, err
		}
		filter["_id"] = bson.M{"$gt": id}
	}

	if req.Agent != nil {
		if *req.Agent != "" {
			agentId, err := bson.ObjectIDFromHex(*req.Agent)
			if err != nil {
				return nil, err
			}
			filter["agent"] = agentId
		} else {
			filter["agent"] = bson.M{"$exists": false}
		}
	}
	if req.Type != nil {
		filter["type"] = schema.ToolType(*req.Type)
	}
	if req.State != nil {
		filter["state"] = schema.ToolStatus(*req.State)
	}

	return s.store.FindTools(
		filter,
		options.Find().SetSort(bson.M{"_id": 1}).SetLimit(req.Limit),
	)
}

func (s *Service) GetToolInfo(id string) (*schema.Tool, *schema.Agent, error) {
	toolId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, nil, err
	}

	objs, err := s.store.FindTools(bson.M{"_id": toolId}, options.Find().SetLimit(1))
	if err != nil {
		return nil, nil, err
	}
	if len(objs) == 0 {
		return nil, nil, mongo.ErrNoDocuments
	}

	toolObj := objs[0]
	if toolObj.Agent != nil {
		agentObjs, err := s.store.FindAgents(bson.M{"_id": *toolObj.Agent}, options.Find().SetLimit(1))
		if err != nil {
			return nil, nil, err
		}
		if len(agentObjs) != 0 {
			return toolObj, agentObjs[0], nil
		}
	}
	return toolObj, nil, nil
}

func (s *Service) UpdateToolState(id string, state int) error {
	toolId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	switch schema.ToolStatus(state) {
	case schema.ToolStatusOnline:
		objs, err := s.store.FindTools(
			bson.M{"_id": toolId, "state": schema.ToolStatusOffline},
			options.Find().SetLimit(1),
		)
		if err != nil {
			return err
		}
		if len(objs) == 0 {
			return mongo.ErrNoDocuments
		}

		switch objs[0].Type {
		case schema.ToolA2A, schema.ToolMcp:
			return s.store.UpdateTool(
				bson.M{"_id": toolId, "state": schema.ToolStatusOffline},
				bson.M{"$set": bson.M{"state": schema.ToolStatusOnline}},
			)
		case schema.ToolLocal, schema.ToolSkill:
			err = s.store.UpdateTool(
				bson.M{"_id": toolId, "state": schema.ToolStatusOffline},
				bson.M{"$set": bson.M{"state": schema.ToolStatusLoading}},
			)
			if err != nil {
				return err
			}

			obj := &schema.Tool{
				ID:    objs[0].ID,
				Name:  objs[0].Name,
				Url:   objs[0].Url,
				Type:  objs[0].Type,
				State: schema.ToolStatusLoading,
			}
			go s.asyncToolLoader(obj)
			return nil
		default:
			return errors.New("unknown tool type")
		}
	case schema.ToolStatusOffline:
		return s.store.UpdateTool(
			bson.M{"_id": toolId, "state": bson.M{"$ne": schema.ToolStatusLoading}},
			bson.M{"$set": bson.M{"state": state}},
		)
	default:
		return errors.New("invalid state")
	}
}

func (s *Service) UpdateToolInfo(id string, req *request.UpdateToolReq) error {
	toolId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	objs, err := s.store.FindTools(bson.M{"_id": toolId}, options.Find().SetLimit(1))
	if err != nil {
		return err
	}
	if len(objs) == 0 {
		return mongo.ErrNoDocuments
	}

	update := bson.M{}
	if req.Agent != nil {
		agentId, err := bson.ObjectIDFromHex(*req.Agent)
		if err != nil {
			return err
		}
		agents, err := s.store.FindAgents(bson.M{"_id": agentId}, options.Find().SetLimit(1))
		if err != nil {
			return err
		}
		if len(agents) == 0 {
			return mongo.ErrNoDocuments
		}
		update["agent"] = &agentId
	}

	toolObj := objs[0]
	switch toolObj.Type {
	case schema.ToolA2A, schema.ToolMcp:
		if req.Credential != nil {
			cred := *req.Credential
			switch cred.Type {
			case schema.CredentialHeaderApi:
				if cred.Key == "" {
					return errors.New("credential key is required")
				}
			case schema.CredentialBearer:
				cred.Key = ""
			default:
				return errors.New("invalid credential type")
			}
			update["credential"] = &cred
		}
		if req.Name != nil {
			update["name"] = *req.Name
		}
		if req.Url != nil {
			update["url"] = *req.Url
		}
		return s.store.UpdateTool(bson.M{"_id": toolId}, bson.M{"$set": update})
	case schema.ToolLocal, schema.ToolSkill:
		if req.Name != nil && *req.Name != toolObj.Name {
			return errors.New("tool name is immutable")
		}
		if req.Url != nil {
			version, err := s.store.NextToolVersion(toolObj.Name)
			if err == nil {
				err = installToolVersion(toolObj.Type, toolObj.Name, *req.Url, version)
			}
			if err != nil {
				return err
			}
			update["url"] = *req.Url
			update["version"] = version
		}
		filter := bson.M{"_id": toolId, "state": bson.M{"$ne": schema.ToolStatusLoading}}
		if version, ok := update["version"]; ok {
			// A slower download must not roll back a newer published version.
			filter["$or"] = bson.A{bson.M{"version": bson.M{"$lt": version}}, bson.M{"version": bson.M{"$exists": false}}}
		}
		return s.store.UpdateTool(filter, bson.M{"$set": update})

	default:
		return errors.New("unknown tool type")
	}
}

func (s *Service) DeleteTool(id string) error {
	toolId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	return s.store.DeleteTool(bson.M{"_id": toolId})
}
