package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/hjhsamuel/agent/pkg/filex"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) AddTool(user string, name string, kind int, baseUrl string) (string, error) {
	obj := &schema.Tool{
		Name:      name,
		Url:       baseUrl,
		State:     schema.ToolStatusOnline,
		CreatedAt: time.Now(),
		Creator:   user,
	}
	withAsync := false
	switch schema.ToolType(kind) {
	case schema.ToolA2A:
		if err := s.tool.AddA2A(baseUrl); err != nil {
			return "", err
		}
		obj.Type = schema.ToolA2A
	case schema.ToolMcp:
		if err := s.tool.AddMcp(baseUrl); err != nil {
			return "", err
		}
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
	var (
		state    schema.ToolStatus
		tempPath string
		err      error
	)
	switch obj.Type {
	case schema.ToolSkill:
		tempPath, err = filex.Download(context.Background(), obj.Url, filepath.Join("skills", "downloads"))
		if err != nil {
			state = schema.ToolStatusOffline
		} else {
			eErr := filex.Extract(context.Background(), tempPath, "skills")
			if eErr != nil {
				state = schema.ToolStatusOffline
			} else {
				state = schema.ToolStatusOnline
			}
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
	if tempPath != "" {
		_ = os.Remove(tempPath)
	}
}
