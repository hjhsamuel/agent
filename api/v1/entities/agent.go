package entities

type AddMainAgentReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
}

type AddAgentGroupReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Level       int    `json:"level"` // 智能程度
}

type AddSubAgentReq struct {
	ID          string `uri:"id" swaggerignore:"true"` // 父级 agent id
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Level       int    `json:"level"` // 智能程度
}

type ModifyAgentReq struct {
	ID          string  `uri:"id" swaggerignore:"true"` // agent id
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Prompt      *string `json:"prompt"`
	Level       *int    `json:"level"`
}

type DeleteAgentReq struct {
	ID string `uri:"id"` // agent id
}

type AgentInfo struct {
	ID          string `json:"id"`          // agent id
	Name        string `json:"name"`        // 名称
	Description string `json:"description"` // 描述
	Prompt      string `json:"prompt"`      // 提示词
	Type        int    `json:"type"`        // 类型
	Level       int    `json:"level"`       // 智能程度
	CreatedAt   int64  `json:"created_at"`  // 创建时间
}

type GetSubAgentsReq struct {
	ID     string `uri:"id"`      // agent id
	Offset string `form:"offset"` // 检索游标
	Limit  int64  `form:"limit"`  // 最大返回数量
	Type   int    `form:"type"`   // 类型
}

type GetSubAgentsRsp struct {
	Offset string       `json:"offset"` // 下次检索游标
	Data   []*AgentInfo `json:"data"`
}
