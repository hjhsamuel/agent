package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hjhsamuel/agent/internal/store/request"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/hjhsamuel/agent/pkg/filex"
	"github.com/hjhsamuel/agent/pkg/skillx"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Service) AddTool(user string, name string, kind int, baseUrl string, agentId string) (string, error) {
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
	case schema.ToolSkill:
		err := s.downloadAndExtractTool(obj.Type, obj.Url, obj.Name)
		if err != nil {
			state = schema.ToolStatusOffline
		} else {
			state = schema.ToolStatusOnline
		}
	case schema.ToolLocal:
		// TODO
		// 暂不支持本地工具
		state = schema.ToolStatusOffline
	default:
		return
	}

	_ = s.store.UpdateTool(
		bson.M{"_id": obj.ID},
		bson.M{"$set": bson.M{"state": state}},
	)
}

func (s *Service) downloadAndExtractTool(kind schema.ToolType, baseUrl string, toolName string) error {
	if kind != schema.ToolSkill {
		return errors.New("unknown tool kind")
	}

	if err := skillx.ValidateName(toolName); err != nil {
		return err
	}

	destination := filepath.Join("skills", toolName)
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("skill %q already exists", toolName)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check skill destination: %w", err)
	}

	// Keep staging on the same filesystem so installation can use rename.
	tempDir, err := os.MkdirTemp("skills", ".tool-*")
	if err != nil {
		return fmt.Errorf("create temporary skill directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempDownloadDir := filepath.Join(tempDir, "downloads")
	tempExtractDir := filepath.Join(tempDir, "extracts")
	dp, err := filex.Download(context.Background(), baseUrl, tempDownloadDir)
	if err != nil {
		return fmt.Errorf("download skill: %w", err)
	}
	if err := filex.Extract(context.Background(), dp, tempExtractDir); err != nil {
		return fmt.Errorf("extract skill: %w", err)
	}

	entries, err := os.ReadDir(tempExtractDir)
	if err != nil {
		return fmt.Errorf("read extracted skill directory: %w", err)
	}
	// Accept both a flat archive and an archive with one enclosing directory.
	skillDir := tempExtractDir
	if len(entries) == 1 && entries[0].IsDir() {
		skillDir = filepath.Join(tempExtractDir, entries[0].Name())
	}
	if err := skillx.ValidateSkill(skillDir, toolName); err != nil {
		return fmt.Errorf("validate skill: %w", err)
	}

	if err := os.Rename(skillDir, destination); err != nil {
		return fmt.Errorf("install skill %q: %w", toolName, err)
	}
	return nil
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
		case schema.ToolLocal:
			return errors.New("not supported")
		case schema.ToolSkill:
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
			bson.M{"_id": toolId},
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
		if req.Name != nil {
			update["name"] = *req.Name
		}
		if req.Url != nil {
			update["url"] = *req.Url
		}
		return s.store.UpdateTool(bson.M{"_id": toolId}, bson.M{"$set": update})
	case schema.ToolSkill:
		if req.Url != nil {
			// 更新地址，表示需要重新拉取
			// TODO
			// 暂不支持
			return errors.New("not supported")
		} else {
			// 更新基础信息，或修改关联
			if req.Name != nil {
				update["name"] = *req.Name
			}
			return s.store.UpdateTool(bson.M{"_id": toolId}, bson.M{"$set": update})
		}
	case schema.ToolLocal:
		// TODO
		// 暂不支持
		return errors.New("not supported")
	default:
		return errors.New("unknown tool type")
	}
}
