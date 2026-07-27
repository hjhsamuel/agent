package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/hjhsamuel/agent/pkg"
)

const WorkSpace = "workspace"

type Shell struct {
	defTimeout time.Duration
}

func (s *Shell) Basic() *ToolDefine {
	return &ToolDefine{
		Name:        "shell",
		Description: "Run a shell command inside the workspace.",
		Parameter: schema(map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to run.",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "The number of seconds to wait before timing out.",
				"minimum":     1,
				"maximum":     300,
			},
		}, "command", "timeout"),
	}
}

func (s *Shell) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	ws, ok := ctx.Value(WorkSpace).(string)
	if !ok {
		return nil, errors.New("workspace not set")
	}

	var req *ShellReq
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("invalid arguments: %v", err)
	}
	if req.Command == "" {
		return nil, errors.New("empty command")
	}

	var limit time.Duration
	if req.Timeout <= 0 {
		limit = s.defTimeout
	} else {
		limit = time.Duration(req.Timeout) * time.Second
	}

	runCtx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(runCtx, "powershell", "-NoProfile", "-NonInteractive", "-Command",
			req.Command)
	} else {
		cmd = exec.CommandContext(runCtx, "/bin/sh", "-c", req.Command)
	}
	cmd.Dir = ws

	stdout, stderr := pkg.NewCmdOutput(10000), pkg.NewCmdOutput(10000)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		content := stderr.String()
		return &ToolResult{Content: content}, fmt.Errorf("command failed: %v", err)
	}
	return &ToolResult{
		Content: stdout.String(),
		Details: map[string]any{
			"exit_code": 0,
		},
	}, nil
}

func (s *Shell) RequireApproval() bool {
	return true
}

func NewShell(timeout time.Duration) Tool {
	return &Shell{
		defTimeout: timeout,
	}
}

type ShellReq struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}
