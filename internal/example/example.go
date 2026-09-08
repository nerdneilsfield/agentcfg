// Package example embeds the canonical example IR shipped at the
// repository root (example.yaml). The embedded copy feeds
// "agentcfg gen-example"; example_test.go keeps the two in sync.
package example

import _ "embed"

// IR is the canonical agentcfg example configuration.
//
//go:embed example.yaml
var IR string
