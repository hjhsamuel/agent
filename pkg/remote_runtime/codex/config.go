package codex

import (
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

// CodexConfig is serialized to the user's config.toml. Zero values leave
// optional settings to Codex defaults. Pointer booleans preserve explicit false.
// See https://developers.openai.com/codex/config-reference.
type CodexConfig struct {
	Model                 string                         `toml:"model,omitempty"`
	ModelProvider         string                         `toml:"model_provider,omitempty"`
	ModelReasoningEffort  string                         `toml:"model_reasoning_effort,omitempty"`
	ModelReasoningSummary string                         `toml:"model_reasoning_summary,omitempty"`
	ApprovalPolicy        string                         `toml:"approval_policy,omitempty"`
	SandboxMode           string                         `toml:"sandbox_mode,omitempty"`
	ModelProviders        map[string]ModelProviderConfig `toml:"model_providers,omitempty"`
	MCPServers            map[string]MCPServerConfig     `toml:"mcp_servers,omitempty"`
	SandboxWorkspaceWrite *SandboxWorkspaceWriteConfig   `toml:"sandbox_workspace_write,omitempty"`
}

// ModelProviderConfig configures a Responses-compatible model endpoint.
// EnvKey names an environment variable; supply its value using Config.Env.
type ModelProviderConfig struct {
	Name               string            `toml:"name"`
	BaseURL            string            `toml:"base_url,omitempty"`
	EnvKey             string            `toml:"env_key,omitempty"`
	WireAPI            string            `toml:"wire_api,omitempty"`
	RequiresOpenAIAuth *bool             `toml:"requires_openai_auth,omitempty"`
	SupportsWebsockets *bool             `toml:"supports_websockets,omitempty"`
	HTTPHeaders        map[string]string `toml:"http_headers,omitempty"`
	EnvHTTPHeaders     map[string]string `toml:"env_http_headers,omitempty"`
	QueryParams        map[string]string `toml:"query_params,omitempty"`
}

// MCPServerConfig supports either a local stdio command or a streamable HTTP
// URL. Timeout values are in seconds. Env belongs to the MCP child process;
// bearer_token_env_var and env_http_headers refer to app-server environment.
type MCPServerConfig struct {
	Command           string            `toml:"command,omitempty"`
	Args              []string          `toml:"args,omitempty"`
	Env               map[string]string `toml:"env,omitempty"`
	EnvVars           []string          `toml:"env_vars,omitempty"`
	Cwd               string            `toml:"cwd,omitempty"`
	URL               string            `toml:"url,omitempty"`
	BearerTokenEnvVar string            `toml:"bearer_token_env_var,omitempty"`
	HTTPHeaders       map[string]string `toml:"http_headers,omitempty"`
	EnvHTTPHeaders    map[string]string `toml:"env_http_headers,omitempty"`
	Enabled           *bool             `toml:"enabled,omitempty"`
	Required          *bool             `toml:"required,omitempty"`
	StartupTimeoutSec *float64          `toml:"startup_timeout_sec,omitempty"`
	ToolTimeoutSec    *float64          `toml:"tool_timeout_sec,omitempty"`
	EnabledTools      []string          `toml:"enabled_tools,omitempty"`
	DisabledTools     []string          `toml:"disabled_tools,omitempty"`
}

type SandboxWorkspaceWriteConfig struct {
	WritableRoots       []string `toml:"writable_roots,omitempty"`
	NetworkAccess       *bool    `toml:"network_access,omitempty"`
	ExcludeTmpdirEnvVar *bool    `toml:"exclude_tmpdir_env_var,omitempty"`
	ExcludeSlashTmp     *bool    `toml:"exclude_slash_tmp,omitempty"`
}

// MarshalTOML validates transport choices before encoding the configuration.
func (c CodexConfig) MarshalTOML() ([]byte, error) {
	for name, p := range c.ModelProviders {
		if name == "" || p.Name == "" {
			return nil, fmt.Errorf("codex: model provider %q requires a name", name)
		}
		if p.WireAPI != "" && p.WireAPI != "responses" {
			return nil, fmt.Errorf("codex: model provider %q requires responses wire API", name)
		}
	}
	for name, m := range c.MCPServers {
		if name == "" || (m.Command == "") == (m.URL == "") {
			return nil, fmt.Errorf("codex: MCP server %q requires exactly one of command or URL", name)
		}
		if m.URL != "" && (len(m.Args) > 0 || len(m.Env) > 0 || len(m.EnvVars) > 0 || m.Cwd != "") {
			return nil, fmt.Errorf("codex: HTTP MCP server %q contains stdio settings", name)
		}
		if m.Command != "" && (m.BearerTokenEnvVar != "" || len(m.HTTPHeaders) > 0 || len(m.EnvHTTPHeaders) > 0) {
			return nil, fmt.Errorf("codex: stdio MCP server %q contains HTTP settings", name)
		}
		if (m.StartupTimeoutSec != nil && *m.StartupTimeoutSec < 0) || (m.ToolTimeoutSec != nil && *m.ToolTimeoutSec < 0) {
			return nil, fmt.Errorf("codex: MCP server %q has a negative timeout", name)
		}
	}
	// Alias avoids invoking this method recursively in TOML encoders.
	type config CodexConfig
	return toml.Marshal(config(c))
}
