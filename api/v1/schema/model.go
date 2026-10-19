package schema

type AddModelReq struct {
	Provider     string    `json:"provider"`      // 供应商名称，仅用于展示，例如：GLM、DeepSeek
	Name         string    `json:"name"`          // 模型名称，例如：glm-5.3，deepseek-v4-pro
	Url          string    `json:"url"`           // 地址
	Type         string    `json:"type"`          // 模型类型。chat：对话模型；embedding：嵌入模型
	ContextLimit int64     `json:"context_limit"` // 上下文限制
	Reasoning    *KeyValue `json:"reasoning"`     // 深度思考
}

type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
