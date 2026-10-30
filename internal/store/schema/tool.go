package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const ToolCollection = "cagent_tool"

type Tool struct {
	ID         bson.ObjectID  `bson:"_id,omitempty"`
	Name       string         `bson:"name"` // 工具名称
	Url        string         `bson:"url"`  // local/skill：压缩包下载地址；a2a/mcp：远端服务地址
	Type       ToolType       `bson:"type"`
	Credential *Credential    `bson:"credential"` // 认证信息
	Hash       *HashInfo      `bson:"hash"`       // 仅用于本地工具校验
	State      ToolStatus     `bson:"state"`
	Agent      *bson.ObjectID `bson:"agent,omitempty"`
	CreatedAt  time.Time      `bson:"created_at"`
	Creator    string         `bson:"creator"`
}

type ToolType int

const (
	ToolLocal ToolType = iota // local tool
	ToolA2A                   // a2a
	ToolMcp                   // mcp
	ToolSkill                 // skill
)

type Credential struct {
	Type  CredentialType `bson:"type"`
	Key   string         `bson:"key"`
	Value string         `bson:"value"`
}

type CredentialType int

const (
	CredentialHeaderApi CredentialType = iota // header-based API key, like 'X-Api-Key'
	CredentialDstBearer                       // the specific bearer token
	CredentialSvcBearer                       // the bearer token using of cagent server
)

type HashInfo struct {
	Type HashType `bson:"type"`
	Val  string   `bson:"val"`
}

type HashType int

const (
	HashMD5    HashType = iota // md5
	HashSHA256                 // sha256
	HashXXH3                   // xxh3
)

type ToolStatus int

const (
	ToolStatusLoading ToolStatus = iota // 准备中
	ToolStatusOnline
	ToolStatusOffline
)
