package mcpx

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/mcptoolset"
)

func NewMCP(path string) (tool.Toolset, error) {
	transport := &mcp.StreamableClientTransport{
		Endpoint: path,
	}
	return mcptoolset.New(mcptoolset.Config{Transport: transport})
}
