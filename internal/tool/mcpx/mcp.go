package mcpx

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/v2/auth"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/mcptoolset"
)

func NewMCP(user string, path string) (tool.Toolset, error) {
	return mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.StreamableClientTransport{
			Endpoint: path,
		},
		Auth: auth.APIKey("X-User-ID", user),
	})
}
