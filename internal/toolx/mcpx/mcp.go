package mcpx

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/v2/auth"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/mcptoolset"
)

func NewMCP(path string, cred auth.CredentialProvider) (tool.Toolset, error) {
	return mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.StreamableClientTransport{
			Endpoint: path,
		},
		Auth: cred,
	})
}
