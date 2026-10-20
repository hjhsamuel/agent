package schema

import "go.mongodb.org/mongo-driver/v2/bson"

const ModelCollection = "model"

type ModelKind string

const (
	ChatModel      ModelKind = "chat"
	EmbeddingModel ModelKind = "embedding"
)

type Model struct {
	ID       bson.ObjectID `bson:"_id,omitempty"`
	Provider string        `bson:"provider"` // 供应商名称
	Name     string        `bson:"name"`     // 模型名称
	Url      string        `bson:"url"`
	Type     ModelKind     `bson:"type"`
	Seq      int64         `bson:"seq"`
	ApiKeys  []*ApiKey     `bson:"api_keys"`
	Options  ModelConfig   `bson:"options"`
	Enable   bool          `bson:"enable"`
}

type ModelConfig struct {
	ContextLimit int64    `bson:"context_limit"`
	Reasoning    *KeyPair `bson:"reasoning"`
}

type ApiKey struct {
	ID         int64  `bson:"id"`
	Version    int64  `bson:"version"`
	Ciphertext []byte `bson:"ciphertext"`
	Nonce      []byte `bson:"nonce"`
	Weight     int64  `bson:"weight"`
	Enable     bool   `bson:"enable"`
}

const (
	MinApiKeyWeight int64 = 1
	MaxApiKeyWeight int64 = 100
)
