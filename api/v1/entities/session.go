package entities

type GetSessionListReq struct {
	Offset string `form:"offset"` // 索引，默认为空
	Limit  int    `form:"limit"`  // 最大返回数量
}

type GetSessionListRsp struct {
	Offset string         `json:"offset"` // 下次检索使用的索引
	Data   []*SessionBase `json:"data"`
}

type SessionBase struct {
	ID        string `json:"id"`         // 会话 id
	Name      string `json:"name"`       // 会话标题
	State     int    `json:"state"`      // 状态。1：运行中；2：结束；3：等待用户输入
	CreatedAt int64  `json:"created_at"` // 创建时间
	UpdatedAt int64  `json:"updated_at"` // 最后对话时间
}

type DeleteSessionReq struct {
	ID string `uri:"id"` // 会话 id
}

type StreamReq struct {
	ID  string `form:"id"`  // 会话 id
	Seq uint64 `form:"seq"` // sse chunk id
}

type ChatReq struct {
	ID      string `uri:"id" swaggerignore:"true"` // 会话 id
	Content string `json:"content"`                // 对话消息
}

type GetChatMessagesReq struct {
	ID     string `uri:"id"`      // 会话 id
	Offset int64  `form:"offset"` // 消息索引，默认为0
	Limit  int    `form:"limit"`  // 最大轮次数量
}

type GetChatMessagesRsp struct {
	Offset int64          `json:"offset"` // 消息索引
	Data   []*MessageInfo `json:"data"`
}

type TurnMessage struct {
	Offset  int64          `json:"offset"`
	Message []*MessageInfo `json:"message"`
}

type MessageInfo struct {
	Role      string `json:"role"`       // 角色
	Content   string `json:"content"`    // 内容
	CreatedAt int64  `json:"created_at"` // 创建时间
}

type InputRequiredReq struct {
	ID      string `uri:"id" swaggerignore:"true"` // 会话 id
	CallID  string `json:"call_id"`                // 任务 id
	Content string `json:"content"`                // 用户补充内容
}

type WaitInputListReq struct {
	ID string `uri:"id"` // 会话 id
}

type WaitInputInfo struct {
	ID      string `json:"id"`      // call id
	Content string `json:"content"` // 提示内容
}
