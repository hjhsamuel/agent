package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/hjhsamuel/agent/pkg/charx"
	"github.com/hjhsamuel/agent/pkg/filex"
	"go.mongodb.org/mongo-driver/v2/bson"
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
	name, err := charx.RandomString(6)
	if err != nil {
		name = uuid.NewString()
	}

	if kind == schema.ToolSkill {
		var (
			tempDownloadDir = filepath.Join("skills", "downloads", name)
			tempExtractDir  = filepath.Join("skills", "extracts", name)
		)
		defer os.Remove(tempDownloadDir)
		defer os.Remove(tempExtractDir)

		dp, err := filex.Download(context.Background(), baseUrl, tempDownloadDir)
		if err != nil {
			return err
		}

		err = filex.Extract(context.Background(), dp, tempExtractDir)
		if err != nil {
			return err
		}

		// 检查是否符合skill目录格式
		entries, err := os.ReadDir(tempExtractDir)
		if err != nil {
			return err
		}
		if len(entries) != 1 {
			return os.Rename(tempExtractDir, filepath.Join("skills", toolName))
		} else {
			return os.Rename(filepath.Join(tempExtractDir, entries[0].Name()),
				filepath.Join("skills", toolName))
		}
	} else {
		return errors.New("unknown tool kind")
	}
}
