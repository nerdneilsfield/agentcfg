// Package primeagent emits Prime Agent models.json and settings.json.
package primeagent

import (
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
					if v.BearerFromEnv != "" {
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
		settings["mcpServers"] = mcp
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
