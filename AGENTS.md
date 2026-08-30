# Project Conventions

These rules apply to all work in this repository. Extended guidance lives in the
skills under `.agents/skills/`.

## Project Overview

See [`README.md`](README.md) for features, architecture, configuration, and usage.
This file covers working conventions for AI agents.

## 1. English Only

Keep the entire project in English: identifiers, comments, commit messages,
documentation, and site content. Do not use Portuguese or any other language in
repository files.

## 2. SOLID, Applied Lightly

Use SOLID as a light design compass to keep development consistent and
maintainable: single responsibilities, open/closed extension, focused
interfaces, and dependency inversion — without over-engineering. See
`.agents/skills/golang-patterns/SKILL.md` for concrete examples.

## 3. Test Coverage ≥ 80% (Always)

Keep total test coverage consistent and always above 80%. The `pre-push`
lefthook hook enforces the minimum; treat it as a floor, not a target. Never add
production code without tests. See `.agents/skills/golang-testing/SKILL.md`.

## 4. Code Style Guidelines

- Match the surrounding file's conventions: naming, comment density, package
  structure.
- Comments explain **why**, never restate **what** the code does.
- Prefer small, scoped, behavior-preserving changes. No adjacent feature work,
  no opportunistic refactors, and no speculative abstractions without a caller
  or requirement.
- No dead code or half-built public surface. Future work belongs in `docs/`
  notes, not as unreachable stubs.
- Keep Go code boring: predictable, clear, and gofmt-clean.

## 5. Cross-Cutting Invariants (Do Not Violate)

These shape the architecture; changes that break them need explicit discussion:

1. **English everywhere** — no Portuguese or other language in any file.
2. **Metrics do not collide.** The `k8s monitor` uses a **dedicated
   Prometheus registry** (`internal/k8smonitor/metrics.go`) with `namespace`,
   `name`, `kind` labels to avoid clashing with the core `internal/metrics`
   package; metric names stay `certificate_`-prefixed.
3. **Read-only by default.** The Kubernetes monitor's RBAC is read-only
   (`get/list/watch` on Secrets and Ingresses) in Phase 1. Write permissions
   arrive only in the approved roadmap phases.
4. **No package-level mutable state.** Dependencies are injected via
   constructors, never reached through globals (see the `golang-patterns`
   skill).
5. **Untrusted certificate input is never dereferenced without validation.**
   Guard against nil, empty, and malformed inputs before reading fields.
6. **Keep dependencies verified** (`go mod verify`) and re-run `govulncheck`
   when adding or bumping dependencies.

## 6. Always Validate with Lefthook

Every change must pass the git hooks before commit/push:

- `pre-commit`: golangci-lint + `go vet`
- `pre-push`: `go mod verify`, golangci-lint, `go vet`, `go test -race`, coverage ≥ 80%

Run the full local gate before declaring any change ready:

```bash
lefthook run pre-commit
lefthook run pre-push
```

For a quick check before pushing you can also use the Makefile shortcuts:

```bash
make lint   # golangci-lint + go vet
make test   # go test -race (count=1, no cache)
```

Commit changes as small, thematic, English commits (see the `git-workflow`
skill). Never commit or push unless explicitly asked.

## 7. Toolchain Notes

- The Go toolchain and linters are managed with `asdf`. Extra tooling such as
  `golangci-lint` is installed into the asdf Go `GOBIN` with shims under
  `~/.asdf/shims`.
- `golangci-lint` must be built with a toolchain compatible with the project's
  Go version. A `golangci-lint` compiled with an older Go cannot analyze newer
  module code (it panics / reports false "undefined" errors). Verify with
  `golangci-lint version` if linting behaves unexpectedly.
- Keep dependencies verified (`go mod verify`) and run `govulncheck` when adding
  or bumping dependencies (see the `security` skill).

## 8. Documentation Map

Key reference material and where to find it:

- `README.md` — features, architecture, configuration, CLI/API usage, and development commands
- `docs/ARCHITECTURE.md` — overall design, layout, and package structure
- `docs/K8S_INTEGRATION.md` — the Kubernetes phase roadmap (Phases 1–3) and the
  `k8s monitor` design constraints (read-only RBAC, metrics contract)
- `docs/WHY.md` — project rationale and design decisions
- `docs/security-audit/` — security review material
- `.agents/skills/` — specialized workflow guidance (Go, git, containers,
  Kubernetes, security, site/design). Load a skill when a task matches its
  description
- `site/` — the Hugo website; keep it in sync with each phase's changes
