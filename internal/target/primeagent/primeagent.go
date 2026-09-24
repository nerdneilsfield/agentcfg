// Package primeagent emits Prime Agent models.json and settings.json.
package primeagent

import (
	"fmt"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
	"agentcfg/internal/target/pifamily"
)

func init() { target.Register(Target{}) }

// Target is the Prime Agent emitter.
type Target struct{}

func (Target) ID() string { return "prime-agent" }

// primeThinkingLevels are the thinking levels Prime Agent documents, in output order.
var primeThinkingLevels = []ir.ReasoningEffort{"off", "minimal", "low", "medium", "high", "xhigh", "max"}

func primeOptions() pifamily.Options {
	return pifamily.Options{ID: Target{}.ID(), Syntax: pifamily.BareEnv, Levels: primeThinkingLevels}
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	diags := pifamily.ValidateLiteralAPIKeys(t.ID(), cfg)
	diags = append(diags, pifamily.ValidateModelInput(t.ID(), cfg)...)
	for i, p := range cfg.Providers {
		for name, v := range p.Headers {
			if v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetWarnf(t.ID(), fmt.Sprintf("providers[%d].headers.%s", i, name),
					"prime-agent provider headers are flat strings with no bearer-from-env convention; use ENV:NAME or a constant value"))
			}
		}
	}
	for i, s := range cfg.MCP {
		if s.Transport == ir.TransportStdio && len(s.Command) == 0 {
			diags = append(diags, diag.TargetWarnf(t.ID(), fmt.Sprintf("mcp[%d].command", i),
				"prime-agent stdio MCP server needs a command; the server is skipped"))
		}
		if s.Transport == ir.TransportHTTP {
			for name, v := range s.Headers {
				if v.FromEnv != "" {
					diags = append(diags, diag.TargetWarnf(t.ID(), fmt.Sprintf("mcp[%d].headers.%s", i, name),
						"prime-agent MCP http headers are static strings; environment interpolation is not verified"))
				}
			}
		}
	}
	return diags
}

// MappedAuthTypes reports the auth_type values prime-agent maps.
func (t Target) MappedAuthTypes() []ir.AuthType { return pifamily.MappedAuthTypes() }

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	modelsJSON, err := pifamily.EncodeJSON(map[string]any{
		"providers": pifamily.Providers(cfg, primeOptions()),
	})
	if err != nil {
		return nil, err
	}

	settings := map[string]any{}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		if providerID, modelID, ok := ir.SplitModelRef(cfg.Defaults.Model); ok {
			settings["defaultProvider"] = providerID
			settings["defaultModel"] = modelID
		}
	}
	if len(cfg.MCP) > 0 {
		mcp := map[string]any{}
		for _, s := range cfg.MCP {
			switch s.Transport {
			case ir.TransportStdio:
				if len(s.Command) == 0 {
					// No command to run; Validate warns and the server is skipped.
					continue
				}
				entry := map[string]any{
					"type":    "stdio",
					"command": s.Command[0],
					"args":    s.Command[1:],
				}
				if s.CWD != "" {
					entry["cwd"] = s.CWD
				}
				if len(s.Env) > 0 {
					env := map[string]any{}
					for name, v := range s.Env {
						if v.FromEnv != "" {
							env[name] = map[string]string{"env": v.FromEnv}
						}
					}
					entry["env"] = env
				}
				if s.Enabled != nil && !*s.Enabled {
					entry["enabled"] = false
				}
				mcp[s.ID] = entry
			case ir.TransportHTTP:
				entry := map[string]any{
					"type": "http",
					"url":  s.URL,
				}
				headers := map[string]string{}
				for name, v := range s.Headers {
					if v.BearerFromEnv != "" || v.FromEnv != "" {
						// MCP http headers carry constant values only; Validate
						// warns and the reference is skipped.
						continue
					}
					headers[name] = v.Value
				}
				if len(headers) > 0 {
					entry["headers"] = headers
				}
				if auth, ok := s.Headers["Authorization"]; ok && auth.BearerFromEnv != "" {
					entry["bearerTokenEnvVar"] = auth.BearerFromEnv
				}
				if s.Enabled != nil && !*s.Enabled {
					entry["enabled"] = false
				}
				mcp[s.ID] = entry
			}
		}
		if len(mcp) > 0 {
			settings["mcpServers"] = mcp
		}
	}

	settingsJSON, err := pifamily.EncodeJSON(settings)
	if err != nil {
		return nil, err
	}
	return []artifact.Artifact{
		{
			Target:        t.ID(),
			Name:          "models.json",
			Format:        "json",
			SuggestedPath: "~/.prime/agent/models.json",
			Content:       modelsJSON,
		},
		{
			Target:        t.ID(),
			Name:          "settings.json",
			Format:        "json",
			SuggestedPath: "~/.prime/agent/settings.json",
			Content:       settingsJSON,
		},
	}, nil
}
