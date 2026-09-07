package ir

import (
	"bytes"
	"fmt"

	"go.yaml.in/yaml/v3"

	"agentcfg/internal/diag"
)

// Load decodes and validates one agentcfg.yaml document.
// It returns the normalized config and all diagnostics.
// A non-nil error is returned only for unrecoverable input problems
// (empty input or YAML syntax errors); semantic problems are reported
// as diagnostics.
func Load(src []byte) (Config, []diag.Diagnostic, error) {
	var cfg Config
	var diags []diag.Diagnostic

	trimmed := bytes.TrimSpace(src)
	if len(trimmed) == 0 {
		return cfg, nil, fmt.Errorf("agentcfg.yaml is empty")
	}

	dec := yaml.NewDecoder(bytes.NewReader(src))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return cfg, nil, fmt.Errorf("decoding agentcfg.yaml: %w", err)
	}

	if cfg.Version != 1 {
		diags = append(diags, diag.Errorf("version", "version must be 1, got %d", cfg.Version))
	}
	return cfg, append(diags, Validate(cfg)...), nil
}

// Validate runs semantic IR validation. It does not read environment
// values or contact endpoints.
func Validate(cfg Config) []diag.Diagnostic {
	var diags []diag.Diagnostic

	providerIDs := map[string]bool{}
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if !validID(p.ID) {
			diags = append(diags, diag.Errorf(path+".id", "invalid provider id %q: must match [a-z][a-z0-9_-]*", p.ID))
		} else if providerIDs[p.ID] {
			diags = append(diags, diag.Errorf(path+".id", "duplicate provider id %q", p.ID))
		}
		providerIDs[p.ID] = true

		switch p.Protocol {
		case ProtocolOpenAICompletions, ProtocolOpenAIResponses, ProtocolAnthropicMessages:
		case "":
			diags = append(diags, diag.Errorf(path+".protocol", "protocol is required"))
		default:
			diags = append(diags, diag.Errorf(path+".protocol", "unknown protocol %q", p.Protocol))
		}
		if p.BaseURL == "" {
			diags = append(diags, diag.Errorf(path+".base_url", "base_url is required"))
		}
		if len(p.Models) == 0 {
			diags = append(diags, diag.Errorf(path+".models", "at least one model is required"))
		}
		diags = append(diags, validateHeaderValues(path+".headers", p.Headers)...)

		modelIDs := map[string]bool{}
		for j, m := range p.Models {
			mpath := fmt.Sprintf("%s.models[%d]", path, j)
			if m.ID == "" {
				diags = append(diags, diag.Errorf(mpath+".id", "model id is required"))
			} else if modelIDs[m.ID] {
				diags = append(diags, diag.Errorf(mpath+".id", "duplicate model id %q in provider %q", m.ID, p.ID))
			}
			modelIDs[m.ID] = true
			diags = append(diags, validateModalities(mpath+".input", m.Input)...)
			diags = append(diags, validateModalities(mpath+".output", m.Output)...)
		}
	}

	mcpIDs := map[string]bool{}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if !validID(s.ID) {
			diags = append(diags, diag.Errorf(path+".id", "invalid MCP server id %q: must match [a-z][a-z0-9_-]*", s.ID))
		} else if mcpIDs[s.ID] {
			diags = append(diags, diag.Errorf(path+".id", "duplicate MCP server id %q", s.ID))
		}
		mcpIDs[s.ID] = true

		switch s.Transport {
		case TransportStdio:
			if len(s.Command) == 0 {
				diags = append(diags, diag.Errorf(path+".command", "command is required for stdio MCP server %q", s.ID))
			}
			if s.URL != "" {
				diags = append(diags, diag.Errorf(path+".url", "url is invalid for stdio MCP server %q", s.ID))
			}
			if len(s.Headers) != 0 {
				diags = append(diags, diag.Errorf(path+".headers", "headers are invalid for stdio MCP server %q", s.ID))
			}
			for name, v := range s.Env {
				if v.BearerFromEnv != "" {
					diags = append(diags, diag.Errorf(path+".env."+name, "bearer_from_env is only valid for HTTP Authorization headers"))
				}
			}
		case TransportHTTP:
			if s.URL == "" {
				diags = append(diags, diag.Errorf(path+".url", "url is required for http MCP server %q", s.ID))
			}
			if len(s.Command) != 0 {
				diags = append(diags, diag.Errorf(path+".command", "command is invalid for http MCP server %q", s.ID))
			}
			if s.CWD != "" {
				diags = append(diags, diag.Errorf(path+".cwd", "cwd is invalid for http MCP server %q", s.ID))
			}
			if len(s.Env) != 0 {
				diags = append(diags, diag.Errorf(path+".env", "env is invalid for http MCP server %q", s.ID))
			}
			diags = append(diags, validateHeaderValues(path+".headers", s.Headers)...)
		case "":
			diags = append(diags, diag.Errorf(path+".transport", "transport is required"))
		default:
			diags = append(diags, diag.Errorf(path+".transport", "unknown transport %q", s.Transport))
		}
	}

	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		providerID, modelID, ok := splitModelRef(cfg.Defaults.Model)
		if !ok {
			diags = append(diags, diag.Errorf("defaults.model", "must have the form provider-id/model-id, got %q", cfg.Defaults.Model))
		} else {
			found := false
			for _, p := range cfg.Providers {
				if p.ID != providerID {
					continue
				}
				for _, m := range p.Models {
					if m.ID == modelID {
						found = true
					}
				}
			}
			if !found {
				diags = append(diags, diag.Errorf("defaults.model", "references unknown model %q", cfg.Defaults.Model))
			}
		}
	}

	return diags
}

func validateHeaderValues(path string, headers map[string]HeaderValue) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for name, v := range headers {
		set := 0
		for _, s := range []string{v.Value, v.FromEnv, v.BearerFromEnv} {
			if s != "" {
				set++
			}
		}
		hpath := path + "." + name
		if set != 1 {
			diags = append(diags, diag.Errorf(hpath, "exactly one of value, from_env, bearer_from_env is required"))
			continue
		}
		if v.BearerFromEnv != "" && name != "Authorization" {
			diags = append(diags, diag.Errorf(hpath, "bearer_from_env is only valid for the Authorization header"))
		}
	}
	return diags
}

func validateModalities(path string, mods []Modality) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for _, m := range mods {
		switch m {
		case ModalityText, ModalityImage, ModalityAudio, ModalityVideo, ModalityPDF:
		default:
			diags = append(diags, diag.Errorf(path, "unknown modality %q", m))
		}
	}
	return diags
}

// SplitModelRef splits a provider-id/model-id reference.
func SplitModelRef(ref string) (string, string, bool) { return splitModelRef(ref) }

func splitModelRef(ref string) (string, string, bool) {
	for i := 0; i+1 < len(ref); i++ {
		if ref[i] == '/' {
			return ref[:i], ref[i+1:], true
		}
	}
	return "", "", false
}

func validID(id string) bool {
	if id == "" || !(id[0] >= 'a' && id[0] <= 'z') {
		return false
	}
	for i := 1; i < len(id); i++ {
		c := id[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
