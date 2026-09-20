# `omp` target

Track: Managed (the pi-family builder already existed, so the edits were clear
across known files). Authorization: implementation, requested by the user.

## Goal

Emit Oh My Pi's native fragments for the IR: `~/.omp/agent/models.yml`,
`~/.omp/agent/mcp.json`, and `~/.omp/agent/config.yml`.

## Change

- `internal/target/pifamily`: add `EncodeYAML`, an `Options.Bearer` renderer, and
  `Options.ModelFields` for fork-specific model fields. Bearer and ModelFields
  are optional; a nil hook is skipped so the forks that reject bearer headers in
  `Validate` never emit an empty value.
- `internal/target/omp`: reuse the shared builder with `BareEnv` syntax (the
  fork resolves the whole value as an environment variable name, unlike Pi's
  `"$NAME"` template), `Bearer` → `"Bearer ${NAME}"`, and `ModelFields` mapping
  IR `variants` to `thinking: {mode: effort, efforts: [...]}` using the fork's
  closed effort vocabulary.
- `internal/target/all`: register the target.
- Docs: `docs/agents/omp.md`, README rows, and the compiled-in id list in
  `docs/protocol.md`.

## Check

- `make check` — formatting, lint, vet, and every target's tests.
- `go test ./internal/target/omp/` covers: bare-name credentials, `authHeader`
  for `bearer`, effort filtering, MCP artifact only when declared, bearer and
  timeout rendering, role artifact only when a default is set, byte-stable
  output, and rejection of literal `$`/`!` credentials.
- Functional: `gen --config <probe> -t omp` twice, then diff — output is
  identical, and `thinking.efforts` drops an out-of-vocabulary name.
- The shared `internal/target/all` suite runs the new target through the
  credential and `auth_type` guards automatically.

## Not covered

Role assignments beyond `default`, retry fallback chains, provider discovery,
and Oh My Pi's `oauth` provider auth are configuration the IR does not model.
