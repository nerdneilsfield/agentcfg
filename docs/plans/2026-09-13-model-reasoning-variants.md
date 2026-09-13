# Model reasoning-variant implementation plan

Plan readiness: ready. The requested feature is a public IR extension. Native contracts were checked on 2026-09-13 before implementation.

## Outcome and scope

Add model-scoped reasoning-effort variants to `agentcfg.yaml`. A variant is a selectable reasoning effort for its enclosing model. It is neither a separate model nor an output verbosity, summary, token budget, or arbitrary provider-option escape hatch.

The source shape is:

```yaml
models:
  - id: glm-5.3
    reasoning: true
    variants: [low, high, max]
```

Effort names are provider/model-supplied lowercase identifiers (for example `low`, `medium`, `high`, `xhigh`, `max`, or `ultra`). Lists are non-empty, contain no duplicates, and require `reasoning: true`. `variants` does not select a default. Existing model configurations without variants retain their current output.

Existing changes: none in the agentcfg worktree. Authorization: implementation is authorized by the user's request to continue. Do not alter KVMux or any global configuration.

## Design and constraints

`ir.Model.Variants` is a model-local ordered list of provider/model-supplied reasoning efforts. Validation owns its syntax, duplication, and `reasoning: true` invariant. Emitters render the documented native model-level availability structure when possible. A fixed-level target skips unknown names; a target without a faithful list skips `variants` and still emits its remaining configuration. No target may reduce a list to the existing boolean `reasoning` flag or a selected default.

Exact native mapping is:

- OpenCode: `models.<id>.variants.<effort>.reasoningEffort`.
- Pi, Prime Agent, OpenClaw: `thinkingLevelMap` for documented native levels; unsupported names are skipped, never remapped.
- Kimi: `support_efforts` only; leave its unrelated `default_effort` unset.
- Crush: `reasoning_levels` only; leave `default_reasoning_effort` unset.
- Grok Build: `supports_reasoning_effort = true` plus `reasoning_efforts` records; no generated labels/descriptions/default selection beyond the canonical identifier.
- Zcode: structured `reasoning` object with `enabled: true` and `levels`; no target-specific default or per-level request options.
- Gajae: `thinking` in `effort` mode with `levels`; do not add its budget/adaptive mode fields.
- DeepSeek Harness: `reasoningEfforts` same-name map for documented native levels. It does not include the route's active/default reasoning setting; unsupported names are skipped.

Codex needs a complete replacement catalog that agentcfg cannot safely construct from the common IR. Cline's custom-model persistence drops its in-memory `reasoningOptions`. Jcode has one selected effort, Goose has no model-level list, and Hermes has a per-model selected override. Those targets skip a non-empty `variants` list. MiMo Code directly supports model variants and maps each source name to `reasoningEffort`.

## Acceptance map

| ID | Behavior or invariant | Task | Check and expected result |
| --- | --- | --- | --- |
| A1 | YAML accepts only a non-empty, duplicate-free model-local provider/model effort list paired with `reasoning: true`. | T1 | Focused IR tests pass. |
| A2 | Every target either renders the documented native availability list or rejects it; no emitter silently drops it. | T2, T3 | Target package tests include an emitted and/or rejection case. |
| A3 | OpenCode's model output has separate `low`, `high`, and `max` variant objects with only `reasoningEffort` semantics. | T2 | Golden/focused emitter test checks exact JSON fields. |
| A4 | Published protocol and target contracts describe the new behavior and limitations. | T1–T3 | `make check` passes and docs match generated field names. |

## Execution

### T1: Add the public IR contract

Status: done
Depends on: none
Acceptance: A1, A4
Targets: `internal/ir/types.go`, `internal/ir/load.go`, `internal/ir/load_test.go`, `docs/protocol.md`, `example.yaml`. The executable example selects only targets that faithfully represent its provider/model-supplied effort list.
Contracts: `Model.Variants` is the ordered provider/model effort list. It does not select a default or carry native wire values.

- [x] Add the IR field, validation, tests, and input documentation.
- [x] Add a runnable `example.yaml` variant list and select targets that can faithfully represent it; document the model-local YAML shape in the protocol.
- [x] Verified: `go test ./internal/ir ./internal/example` and `go run ./cmd/agentcfg validate -c example.yaml` pass; the example selects targets that preserve its listed effort names.
- [x] Committed as `12e7caa feat: add model reasoning variants to IR` and `c88a4dd fix: preserve provider reasoning effort names`.

Evidence: 2026-09-13 focused IR and embedded-example tests passed; `example.yaml: OK (2 providers, 2 MCP servers, 5 targets)`. Follow-up: names are provider/model-supplied rather than fixed canonical values; `low, medium, high, xhigh, max, ultra` was emitted unchanged for OpenCode.

### T2: Emit exact model-level variant availability

Status: done
Depends on: T1
Acceptance: A2, A3, A4
Targets: `internal/target/{opencode,pi,primeagent,kimi,crush,grok,zcode,gajae,deepseekharness,openclaw}`, their tests and contracts.
Contracts: Render only the canonical availability list. Do not write defaults, verbosity, summaries, budgets, arbitrary provider options, or changed model identities.

- [x] Implement documented output mappings and focused emission tests.
- [x] Update each changed target contract with supported shape and exclusions.
- [x] Verified: affected package tests and a generated OpenCode fixture show distinct `low`, `high`, `max` variants.
- [x] Committed target batches: `58bfd27`, `1030f0f`, `82da029`, and `2c28a73`; DeepSeek Harness follows in the final T2 commit.

Evidence: all listed mappings passed focused package tests on 2026-09-13. OpenCode, Crush, Grok, Zcode, and Gajae preserve `ultra`; targets with a fixed native level map reject it rather than remapping it.

### T3: Reject targets without model-level availability lists

Status: done
Depends on: T1
Acceptance: A2, A4
Targets: `internal/target/{codex,cline,jcode,goose,hermes,mimocode}`, their tests and contracts.
Contracts: a non-empty `variants` list is skipped when the target has no faithful model-level availability list; empty/absent lists preserve existing behavior.

- [x] Skip unsupported variants and add focused generation tests.
- [x] Document each native limitation in its target contract.
- [x] Verified: package tests for affected targets pass.
- [x] Correct prior blocking behavior in a follow-up commit.

Evidence: Current-source revalidation showed MiMo Code has a native model variants map. Codex's replacement catalog and Cline's nonpersistent `reasoningOptions` cannot safely represent this IR. Jcode, Goose, and Hermes use only selected/default controls or no list. These targets skip a populated `variants` list.

## Final acceptance

From `/Users/dengqi/Source/langs/go/agentcfg`, run `make check`. Then run `go run ./cmd/agentcfg gen -c <variant fixture> -t opencode` and confirm a single model contains independent `low`, `high`, and `max` variant entries, each setting only `reasoningEffort`.

## Progress and handoff

Implementation is complete. Target research came from child reports `variant-contracts-a` and `variant-contracts-b` on 2026-09-13, plus the local OpenCode, Prime Agent, Pi, Grok, and DeepSeek Harness source evidence cited above. Final action: run the full repository check.
