package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hjhsamuel/agent/pkg/tool"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetMCPTools discovers tools at a Streamable HTTP MCP endpoint.
func GetMCPTools(addr string) ([]tool.Tool, error) {
	return GetMCPToolsWithContext(context.Background(), addr, "")
}

// GetMCPToolsWithContext supports cancellation and authenticated discovery.
// The discovery token is not retained; Execute uses its own token.
func GetMCPToolsWithContext(ctx context.Context, addr, token string) ([]tool.Tool, error) {
	client := sdk.NewClient(&sdk.Implementation{Name: "agent", Version: "1.0.0"}, nil)
	session, err := connect(ctx, client, addr, token)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	name := strings.NewReplacer(" ", "_", "-", "_").Replace(session.InitializeResult().ServerInfo.Name)
	out := make([]tool.Tool, 0)
	// The iterator follows pagination cursors until all tools have been read.
	for definition, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list MCP tools: %w", err)
		}
		t, err := NewMCP(
			client,
			fmt.Sprintf("%s_%s_%s", tool.McpTool, name, definition.Name),
			addr,
			definition,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func connect(
	ctx context.Context,
	client *sdk.Client,
	addr string,
	token string,
) (*sdk.ClientSession, error) {
	session, err := client.Connect(
		ctx,
		&sdk.StreamableClientTransport{
			Endpoint: addr,
			HTTPClient: &http.Client{
				Transport: bearerTransport{
					base:  http.DefaultTransport,
					token: token,
				},
				// Do not forward credentials to a redirected endpoint.
				CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			},
			DisableStandaloneSSE: true,
		}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect MCP server: %w", err)
	}
	return session, nil
}

type bearerTransport struct {
	base  http.RoundTripper
	token string
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		req = req.Clone(req.Context())
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.base.RoundTrip(req)
}
