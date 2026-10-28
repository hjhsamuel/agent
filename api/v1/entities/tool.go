package entities

type AddToolReq struct {
	Name  string `json:"name"`  // 名称
	Kind  int    `json:"kind"`  // 类型。0：local tool; 1: a2a; 2: mcp; 3: skill
	Url   string `json:"url"`   // 远程服务地址，压缩包下载地址
	Agent string `json:"agent"` // agent id
}

type GetToolListReq struct {
	Offset string `json:"offset"` // 检索游标
	Limit  int64  `json:"limit"`  // 最大返回数量

	Agent *string `json:"agent"` // 关联 agent id，为空时表示检索未关联的
	Type  *int    `json:"type"`  // 类型
	State *int    `json:"state"` // 状态
}

type GetToolListRsp struct {
	Offset string        `json:"offset"` // 下次检索使用的游标
	Data   []*ToolOfList `json:"data"`
}

type ToolOfList struct {
	ToolBase
	Link bool `json:"link"` // 是否关联
}

type ToolBase struct {
	ID        string `json:"id"`         // tool id
	Name      string `json:"name"`       // 名称
	Type      int    `json:"type"`       // 类型
	State     int    `json:"state"`      // 状态
	Creator   string `json:"creator"`    // 创建者
	CreatedAt int64  `json:"created_at"` // 创建时间
}

type GetToolInfoReq struct {
	ID string `url:"id"` // tool id
}

type GetToolInfoRsp struct {
	ToolBase
	AgentName string `json:"agent_name"` // agent 名称
	AgentID   string `json:"agent_id"`   // agent id
}

type UpdateToolStateReq struct {
	ID    string `url:"id"`     // tool id
	State int    `json:"state"` // 状态
}

type UpdateToolReq struct {
	ID    string  `url:"id"`     // tool id
	Name  *string `json:"name"`  // 名称
	Url   *string `json:"url"`   // 地址
	Agent *string `json:"agent"` // agent id
}
