package request

type GetToolsReq struct {
	Agent *string
	Type  *int
	State *int

	Offset string
	Limit  int64
}

type UpdateToolReq struct {
	Name  *string
	Url   *string
	Agent *string
}
