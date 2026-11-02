package request

import "github.com/hjhsamuel/agent/internal/store/schema"

type GetToolsReq struct {
	Agent *string
	Type  *int
	State *int

	Offset string
	Limit  int64
}

type UpdateToolReq struct {
	Credential *schema.Credential
	Name       *string
	Url        *string
	Agent      *string
}
