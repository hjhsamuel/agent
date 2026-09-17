package schema

type FinishActiveReq struct {
	TaskId     string // 异步任务id
	ToolCallId string
	Content    string
}
