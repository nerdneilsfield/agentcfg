// Package all registers every compiled-in target emitter.
package all

import (
	_ "agentcfg/internal/target/codex"
	_ "agentcfg/internal/target/deepseekharness"
	_ "agentcfg/internal/target/opencode"
	_ "agentcfg/internal/target/pi"
	_ "agentcfg/internal/target/primeagent"
)
