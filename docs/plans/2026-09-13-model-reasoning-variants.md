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

`ir.Model.Variants` is a model-local ordered list of provider/model-supplied reasoning efforts. Validation owns its syntax, duplication, and `reasoning: true` invariant. Emitters must treat a populated list as required configuration: render the documented native model-level availability structure or diagnose that the target cannot faithfully represent it. No target may reduce it to the existing boolean `reasoning` flag.

Exact native mapping is:

- OpenCode: `models.<id>.variants.<effort>.reasoningEffort`.
- Pi, Prime Agent, OpenClaw: `thinkingLevelMap` only when every configured variant name is a documented native level; unsupported names are target diagnostics, never remapped.
- Kimi: `support_efforts` only; leave its unrelated `default_effort` unset.
- Crush: `reasoning_levels` only; leave `default_reasoning_effort` unset.
- Grok Build: `supports_reasoning_effort = true` plus `reasoning_efforts` records; no generated labels/descriptions/default selection beyond the canonical identifier.
- Zcode: structured `reasoning` object with `enabled: true` and `levels`; no target-specific default or per-level request options.
- Gajae: `thinking` in `effort` mode with `levels`; do not add its budget/adaptive mode fields.
- DeepSeek Harness: `reasoningEfforts` same-name map when every configured variant name is a documented native level. It does not include the route's active/default reasoning setting.

Codex owns the active effort outside the provider model and needs a separate catalog artifact. Cline and Jcode only have one selected/provider default effort. Goose and Hermes have no model-level availability list. MiMo Code's generic pass-through variants/options do not establish a reasoning-effort selector. Those targets reject a non-empty `variants` list with a target diagnostic.

## Acceptance map

| ID | Behavior or invariant | Task | Check and expected result |
| --- | --- | --- | --- |
| A1 | YAML accepts only a non-empty, duplicate-free model-local canonical effort list paired with `reasoning: true`. | T1 | Focused IR tests pass. |
| A2 | Every target either renders the documented native availability list or rejects it; no emitter silently drops it. | T2, T3 | Target package tests include an emitted and/or rejection case. |
| A3 | OpenCode's model output has separate `low`, `high`, and `max` variant objects with only `reasoningEffort` semantics. | T2 | Golden/focused emitter test checks exact JSON fields. |
| A4 | Published protocol and target contracts describe the new behavior and limitations. | T1–T3 | `make check` passes and docs match generated field names. |

## Execution

### T1: Add the public IR contract

Status: done
Depends on: none
Acceptance: A1, A4
Targets: `internal/ir/types.go`, `internal/ir/load.go`, `internal/ir/load_test.go`, `docs/protocol.md`. The existing all-target `example.yaml` remains variant-free because its declared targets include targets that correctly reject model-level variant lists.
Contracts: `Model.Variants` is the ordered canonical effort list. It does not select a default or carry native wire values.

- [x] Add the IR field, validation, tests, and input documentation.
- [x] Keep the all-target example variant-free; document the model-local YAML shape in the protocol.
- [x] Verified: `go test ./internal/ir ./internal/example` and `go run ./cmd/agentcfg validate -c example.yaml` passed; the example remains valid for its all-target subset.
- [ ] Inspect the diff and commit `feat: add model reasoning variants to IR`.

Evidence: 2026-09-13 focused IR and embedded-example tests passed; `example.yaml: OK (2 providers, 2 MCP servers, 5 targets)`. Follow-up: names are provider/model-supplied rather than fixed canonical values; `low, medium, high, xhigh, max, ultra` was emitted unchanged for OpenCode.

### T2: Emit exact model-level variant availability

Status: in_progress
Depends on: T1
Acceptance: A2, A3, A4
Targets: `internal/target/{opencode,pi,primeagent,kimi,crush,grok,zcode,gajae,deepseekharness,openclaw}`, their tests and contracts.
Contracts: Render only the canonical availability list. Do not write defaults, verbosity, summaries, budgets, arbitrary provider options, or changed model identities.

- [ ] Implement documented output mappings and focused emission tests.
- [ ] Update each changed target contract with supported shape and exclusions.
- [ ] Verify: affected package tests and a generated OpenCode fixture show distinct `low`, `high`, `max` variants.
- [ ] Inspect the diff and commit `feat: emit model reasoning variants`.

Evidence: OpenCode, Pi, Prime Agent, OpenClaw, Crush, Grok, and Kimi mappings passed focused package tests on 2026-09-13. OpenCode, Crush, and Grok preserve `ultra`; fixed-map targets reject it rather than remapping it. Zcode and Gajae mappings passed focused package tests on 2026-09-13; DeepSeek Harness mapping remains pending.

### T3: Reject targets without model-level availability lists

Status: done
Depends on: T1
Acceptance: A2, A4
Targets: `internal/target/{codex,cline,jcode,goose,hermes,mimocode}`, their tests and contracts.
Contracts: a non-empty `variants` list must cause a target-scoped diagnostic; empty/absent lists preserve existing behavior.

- [x] Add target validation and focused rejection tests.
- [x] Document the exact native limitation in each target contract.
- [x] Verified: package tests for Codex, Cline, Jcode, Goose, Hermes, and MiMo Code passed.
- [ ] Inspect the diff and commit `fix: reject unsupported model reasoning variants`.

Evidence: 2026-09-13 focused package tests passed. Codex catalog, Cline provider-wide setting, Jcode single effort, and the targets with no verified list all reject a populated `variants` list.

## Final acceptance

From `/Users/dengqi/Source/langs/go/agentcfg`, run `make check`. Then run `go run ./cmd/agentcfg gen -c <variant fixture> -t opencode` and confirm a single model contains independent `low`, `high`, and `max` variant entries, each setting only `reasoningEffort`.

## Progress and handoff

T1 is verified and awaiting its scoped commit. Target research is complete: child reports `variant-contracts-a` and `variant-contracts-b` on 2026-09-13, plus the local OpenCode, Prime Agent, Pi, Grok, and target research evidence cited above. Next action: implement and verify T1.
