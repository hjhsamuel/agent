package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/hjhsamuel/agent/pkg/filex"
	"github.com/hjhsamuel/agent/pkg/skillx"
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
