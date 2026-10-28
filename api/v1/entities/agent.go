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
	ID          string `uri:"id"` // 父级 agent id
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Level       int    `json:"level"` // 智能程度
}

type ModifyAgentReq struct {
	ID          string  `uri:"id"` // agent id
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Prompt      *string `json:"prompt"`
	Level       *int    `json:"level"`
}

type DeleteAgentReq struct {
	ID string `uri:"id"` // agent id
}
