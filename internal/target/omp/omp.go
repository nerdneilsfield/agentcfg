// Package omp emits Oh My Pi provider, MCP, and role fragments.
package omp

import (
	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
	"agentcfg/internal/target/pifamily"
)

func init() { target.Register(Target{}) }

// Target is the Oh My Pi emitter.
type Target struct{}

func (Target) ID() string { return "omp" }

// ompEfforts are the thinking efforts Oh My Pi's schema accepts, in output order.
var ompEfforts = []ir.ReasoningEffort{"minimal", "low", "medium", "high", "xhigh", "max"}

func ompOptions() pifamily.Options {
	return pifamily.Options{
		ID:          Target{}.ID(),
		Syntax:      pifamily.BareEnv,
		Bearer:      func(name string) string { return "Bearer ${" + name + "}" },
		ModelFields: ompModelFields,
	}
}

// ompModelFields maps IR variants to Oh My Pi's model-level thinking block. Its
// mode selects how an effort reaches the provider; IR variants only carry names,
// so the effort mode is used and the provider dialect stays Oh My Pi's concern.
func ompModelFields(m ir.Model) map[string]any {
	if len(m.Variants) == 0 {
		return nil
	}
	declared := make(map[ir.ReasoningEffort]bool, len(m.Variants))
	for _, effort := range m.Variants {
		declared[effort] = true
	}
	var efforts []string
	for _, effort := range ompEfforts {
		if declared[effort] {
			efforts = append(efforts, string(effort))
		}
	}
	if len(efforts) == 0 {
		return nil
	}
	return map[string]any{
		"thinking": map[string]any{"mode": "effort", "efforts": efforts},
	}
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	diags := pifamily.ValidateLiteralAPIKeys(t.ID(), cfg)
	return append(diags, pifamily.ValidateModelInput(t.ID(), cfg)...)
}

// MappedAuthTypes reports the auth_type values Oh My Pi maps.
func (t Target) MappedAuthTypes() []ir.AuthType { return pifamily.MappedAuthTypes() }

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	models, err := pifamily.EncodeYAML(map[string]any{
		"providers": pifamily.Providers(cfg, ompOptions()),
	})
	if err != nil {
		return nil, err
	}
	arts := []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "models.yml",
		Format:        "yaml",
		SuggestedPath: "~/.omp/agent/models.yml",
		Content:       models,
	}}

	if len(cfg.MCP) > 0 {
		mcp, err := pifamily.EncodeJSON(map[string]any{"mcpServers": mcpServers(cfg)})
		if err != nil {
			return nil, err
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          "mcp.json",
			Format:        "json",
			SuggestedPath: "~/.omp/agent/mcp.json",
			Content:       mcp,
		})
	}

	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		roles, err := pifamily.EncodeYAML(map[string]any{
			"modelRoles": map[string]any{"default": cfg.Defaults.Model},
		})
		if err != nil {
			return nil, err
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          "config.yml",
			Format:        "yaml",
			SuggestedPath: "~/.omp/agent/config.yml",
			Content:       roles,
		})
	}

	return arts, nil
}

// mcpServers renders the mcpServers map. Oh My Pi takes stdio and http/sse
// servers; the IR carries stdio and http.
func mcpServers(cfg ir.Config) map[string]any {
	servers := make(map[string]any, len(cfg.MCP))
	for _, s := range cfg.MCP {
		entry := map[string]any{"type": string(s.Transport)}
		switch s.Transport {
		case ir.TransportStdio:
			entry["command"] = s.Command[0]
			if len(s.Command) > 1 {
				entry["args"] = s.Command[1:]
			}
			if s.CWD != "" {
				entry["cwd"] = s.CWD
			}
			if len(s.Env) > 0 {
				env := make(map[string]string, len(s.Env))
				for name, v := range s.Env {
					if v.FromEnv != "" {
						env[name] = v.FromEnv
						continue
					}
					env[name] = v.Value
				}
				entry["env"] = env
			}
		case ir.TransportHTTP:
			entry["url"] = s.URL
			if len(s.Headers) > 0 {
				headers := make(map[string]string, len(s.Headers))
				for name, v := range s.Headers {
					switch {
					case v.BearerFromEnv != "":
						headers[name] = "Bearer ${" + v.BearerFromEnv + "}"
					case v.FromEnv != "":
						headers[name] = v.FromEnv
					default:
						headers[name] = v.Value
					}
				}
				entry["headers"] = headers
			}
		}
		if s.TimeoutMS != nil {
			entry["timeout"] = *s.TimeoutMS
		}
		if s.Enabled != nil && !*s.Enabled {
			entry["enabled"] = false
		}
		servers[s.ID] = entry
	}
	return servers
}
