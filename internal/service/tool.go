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
	var state schema.ToolStatus
	switch obj.Type {
	case schema.ToolSkill:
		err := s.downloadAndExtractTool(obj.Type, obj.Url)
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

func (s *Service) downloadAndExtractTool(kind schema.ToolType, baseUrl string) error {
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

		entries, err := os.ReadDir(tempExtractDir)
		if err != nil {
			return err
		}
		if len(entries) != 1 {
			return errors.New("invalid archive format")
		}

		return os.Rename(filepath.Join(tempExtractDir, entries[0].Name()),
			filepath.Join("skills", entries[0].Name()))
	} else {
		return errors.New("unknown tool kind")
	}
}
