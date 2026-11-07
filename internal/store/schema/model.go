package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const ModelCollection = "cagent_model"

type ModelKind string

const (
	ChatModel      ModelKind = "chat"
	EmbeddingModel ModelKind = "embedding"
)

const ModelCacheTTL = time.Minute * 5

type ApiKind int

const (
	ChatCompletions ApiKind = iota
	Responses
)

type Model struct {
	ID       bson.ObjectID `bson:"_id,omitempty"`
	Provider string        `bson:"provider"` // 供应商名称
	Name     string        `bson:"name"`     // 模型名称
	Url      string        `bson:"url"`
	Type     ModelKind     `bson:"type"`
	Mode     ApiKind       `bson:"mode"` // 使用的 api 请求类型
	Seq      int64         `bson:"seq"`
	ApiKeys  []*ApiKey     `bson:"api_keys"`
	Options  ModelConfig   `bson:"options"`
	Enable   bool          `bson:"enable"`
	Level    ModelLevel    `bson:"level"`
}

type ModelConfig struct {
	ContextLimit int64          `bson:"context_limit"` // 上下文限制
	Thinking     map[string]any `bson:"thinking"`      // 开启深度思考使用的配置
	Extra        map[string]any `bson:"extra"`         // 永久启用的配置
}

type ApiKey struct {
	ID         int64  `bson:"id"`
	Name       string `bson:"name"`
	Version    int64  `bson:"version"`
	Ciphertext []byte `bson:"ciphertext"`
	Nonce      []byte `bson:"nonce"`
	Weight     int64  `bson:"weight"`
	Enable     bool   `bson:"enable"`
}

type ModelLevel int

const (
	ModelLevelLow ModelLevel = iota
	ModelLevelMedium
	ModelLevelHigh
	ModelLevelExtraHigh
	ModelLevelUltra
)

const (
	MinApiKeyWeight int64 = 1
	MaxApiKeyWeight int64 = 100
)
