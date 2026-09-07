// Package ir defines the agentcfg intermediate representation (IR)
// described in docs/protocol.md.
package ir

// Protocol is a provider request protocol.
type Protocol string

const (
	ProtocolOpenAICompletions Protocol = "openai-completions"
	ProtocolOpenAIResponses   Protocol = "openai-responses"
	ProtocolAnthropicMessages Protocol = "anthropic-messages"
)

// Transport is an MCP server transport.
type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportHTTP  Transport = "http"
)

// Modality is an input or output modality.
type Modality string

const (
	ModalityText  Modality = "text"
	ModalityImage Modality = "image"
	ModalityAudio Modality = "audio"
	ModalityVideo Modality = "video"
	ModalityPDF   Modality = "pdf"
)

// HeaderValue is exactly one of a constant value, an environment
// reference, or an Authorization bearer environment reference.
type HeaderValue struct {
	Value         string `yaml:"value"`
	FromEnv       string `yaml:"from_env"`
	BearerFromEnv string `yaml:"bearer_from_env"`
}

// Model describes one model served by a provider.
type Model struct {
	ID              string     `yaml:"id"`
	Name            string     `yaml:"name"`
	ContextWindow   *int64     `yaml:"context_window"`
	MaxOutputTokens *int64     `yaml:"max_output_tokens"`
	Input           []Modality `yaml:"input"`
	Output          []Modality `yaml:"output"`
	Reasoning       *bool      `yaml:"reasoning"`
	ToolCalling     *bool      `yaml:"tool_calling"`
}

// Provider is one model provider route.
type Provider struct {
	ID        string                 `yaml:"id"`
	Name      string                 `yaml:"name"`
	Protocol  Protocol               `yaml:"protocol"`
	BaseURL   string                 `yaml:"base_url"`
	APIKeyEnv string                 `yaml:"api_key_env"`
	Headers   map[string]HeaderValue `yaml:"headers"`
	Models    []Model                `yaml:"models"`
}

// MCPServer is one MCP server.
type MCPServer struct {
	ID        string                 `yaml:"id"`
	Transport Transport              `yaml:"transport"`
	Enabled   *bool                  `yaml:"enabled"`
	Command   []string               `yaml:"command"`
	Env       map[string]HeaderValue `yaml:"env"`
	URL       string                 `yaml:"url"`
	Headers   map[string]HeaderValue `yaml:"headers"`
	CWD       string                 `yaml:"cwd"`
	TimeoutMS *int64                 `yaml:"timeout_ms"`
}

// Defaults selects the default provider route and model.
type Defaults struct {
	Model string `yaml:"model"`
}

// Config is the root of agentcfg.yaml.
type Config struct {
	Version   int         `yaml:"version"`
	Providers []Provider  `yaml:"providers"`
	MCP       []MCPServer `yaml:"mcp"`
	Defaults  *Defaults   `yaml:"defaults"`
	Targets   []string    `yaml:"targets"`
}
