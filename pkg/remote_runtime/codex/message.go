package codex

import (
	"encoding/json"
	"fmt"
)

// Message is a server notification or a server-initiated HITL request.
type Message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("codex RPC %d: %s", e.Code, e.Message) }

// Method names are shared by request dispatch and callers.
const (
	MethodInitialize                = "initialize"
	MethodInitialized               = "initialized"
	MethodThreadStart               = "thread/start"
	MethodThreadResume              = "thread/resume"
	MethodTurnStart                 = "turn/start"
	MethodAgentMessageDelta         = "item/agentMessage/delta"
	MethodReasoningSummaryTextDelta = "item/reasoning/summaryTextDelta"
	MethodReasoningSummaryPartAdded = "item/reasoning/summaryPartAdded"
	MethodReasoningTextDelta        = "item/reasoning/textDelta"
	MethodTurnCompleted             = "turn/completed"
	MethodCommandApproval           = "item/commandExecution/requestApproval"
	MethodFileChangeApproval        = "item/fileChange/requestApproval"
	MethodRequestUserInput          = "item/tool/requestUserInput"
	MethodMCPElicitation            = "mcpServer/elicitation/request"
)

// Request and Notification are outgoing protocol envelopes. Incoming messages
// retain raw payloads so dispatch can choose the appropriate concrete type.
type Request[T any] struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params T      `json:"params"`
}

type Notification struct {
	Method string `json:"method"`
}

type Response[T any] struct {
	ID     json.RawMessage `json:"id"`
	Result T               `json:"result"`
}

type ErrorResponse struct {
	ID    json.RawMessage `json:"id"`
	Error RPCError        `json:"error"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

type InitializeCapabilities struct {
	ExperimentalAPI bool `json:"experimentalApi"`
}

type InitializeParams struct {
	ClientInfo   ClientInfo             `json:"clientInfo"`
	Capabilities InitializeCapabilities `json:"capabilities"`
}

type InitializeResult struct {
	UserAgent string `json:"userAgent"`
}

type ThreadStartParams struct {
	ApprovalPolicy string `json:"approvalPolicy"`
}

type Thread struct {
	ID string `json:"id"`
}

type ThreadStartResult struct {
	Thread Thread `json:"thread"`
}

type ThreadResumeParams struct {
	ThreadID     string `json:"threadId"`
	ExcludeTurns bool   `json:"excludeTurns"`
}

type ThreadResumeResult struct {
	Thread Thread `json:"thread"`
}

type TextInput struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type TurnStartParams struct {
	ThreadID string      `json:"threadId"`
	Input    []TextInput `json:"input"`
}

type TurnError struct {
	Message           string          `json:"message"`
	CodexErrorInfo    json.RawMessage `json:"codexErrorInfo,omitempty"` // Protocol union.
	AdditionalDetails *string         `json:"additionalDetails,omitempty"`
}

type Turn struct {
	ID     string     `json:"id"`
	Status string     `json:"status"`
	Error  *TurnError `json:"error"`
}

type TurnStartResult struct {
	Turn Turn `json:"turn"`
}

type AgentMessageDeltaParams struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	ItemID   string `json:"itemId"`
	Delta    string `json:"delta"`
}

type TurnCompletedParams struct {
	ThreadID string `json:"threadId"`
	Turn     Turn   `json:"turn"`
}

// ReasoningSummaryTextDeltaParams carries readable summary text, not the
// complete internal reasoning. Group deltas by itemId and summaryIndex.
type ReasoningSummaryTextDeltaParams struct {
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
	ItemID       string `json:"itemId"`
	Delta        string `json:"delta"`
	SummaryIndex int    `json:"summaryIndex"`
}

type ReasoningSummaryPartAddedParams struct {
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
	ItemID       string `json:"itemId"`
	SummaryIndex int    `json:"summaryIndex"`
}

// ReasoningTextDeltaParams is emitted only when the model/server exposes it.
// Group deltas by itemId and contentIndex.
type ReasoningTextDeltaParams struct {
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
	ItemID       string `json:"itemId"`
	Delta        string `json:"delta"`
	ContentIndex int    `json:"contentIndex"`
}

// HITL payloads can be decoded with DecodeParams inside Handler.
type CommandApprovalParams struct {
	ThreadID string  `json:"threadId"`
	TurnID   string  `json:"turnId"`
	ItemID   string  `json:"itemId"`
	Command  *string `json:"command,omitempty"`
	Cwd      *string `json:"cwd,omitempty"`
	Reason   *string `json:"reason,omitempty"`
}

type FileChangeApprovalParams struct {
	ThreadID  string  `json:"threadId"`
	TurnID    string  `json:"turnId"`
	ItemID    string  `json:"itemId"`
	Reason    *string `json:"reason,omitempty"`
	GrantRoot *string `json:"grantRoot,omitempty"`
}

// ApprovalResult handles simple decisions: accept, decline, cancel, acceptForSession.
type ApprovalResult struct {
	Decision string `json:"decision"`
}

type UserInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type UserInputQuestion struct {
	ID       string            `json:"id"`
	Header   string            `json:"header"`
	Question string            `json:"question"`
	IsOther  bool              `json:"isOther"`
	IsSecret bool              `json:"isSecret"`
	Options  []UserInputOption `json:"options"`
}

type UserInputParams struct {
	ThreadID  string              `json:"threadId"`
	TurnID    string              `json:"turnId"`
	ItemID    string              `json:"itemId"`
	Questions []UserInputQuestion `json:"questions"`
}

type UserInputAnswer struct {
	Answers []string `json:"answers"`
}

type UserInputResult struct {
	Answers map[string]UserInputAnswer `json:"answers"`
}

type MCPElicitationParams struct {
	ThreadID        string          `json:"threadId"`
	ServerName      string          `json:"serverName"`
	Mode            string          `json:"mode"`
	Message         string          `json:"message"`
	RequestedSchema json.RawMessage `json:"requestedSchema,omitempty"` // Arbitrary JSON Schema.
	URL             string          `json:"url,omitempty"`
	ElicitationID   string          `json:"elicitationId,omitempty"`
}

type MCPElicitationResult struct {
	Action  string          `json:"action"`
	Content json.RawMessage `json:"content,omitempty"` // Data defined by requestedSchema.
}

func DecodeParams[T any](message Message) (T, error) {
	var params T
	err := json.Unmarshal(message.Params, &params)
	return params, err
}

// StreamEvent is a decoded streaming notification. OnStreamEvent receives value
// types; use a type switch to distinguish the four payloads below. Method can
// be used as the SSE event name, with the payload JSON-encoded as data.
type StreamEvent interface {
	Method() string
}

func (AgentMessageDeltaParams) Method() string {
	return MethodAgentMessageDelta
}

func (ReasoningSummaryTextDeltaParams) Method() string {
	return MethodReasoningSummaryTextDelta
}

func (ReasoningSummaryPartAddedParams) Method() string {
	return MethodReasoningSummaryPartAdded
}

func (ReasoningTextDeltaParams) Method() string {
	return MethodReasoningTextDelta
}
