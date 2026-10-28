package request

type UpdateAgentReq struct {
	Name        *string
	Description *string
	Prompt      *string
	Level       *int
}
