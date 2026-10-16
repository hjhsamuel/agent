package schema

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

type StreamReq struct {
	ID string `form:"id"` // 会话 id
}
