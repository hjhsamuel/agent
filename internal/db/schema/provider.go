package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const ProviderCollection = "agent_provider"

type Provider struct {
	ID           bson.ObjectID      `bson:"_id,omitempty"`
	Provider     string             `bson:"provider"`     // 供应商名称，例如：GLM，DeekSeep
	Name         string             `bson:"name"`         // 模型名称，例如：glm-5.2，deepseek-v4-flash
	Url          string             `bson:"url"`          // api 地址
	Capabilities *ModelCapabilities `bson:"capabilities"` // 模型配置信息
	Type         ModelType          `bson:"type"`         // 模型类型
	Index        int64              `bson:"index"`        // api key 自增索引
	ApiKeys      []*ApiKey          `bson:"api_keys"`     // api key
	Enabled      bool               `bson:"enabled"`      // 是否可用
	Extra        map[string]any     `bson:"extra"`        // 额外参数
	CreatedAt    time.Time          `bson:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at"`
}

type ModelCapabilities struct {
	ContextLimit int64  `bson:"context_limit"` // 上下文限制
	Reasoning    string `bson:"reasoning"`     // 开启深度思考的key，为空表示不支持
}

type ApiKey struct {
	Index      int64     `bson:"index"`      // 索引
	Name       string    `bson:"name"`       // 名称
	Ciphertext []byte    `bson:"ciphertext"` // 加密后的 api key
	Nonce      []byte    `bson:"nonce"`      // nonce
	Version    int       `bson:"version"`    // 密钥版本
	Enabled    bool      `bson:"enabled"`    // 是否可用
	Weight     int       `bson:"weight"`     // 调度权重
	CreatedAt  time.Time `bson:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at"`
}

type ModelType string

const (
	ChatModel      ModelType = "chat"
	EmbeddingModel ModelType = "embedding"
	RerankModel    ModelType = "rerank"
	ImageModel     ModelType = "image"
	AudioModel     ModelType = "audio"
)
