package mcpx

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/v2/auth"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/mcptoolset"
)

func NewTransport(path string) mcp.Transport {
	return &mcp.StreamableClientTransport{
		Endpoint: path,
	}
}

func NewMCP(transport mcp.Transport, user string) (tool.Toolset, error) {
	return mcptoolset.New(mcptoolset.Config{
		Transport: transport,
		Auth:      auth.APIKey("X-User-ID", user),
	})
}
