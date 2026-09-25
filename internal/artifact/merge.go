package artifact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"go.yaml.in/yaml/v3"
)

// ownedPaths names replaceable sections, including absent sections that must
// clear stale entries. Other generated fields are merged at their leaves.
func ownedPaths(a Artifact) []string {
	switch a.Target {
	case "qwen-code":
		return []string{"modelProviders", "providerProtocol", "mcpServers", "mcp.excluded", "model.name", "security.auth.selectedType", "security.auth.baseUrl"}
	case "openclaw":
		return []string{"models.providers", "mcp.servers", "agents.defaults.model"}
	case "opencode":
		return []string{"providers", "mcp.servers", "model"}
	case "fast-agent":
		if a.Name == "fastagent.config.yaml" {
			return []string{"mcp.servers", "default_model"}
		}
		if strings.HasPrefix(a.Name, "model-overlays/") {
			return []string{"connection", "defaults", "metadata", "picker", "model", "provider"}
		}
	case "goose":
		if a.Name == "config.yaml" {
			return []string{"providers", "active_provider"}
		}
		return []string{"models", "headers", "api_key_env", "api_key", "base_url", "engine"}
	case "deepseek-harness":
		return []string{"llm-pi-ai.providers"}
	case "grok":
		return []string{"model", "models.default", "mcp_servers"}
	case "hermes":
		return []string{"providers", "model.provider", "model.default", "mcp_servers"}
	case "jcode":
		return []string{"providers", "provider.default_provider", "provider.default_model", "mcpServers"}
	case "gajae", "omp":
		return []string{"providers", "mcpServers", "modelRoles.default"}
	case "kilo", "mimocode":
		return []string{"provider", "mcp", "model"}
	case "zcode":
		return []string{"provider", "model.main", "mcp.servers"}
	case "codex":
		return []string{"model_providers", "mcp_servers", "model", "model_provider"}
	case "kimi":
		return []string{"providers", "models", "default_model", "mcpServers"}
	case "continue":
		return []string{"models", "mcpServers"}
	case "crow", "crush":
		return []string{"providers", "models", "mcpServers", "mcp"}
	case "cline":
		return []string{"providers", "mcpServers", "lastUsedProvider"}
	case "commandcode":
		return []string{"provider", "model", "mcpServers"}
	case "prime-agent":
		return []string{"providers", "mcpServers", "defaultModel", "defaultProvider"}
	case "pi":
		return []string{"providers"}
	case "cometixcode":
		return []string{"mcpServers", "env.ANTHROPIC_BASE_URL", "env.ANTHROPIC_API_KEY", "env.ANTHROPIC_AUTH_TOKEN", "env.ANTHROPIC_MODEL"}
	case "aider":
		if a.Name == ".aider.conf.yml" {
			return []string{"model"}
		}
	}
	return nil
}

func decode(a Artifact, data []byte) (any, error) {
	switch a.Format {
	case "json":
		clean, err := jsonComments(data)
		if err != nil {
			return nil, err
		}
		var v any
		d := json.NewDecoder(bytes.NewReader(clean))
		d.UseNumber()
		if err := d.Decode(&v); err != nil {
			return nil, err
		}
		var extra any
		if err := d.Decode(&extra); err != io.EOF {
			return nil, fmt.Errorf("expected one JSON document")
		}
		return v, nil
	case "yaml":
		if a.Target == "deepseek-harness" {
			data = quoteExpressions(data)
		}
		var v any
		d := yaml.NewDecoder(bytes.NewReader(data))
		if err := d.Decode(&v); err != nil {
			return nil, err
		}
		var extra any
		if err := d.Decode(&extra); err != io.EOF {
			return nil, fmt.Errorf("expected one YAML document")
		}
		return v, nil
	case "toml":
		v := map[string]any{}
		_, err := toml.Decode(string(data), &v)
		return v, err
	default:
		return nil, fmt.Errorf("cannot merge format %q", a.Format)
	}
}

func merge(a Artifact, previous []byte) ([]byte, error) {
	if a.Target == "deepseek-harness" && a.Name == "cordis.patch.yml" {
		return mergeCordis(previous, a.Content)
	}
	generated, err := decode(a, a.Content)
	if err != nil {
		return nil, fmt.Errorf("generated document: %w", err)
	}
	old, err := decode(a, previous)
	if err != nil {
		return nil, err
	}
	if a.Target == "aider" && a.Name != ".aider.conf.yml" {
		// Both files consist entirely of model definitions.
		return a.Content, nil
	}
	dst, ok := old.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("existing configuration must be an object")
	}
	src, ok := generated.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("generated configuration must be an object")
	}
	for _, p := range ownedPaths(a) {
		erase(dst, strings.Split(p, "."))
	}
	if a.Target == "goose" && a.Name == "config.yaml" {
		if extensions, ok := dst["extensions"].(map[string]any); ok {
			for k, v := range extensions {
				if entry, ok := v.(map[string]any); ok {
					switch entry["type"] {
					case "stdio", "sse", "streamable_http":
						delete(extensions, k)
					}
				}
			}
		}
	}
	// Qwen's generated env keys have a reserved prefix; unrelated env survives.
	if a.Target == "qwen-code" {
		if env, ok := dst["env"].(map[string]any); ok {
			for k := range env {
				if strings.HasPrefix(k, "AGENTCFG_KEY_") {
					delete(env, k)
				}
			}
		}
	}
	mergeMap(dst, src)
	switch a.Format {
	case "json":
		b, e := json.MarshalIndent(dst, "", "  ")
		return append(b, '\n'), e
	case "yaml":
		return yaml.Marshal(dst)
	case "toml":
		var b bytes.Buffer
		err := toml.NewEncoder(&b).Encode(dst)
		return b.Bytes(), err
	}
	return nil, fmt.Errorf("unsupported format %q", a.Format)
}

// Cordis uses JavaScript-tagged scalars, including unquoted backticks that a
// standard YAML parser rejects. Quote their source text, retaining the tag.
func quoteExpressions(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if at := strings.Index(line, ": !!js "); at >= 0 {
			value := line[at+7:]
			if strings.HasPrefix(value, "`") || strings.HasPrefix(value, "process.env.") {
				lines[i] = line[:at+7] + strconv.Quote(value)
			}
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

func mergeCordis(old, generated []byte) ([]byte, error) {
	var dst, src yaml.Node
	for _, pair := range []struct {
		b []byte
		n *yaml.Node
	}{{old, &dst}, {generated, &src}} {
		if err := yaml.Unmarshal(quoteExpressions(pair.b), pair.n); err != nil {
			return nil, err
		}
		if len(pair.n.Content) != 1 || pair.n.Content[0].Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("cordis patch must be a sequence")
		}
	}
	for _, op := range dst.Content[0].Content {
		if op.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i < len(op.Content); i += 2 {
			if op.Content[i].Value != "insert" {
				continue
			}
			rows := op.Content[i+1]
			kept := []*yaml.Node{}
			for _, row := range rows.Content {
				managed := false
				if row.Kind == yaml.MappingNode {
					for j := 0; j < len(row.Content); j += 2 {
						if row.Content[j].Value == "name" && (row.Content[j+1].Value == "@deepseek-ai/dsh-mcp-client" || row.Content[j+1].Value == "@deepseek-ai/dsh-agent-default-model") {
							managed = true
						}
					}
				}
				if !managed {
					kept = append(kept, row)
				}
			}
			rows.Content = kept
		}
	}
	ops := []*yaml.Node{}
	for _, op := range dst.Content[0].Content {
		if op.Kind == yaml.MappingNode && len(op.Content) == 2 && op.Content[0].Value == "insert" && len(op.Content[1].Content) == 0 {
			continue
		}
		ops = append(ops, op)
	}
	dst.Content[0].Content = append(ops, src.Content[0].Content...)
	return yaml.Marshal(&dst)
}

func erase(m map[string]any, path []string) {
	if len(path) == 1 {
		delete(m, path[0])
		return
	}
	if child, ok := m[path[0]].(map[string]any); ok {
		erase(child, path[1:])
	}
}

func mergeMap(dst, src map[string]any) {
	for k, v := range src {
		if child, ok := v.(map[string]any); ok {
			if old, ok := dst[k].(map[string]any); ok {
				mergeMap(old, child)
				continue
			}
		}
		// Schema/name/version identify a document, rather than a managed section.
		if k == "$schema" || k == "schema" || k == "version" || k == "name" {
			if _, exists := dst[k]; exists {
				continue
			}
		}
		dst[k] = v
	}
}

// jsonComments accepts JSONC comments and trailing commas without interpreting
// string contents (URLs and native variable expressions remain literal).
func jsonComments(data []byte) ([]byte, error) {
	b := append([]byte(nil), data...)
	quoted, escape := false, false
	for i := 0; i < len(b); i++ {
		if quoted {
			if escape {
				escape = false
			} else if b[i] == '\\' {
				escape = true
			} else if b[i] == '"' {
				quoted = false
			}
			continue
		}
		if b[i] == '"' {
			quoted = true
			continue
		}
		if b[i] != '/' || i+1 >= len(b) {
			continue
		}
		switch b[i+1] {
		case '/':
			for i < len(b) && b[i] != '\n' {
				b[i] = ' '
				i++
			}
		case '*':
			b[i], b[i+1] = ' ', ' '
			i += 2
			for i+1 < len(b) && (b[i] != '*' || b[i+1] != '/') {
				if b[i] != '\n' {
					b[i] = ' '
				}
				i++
			}
			if i+1 >= len(b) {
				return nil, fmt.Errorf("unterminated JSON comment")
			}
			b[i], b[i+1] = ' ', ' '
			i++
		}
	}
	quoted, escape = false, false
	for i := 0; i < len(b); i++ {
		if quoted {
			if escape {
				escape = false
			} else if b[i] == '\\' {
				escape = true
			} else if b[i] == '"' {
				quoted = false
			}
			continue
		}
		if b[i] == '"' {
			quoted = true
			continue
		}
		if b[i] != ',' {
			continue
		}
		j := i + 1
		for j < len(b) && strings.ContainsRune(" \t\r\n", rune(b[j])) {
			j++
		}
		if j < len(b) && (b[j] == '}' || b[j] == ']') {
			b[i] = ' '
		}
	}
	return b, nil
}
