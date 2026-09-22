package schema

type SSEStreamReq struct {
	ID  string `form:"id"`
	Seq uint64 `form:"seq"`
}

type ChatReq struct {
	ID      string `json:"id" uri:"id"`
	Content string `json:"content"`
}

type TaskInputReq struct {
	ContextID string `json:"context_id"`
	TaskID    string `json:"task_id"`
	Content   string `json:"content"`
}
