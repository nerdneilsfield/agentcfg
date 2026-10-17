// Package pi emits a Pi coding-agent custom models document.
package pi

import (
	"fmt"
	"strings"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
	"agentcfg/internal/target/pifamily"
)

func init() { target.Register(Target{}) }

// Target is the Pi emitter.
type Target struct{}

func (Target) ID() string { return "pi" }

// piThinkingLevels are the thinking levels Pi documents, in output order.
var piThinkingLevels = []ir.ReasoningEffort{"off", "minimal", "low", "medium", "high", "xhigh", "max"}

func piOptions() pifamily.Options {
	return pifamily.Options{ID: Target{}.ID(), Syntax: pifamily.DollarEnv, Levels: piThinkingLevels}
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	diags := pifamily.ValidateLiteralAPIKeys(t.ID(), cfg)
	diags = append(diags, pifamily.ValidateModelInput(t.ID(), cfg)...)
	diags = append(diags, piMCPDiagnostics(cfg)...)
	for i, p := range cfg.Providers {
		for name, v := range p.Headers {
			if v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetWarnf(t.ID(), fmt.Sprintf("providers[%d].headers.%s", i, name),
					"pi header values use the $NAME expression syntax; Bearer ENV:NAME has no verified bearer expansion; use auth_type: bearer instead"))
			}
		}
	}
	return diags
}

// MappedAuthTypes reports the auth_type values pi maps.
func (t Target) MappedAuthTypes() []ir.AuthType { return pifamily.MappedAuthTypes() }

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	content, err := pifamily.EncodeJSON(map[string]any{
		"providers": pifamily.Providers(cfg, piOptions()),
	})
	if err != nil {
		return nil, err
	}
	arts := []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "models.json",
		Format:        "json",
		SuggestedPath: "~/.pi/agent/models.json",
		Content:       content,
	}}
	if len(cfg.MCP) > 0 {
		mcpContent, err := pifamily.EncodeJSON(map[string]any{"mcpServers": piMCPServers(cfg)})
		if err != nil {
			return nil, err
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          "mcp.json",
			Format:        "json",
			SuggestedPath: "~/.pi/agent/mcp.json",
			Content:       mcpContent,
		})
	}
	return arts, nil
}

func piMCPServers(cfg ir.Config) map[string]any {
	servers := make(map[string]any)
	seen := make(map[string]bool)
	for _, server := range cfg.MCP {
		identity := strings.ReplaceAll(server.ID, "-", "_")
		if seen[identity] {
			continue
		}
		seen[identity] = true
		entry := map[string]any{}
		switch server.Transport {
		case ir.TransportStdio:
			if len(server.Command) > 0 {
				entry["command"] = server.Command[0]
				if len(server.Command) > 1 {
					entry["args"] = server.Command[1:]
				}
			}
			if server.CWD != "" {
				entry["cwd"] = server.CWD
			}
			if env := piMCPValues(server.Env); len(env) > 0 {
				entry["env"] = env
			}
		case ir.TransportHTTP:
			entry["url"] = server.URL
			if headers := piMCPValues(server.Headers); len(headers) > 0 {
				entry["headers"] = headers
			}
		}
		if server.Enabled != nil {
			entry["enabled"] = *server.Enabled
		}
		if server.TimeoutMS != nil && *server.TimeoutMS > 0 {
			entry["timeout"] = float64(*server.TimeoutMS) / 1000
		}
		servers[server.ID] = entry
	}
	return servers
}

func piMCPValues(values map[string]ir.HeaderValue) map[string]string {
	out := make(map[string]string)
	for name, value := range values {
		out[name] = piMCPValue(value)
	}
	return out
}

func piMCPValue(value ir.HeaderValue) string {
	switch {
	case value.FromEnv != "":
		return "${" + value.FromEnv + "}"
	case value.BearerFromEnv != "":
		return "Bearer ${" + value.BearerFromEnv + "}"
	default:
		// Pi's MCP resolver interprets leading ! as a command and expands $NAME
		// / ${NAME}; $$ escapes a dollar and $! escapes a literal exclamation.
		literal := strings.ReplaceAll(value.Value, "$", "$$")
		if strings.HasPrefix(literal, "!") {
			literal = "$!" + literal[1:]
		}
		return literal
	}
}

func piMCPDiagnostics(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	seen := make(map[string]int)
	for i, server := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		identity := strings.ReplaceAll(server.ID, "-", "_")
		if first, ok := seen[identity]; ok {
			diags = append(diags, diag.TargetWarnf(Target{}.ID(), path+".id", "server name collides with mcp[%d].id after Pi normalization; this entry is skipped", first))
			continue
		}
		seen[identity] = i
	}
	return diags
}
