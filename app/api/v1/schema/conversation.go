package schema

type SSEStreamReq struct {
	ID  string `uri:"id"`
	Seq uint64 `form:"seq"`
}
