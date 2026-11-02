package local

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"

	"github.com/hjhsamuel/agent/pkg/filex"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

var validName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

func ValidateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid local tool name %q", name)
	}
	return nil
}

type Executable struct {
	declaration genai.FunctionDeclaration
	dir, binary string
}

func Load(dir, name string) (*Executable, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	if dir == "" {
		return nil, fmt.Errorf("local tool has no installed version")
	}

	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "tool.json"))
	if err != nil {
		return nil, err
	}

	t := &Executable{dir: dir, binary: name}
	if runtime.GOOS == "windows" {
		t.binary += ".exe"
	}
	if err := json.Unmarshal(data, &t.declaration); err != nil {
		return nil, err
	}
	if t.Name() != name {
		return nil, fmt.Errorf("declaration name must equal %q", name)
	}
	info, err := os.Lstat(filepath.Join(dir, t.binary))
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("executable must be a regular file")
	}
	return t, nil
}

func (t *Executable) Name() string {
	return t.declaration.Name
}

func (t *Executable) Description() string {
	return t.declaration.Description
}

func (t *Executable) IsLongRunning() bool {
	return false
}

func (t *Executable) Declaration() *genai.FunctionDeclaration {
	return &t.declaration
}

func (t *Executable) ProcessRequest(ctx agent.Context, req *model.LLMRequest) error {
	return PackTool(req, t)
}

func (t *Executable) Run(ctx agent.Context, args any) (map[string]any, error) {
	return t.run(ctx, args)
}

func (t *Executable) run(ctx context.Context, args any) (map[string]any, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/d", "/s", "/c", t.binary)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", `exec "./`+t.binary+`"`)
	}
	cmd.Dir = t.dir
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("execute %s: %w", t.Name(), err)
	}
	var result map[string]any
	if err = json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("tool output must be a JSON object: %w", err)
	}
	return result, nil
}

// VersionDir derives a stable directory without persisting a host-specific path.
func VersionDir(name string, version int64) string {
	return filepath.Join("tools", name, strconv.FormatInt(version, 10))
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
	root := filepath.Join("tools", name)
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

	if _, err = Load(dir, name); err != nil {
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
