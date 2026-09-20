// Package ir defines the agentcfg intermediate representation (IR)
// described in docs/protocol.md.
package ir

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Protocol is a provider request protocol.
type Protocol string

const (
	ProtocolOpenAICompletions Protocol = "openai-completions"
	ProtocolOpenAIResponses   Protocol = "openai-responses"
	ProtocolAnthropicMessages Protocol = "anthropic-messages"
)

// AuthType is how a provider credential is presented to the upstream. It
// overrides the credential injection the selected protocol uses natively.
// The empty value means AuthTypeOfficial.
type AuthType string

const (
	AuthTypeOfficial AuthType = "official"
	AuthTypeBearer   AuthType = "bearer"
	AuthTypeNone     AuthType = "none"
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
	Value         string `yaml:"-"`
	FromEnv       string `yaml:"-"`
	BearerFromEnv string `yaml:"-"`
}

// ReasoningEffort is one model-provider supplied selectable reasoning level.
// Its name is passed through unchanged; it is not a model identifier or output setting.
type ReasoningEffort string

// Model describes one model served by a provider.
type Model struct {
	ID              string            `yaml:"id"`
	Name            string            `yaml:"name"`
	ContextWindow   *int64            `yaml:"context_window"`
	MaxOutputTokens *int64            `yaml:"max_output_tokens"`
	Input           []Modality        `yaml:"input"`
	Output          []Modality        `yaml:"output"`
	Reasoning       *bool             `yaml:"reasoning"`
	Variants        []ReasoningEffort `yaml:"variants"`
	ToolCalling     *bool             `yaml:"tool_calling"`
}

// Provider is one model provider route.
type Provider struct {
	ID       string                 `yaml:"id"`
	Name     string                 `yaml:"name"`
	Protocol Protocol               `yaml:"protocol"`
	BaseURL  string                 `yaml:"base_url"`
	APIKey   HeaderValue            `yaml:"api_key"`
	AuthType AuthType               `yaml:"auth_type"`
	Headers  map[string]HeaderValue `yaml:"headers"`
	Models   []Model                `yaml:"models"`
}

// EffectiveAuthType returns the provider auth type, defaulting to official.
func (p Provider) EffectiveAuthType() AuthType {
	if p.AuthType == "" {
		return AuthTypeOfficial
	}
	return p.AuthType
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

// UnmarshalYAML parses scalar values without consulting the process environment.
func (v *HeaderValue) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode || n.Tag != "!!str" {
		return fmt.Errorf("value must be a string scalar")
	}
	*v = HeaderValue{}
	switch {
	case strings.HasPrefix(n.Value, "Bearer ENV:"):
		v.BearerFromEnv = strings.TrimPrefix(n.Value, "Bearer ENV:")
		if !validEnvName(v.BearerFromEnv) {
			return fmt.Errorf("bearer ENV: requires an environment variable name matching [A-Za-z_][A-Za-z0-9_]*")
		}
	case strings.HasPrefix(n.Value, "ENV:"):
		v.FromEnv = strings.TrimPrefix(n.Value, "ENV:")
		if !validEnvName(v.FromEnv) {
			return fmt.Errorf("ENV: requires an environment variable name matching [A-Za-z_][A-Za-z0-9_]*")
		}
	default:
		v.Value = n.Value
	}
	return nil
}

func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, c := range name {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || (i > 0 && c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	return true
}
