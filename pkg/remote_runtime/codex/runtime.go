// Package codex runs one app-server process per user over newline-delimited JSON.
package codex

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Config belongs to one user. ConfigTOML is that user's complete Codex config,
// including [mcp_servers.*]. Credentials must be provisioned in the user's home
// or supplied through Env. Do not run two instances with the same user/home.
type Config struct {
	Binary     string
	RootDir    string
	UserID     string
	ConfigTOML string
	Dir        string
	Env        []string
	Output     io.Writer
	Stderr     io.Writer
	// Stream callbacks run synchronously in receive order. They must honor ctx
	// and must not reenter Runtime. Errors propagate to the current operation.
	// Output still receives only assistant text; use io.Discard to disable it.
	OnStreamEvent func(context.Context, StreamEvent) error
}

// Handler returns the protocol result for a server request (approval, user
// input, MCP elicitation, etc.). It must honor ctx and must not reenter Runtime.
type Handler func(ctx context.Context, request Message) (any, error)

type Runtime struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	messages  chan Message
	done      chan struct{}
	readErr   error
	waitErr   error
	output    io.Writer
	mu        sync.Mutex // Serializes conversations and protocol writes.
	closeOnce sync.Once
	nextID    int
	config    Config
}

// Start starts and initializes the server. ctx bounds initialization only;
// callers own the process and must call Close.
func Start(ctx context.Context, cfg Config) (*Runtime, error) {
	if cfg.RootDir == "" || cfg.UserID == "" {
		return nil, errors.New("codex: RootDir and UserID are required")
	}
	home, err := filepath.Abs(filepath.Join(cfg.RootDir, fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.UserID)))))
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(home, 0700); err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(home, "config.toml"), []byte(cfg.ConfigTOML), 0600); err != nil {
		return nil, err
	}
	binary := cfg.Binary
	if binary == "" {
		binary = "codex"
	}
	cmd := exec.Command(binary, "app-server", "--listen", "stdio://")
	cmd.Dir = cfg.Dir
	for _, entry := range append(os.Environ(), cfg.Env...) {
		if !strings.EqualFold(strings.SplitN(entry, "=", 2)[0], "CODEX_HOME") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "CODEX_HOME="+home)
	cmd.Stderr = cfg.Stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}
	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}
	r := &Runtime{
		cmd:      cmd,
		stdin:    stdin,
		messages: make(chan Message),
		done:     make(chan struct{}),
		output:   output,
		config:   cfg,
	}
	go func() {
		defer close(r.messages)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 4096), 16*1024*1024)
		for scanner.Scan() {
			var msg Message
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				r.readErr = err
				break
			}
			select {
			case r.messages <- msg:
			case <-r.done:
				return
			}
		}
		if r.readErr == nil {
			r.readErr = scanner.Err()
		}
	}()
	// Wait only after stdout has been consumed; Close kills the process if needed.
	_, err = r.call(
		ctx,
		MethodInitialize,
		InitializeParams{ClientInfo: ClientInfo{Name: "agent", Title: "Agent", Version: "1.0.0"}, Capabilities: InitializeCapabilities{ExperimentalAPI: true}},
		nil,
	)
	if err == nil {
		err = r.write(Notification{Method: MethodInitialized})
	}
	if err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}

// Close is idempotent and terminates and reaps the app-server process.
func (r *Runtime) Close() error {
	r.closeOnce.Do(func() { close(r.done); r.stdin.Close(); _ = r.cmd.Process.Kill(); r.waitErr = r.cmd.Wait() })
	// A forced termination is expected here.
	var exit *exec.ExitError
	if errors.As(r.waitErr, &exit) {
		return nil
	}
	return r.waitErr
}

func (r *Runtime) write(value any) error {
	return json.NewEncoder(r.stdin).Encode(value)
}

func (r *Runtime) receive(ctx context.Context) (Message, error) {
	select {
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case <-r.done:
		return Message{}, errors.New("codex: closed")
	case msg, ok := <-r.messages:
		if !ok {
			if r.readErr != nil {
				return Message{}, r.readErr
			}
			return Message{}, io.EOF
		}
		return msg, nil
	}
}

func (r *Runtime) dispatch(ctx context.Context, msg Message, handler Handler) error {
	if len(msg.ID) != 0 && msg.Method != "" {
		var result any
		var err error
		if handler == nil {
			err = errors.New("no HITL handler configured")
		} else {
			result, err = handler(ctx, msg)
		}
		if err != nil {
			return r.write(ErrorResponse{ID: msg.ID, Error: RPCError{Code: -32603, Message: err.Error()}})
		}
		return r.write(Response[any]{ID: msg.ID, Result: result})
	}
	switch msg.Method {
	case MethodAgentMessageDelta:
		var p AgentMessageDeltaParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return err
		}
		if r.config.OnStreamEvent != nil {
			if err := r.config.OnStreamEvent(ctx, p); err != nil {
				return err
			}
		}
		_, err := io.WriteString(r.output, p.Delta)
		return err
	case MethodReasoningSummaryTextDelta:
		return handleStream[ReasoningSummaryTextDeltaParams](ctx, msg, r.config.OnStreamEvent)
	case MethodReasoningSummaryPartAdded:
		return handleStream[ReasoningSummaryPartAddedParams](ctx, msg, r.config.OnStreamEvent)
	case MethodReasoningTextDelta:
		return handleStream[ReasoningTextDeltaParams](ctx, msg, r.config.OnStreamEvent)
	}
	return nil
}

func handleStream[T StreamEvent](ctx context.Context, msg Message, handler func(context.Context, StreamEvent) error) error {
	params, err := DecodeParams[T](msg)
	if err != nil {
		return err
	}
	if handler != nil {
		return handler(ctx, params)
	}
	return nil
}

func (r *Runtime) call(ctx context.Context, method string, params any, handler Handler) (json.RawMessage, error) {
	r.nextID++
	id := r.nextID
	if err := r.write(Request[any]{ID: id, Method: method, Params: params}); err != nil {
		return nil, err
	}
	for {
		msg, err := r.receive(ctx)
		if err != nil {
			return nil, err
		}
		if msg.Method == "" && string(msg.ID) == fmt.Sprint(id) {
			if msg.Error != nil {
				return nil, msg.Error
			}
			return msg.Result, nil
		}
		if err = r.dispatch(ctx, msg, handler); err != nil {
			return nil, err
		}
	}
}

// StartThread creates a conversation. Reuse its ID for subsequent Chat calls.
func (r *Runtime) StartThread(ctx context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	raw, err := r.call(ctx, MethodThreadStart, ThreadStartParams{ApprovalPolicy: "on-request"}, nil)
	if err != nil {
		return "", err
	}
	var result ThreadStartResult
	err = json.Unmarshal(raw, &result)
	return result.Thread.ID, err
}

// Chat prints streamed assistant text and waits for completion. Cancellation
// closes the runtime, ensuring a canceled turn cannot continue in the background.
func (r *Runtime) Chat(ctx context.Context, threadID, text string, handler Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer func() {
		if ctx.Err() != nil {
			_ = r.Close()
		}
	}()
	_, err := r.call(ctx, MethodTurnStart, TurnStartParams{ThreadID: threadID, Input: []TextInput{{Type: "text", Text: text}}}, handler)
	if err != nil {
		return err
	}
	for {
		msg, err := r.receive(ctx)
		if err != nil {
			return err
		}
		if err = r.dispatch(ctx, msg, handler); err != nil {
			return err
		}
		if msg.Method == MethodTurnCompleted {
			var p TurnCompletedParams
			if err = json.Unmarshal(msg.Params, &p); err != nil {
				return err
			}
			if p.ThreadID != threadID {
				continue
			}
			if p.Turn.Status != "completed" {
				return fmt.Errorf("codex turn %s: %+v", p.Turn.Status, p.Turn.Error)
			}
			_, err = fmt.Fprintln(r.output)
			return err
		}
	}
}
