# Repository guidance

## Target generation

When a target cannot represent one optional or target-specific capability, skip only the unsupported field, entry, or artifact and continue generating every supported output (for example, models plus supported MCP entries). Do not fail the whole target for a representability limitation. Emit an error only for invalid IR or when generation cannot safely produce the target output. Keep the skip behavior in the emitter and avoid diagnostics that contradict it.
