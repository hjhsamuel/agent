package request

import "go.mongodb.org/mongo-driver/v2/bson"

type GroupModel struct {
	Provider string       // 供应商名称
	Models   []*ModelInfo // 模型信息
}

type ModelInfo struct {
	ID     bson.ObjectID
	Name   string
	Type   string
	Enable bool
	Level  int
}

type UpdateModelReq struct {
	Provider     *string
	Name         *string
	Url          *string
	ContextLimit *int64
	Reasoning    *struct {
		Key   string
		Value string
	}
	Enable *bool
	Level  *int
}

type UpdateApiKeyReq struct {
	Name    *string
	Upgrade bool
	Weight  *int64
	Enable  *bool
}
