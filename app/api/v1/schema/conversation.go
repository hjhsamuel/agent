package schema

type SSEStreamReq struct {
	ID  string `form:"id"`
	Seq uint64 `form:"seq"`
}

type ChatReq struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}
