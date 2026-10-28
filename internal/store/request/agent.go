package request

type UpdateAgentReq struct {
	Name        *string
	Description *string
	Prompt      *string
	Level       *int
}

type GetAgentsReq struct {
	Offset string
	Limit  int64
	Type   int
	Parent string
}
