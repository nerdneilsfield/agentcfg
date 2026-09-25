package artifact

// Include optional native companions even when an emitter has no entries, so
// removing the last MCP server also clears a previously populated MCP file.
func companions(arts []Artifact) []Artifact {
	targets := map[string]bool{}
	names := map[string]bool{}
	for _, a := range arts {
		targets[a.Target] = true
		names[a.Target+"/"+a.Name] = true
	}
	for target := range targets {
		var extras []Artifact
		add := func(name, format, path, content string) {
			extras = append(extras, Artifact{Target: target, Name: name, Format: format, SuggestedPath: path, Content: []byte(content)})
		}
		switch target {
		case "cline":
			add("cline_mcp_settings.json", "json", "~/.cline/data/settings/cline_mcp_settings.json", "{\"mcpServers\":{}}\n")
		case "cometixcode", "commandcode":
			add(".mcp.json", "json", ".mcp.json", "{\"mcpServers\":{}}\n")
		case "jcode":
			add("mcp.json", "json", "~/.jcode/mcp.json", "{\"mcpServers\":{}}\n")
		case "kimi":
			add("mcp.json", "json", "~/.kimi-code/mcp.json", "{\"mcpServers\":{}}\n")
		case "gajae":
			add("mcp.json", "json", "~/.gjc/agent/mcp.json", "{\"mcpServers\":{}}\n")
			add("config.yml", "yaml", "~/.gjc/agent/config.yml", "{}\n")
		case "omp":
			add("mcp.json", "json", "~/.omp/agent/mcp.json", "{\"mcpServers\":{}}\n")
			add("config.yml", "yaml", "~/.omp/agent/config.yml", "{}\n")
		case "goose":
			add("config.yaml", "yaml", "~/.config/goose/config.yaml", "{}\n")
		case "deepseek-harness":
			add("cordis.patch.yml", "yaml", "$DSH_HOME/cordis.patch.yml", "[]\n")
		}
		for _, a := range extras {
			if !names[a.Target+"/"+a.Name] {
				arts = append(arts, a)
			}
		}
	}
	return SortedByName(arts)
}
