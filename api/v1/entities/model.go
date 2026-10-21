package entities

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

type GetModelsRsp struct {
	ID       string `json:"id"`       // model id
	Provider string `json:"provider"` // 供应商
	Name     string `json:"name"`     // 模型
	Type     string `json:"type"`     // 类型
	Enable   bool   `json:"enable"`   // 是否可用
}

type GetModelInfoReq struct {
	ID string `uri:"id"` // 模型 id
}

type ApiKeyInfo struct {
	ID      int64  `json:"id"`      // api key id
	Name    string `json:"name"`    // 名称
	Version int64  `json:"version"` // 密钥版本
	Weight  int64  `json:"weight"`  // 权重
	Enable  bool   `json:"enable"`  // 是否可用
}

type UpdateModelReq struct {
	ID           string    `uri:"id" swaggerignore:"true"` // 模型 id
	Provider     *string   `json:"provider"`               // 供应商
	Name         *string   `json:"name"`                   // 模型名称
	Url          *string   `json:"url"`                    // 地址
	Enable       *bool     `json:"enable"`                 // 是否可用
	ContextLimit *int64    `json:"context_limit"`          // 上下文限制
	Reasoning    *KeyValue `json:"reasoning"`              // 深度思考
}

type AddApiKeyReq struct {
	ID     string `uri:"id" swaggerignore:"true"` // model id
	Key    string `json:"key"`                    // api key
	Weight int64  `json:"weight"`                 // 权重
	Name   string `json:"name"`                   // 名称，仅用于展示
}

type DeleteApiKeyReq struct {
	ID  string `uri:"id"`  // 模型 id
	Api int64  `uri:"api"` // api key id
}

type UpdateApiKeyReq struct {
	ID      string  `uri:"id" swaggerignore:"true"`  // 模型 id
	Api     int64   `uri:"api" swaggerignore:"true"` // api key id
	Name    *string `json:"name"`                    // 名称
	Upgrade bool    `json:"upgrade"`                 // 密钥轮换
	Weight  *int64  `json:"weight"`                  // 权重
	Enable  *bool   `json:"enable"`                  // 是否可用
}
