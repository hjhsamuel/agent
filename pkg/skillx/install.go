package skillx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hjhsamuel/agent/pkg/filex"
)

// VersionDir derives a stable directory without persisting a host-specific path.
func VersionDir(name string, version int64) string {
	return filepath.Join("skills", name, strconv.FormatInt(version, 10))
}

// Install creates an immutable version retained for existing runners.
func Install(ctx context.Context, name, url string, version int64) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if version <= 0 {
		return "", fmt.Errorf("invalid tool version %d", version)
	}

	destination := VersionDir(name, version)
	root := filepath.Join("skills", name)
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}

	stage, err := os.MkdirTemp(root, ".staging-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)

	archive, err := filex.Download(ctx, url, filepath.Join(stage, "download"))
	if err != nil {
		return "", err
	}

	dir := filepath.Join(stage, "payload")
	if err = filex.Extract(ctx, archive, dir); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		dir = filepath.Join(dir, entries[0].Name())
	}

	if err = ValidateSkill(dir, name); err != nil {
		return "", err
	}
	// Never replace an existing version, including one retained after deletion.
	if _, err := os.Lstat(destination); err == nil {
		return "", fmt.Errorf("tool version already exists")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(dir, destination); err != nil {
		return "", err
	}
	dir, err = filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	return dir, nil
}
