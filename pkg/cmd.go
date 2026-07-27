package pkg

import (
	"bytes"
	"fmt"
)

type CmdOutput struct {
	buffer    bytes.Buffer
	maxBytes  int64
	discarded int64
}

func (o *CmdOutput) Write(p []byte) (int, error) {
	remain := o.maxBytes - int64(o.buffer.Len())
	dataSize := int64(len(p))
	if remain > 0 {
		keep := min(dataSize, remain)
		o.buffer.Write(p[:keep])
		o.discarded += dataSize - keep
	} else {
		o.discarded += dataSize
	}
	return len(p), nil
}

func (o *CmdOutput) String() string {
	content := o.buffer.String()
	if o.discarded > 0 {
		content += fmt.Sprintf("\n...[output truncated; %d bytes omitted]", o.discarded)
	}
	return content
}

func NewCmdOutput(limit int64) *CmdOutput {
	return &CmdOutput{
		maxBytes: limit,
	}
}
