package schema

import "go.mongodb.org/mongo-driver/v2/bson"

const ProviderCollection = "provider"

type ModelKind string

const (
	ChatModel      ModelKind = "chat"
	EmbeddingModel ModelKind = "embedding"
)

type Provider struct {
	ID       bson.ObjectID `bson:"_id,omitempty"`
	Provider string        `bson:"provider"`
	Models   []*Model      `bson:"models"`
	Enabled  bool          `bson:"enabled"`
}

type Model struct {
	ID      string      `bson:"id"`
	Name    string      `bson:"name"`
	Url     string      `bson:"url"`
	Type    ModelKind   `bson:"type"`
	ApiKeys []*ApiKey   `bson:"api_keys"`
	Options ModelConfig `bson:"options"`
	Enable  bool        `bson:"enable"`
}

type ModelConfig struct {
	ContextLimit int64    `bson:"context_limit"`
	Reasoning    *KeyPair `bson:"reasoning"`
}

type ApiKey struct {
	ID         string `bson:"id"`
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
