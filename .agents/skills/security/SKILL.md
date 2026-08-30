---
name: security
description: Security review practices for this project, covering TLS/certificates, secrets handling, minimal RBAC, HTTP headers, and safe dependency management. Use when reviewing code, adding endpoints, handling secrets, or changing Kubernetes permissions.
origin: ECC
---

# Security

Practical security habits for a certificate- and TLS-focused Go service that
also runs inside Kubernetes. The aim is defense in depth without ceremony: apply
these checks to every change that touches data, secrets, HTTP surfaces, or
Kubernetes permissions.

## When to Activate

- Reviewing or writing code that handles certificates, private keys, or secrets
- Adding or modifying HTTP endpoints and webhooks
- Editing Kubernetes RBAC, manifests, or the monitor agent
- Adding dependencies or third-party integrations

## Core Checks

### 1. Secrets and Private Keys

- **Never log, print, or embed** private keys, passwords, tokens, or raw PEM
  material. If a key is needed only transiently, clear it after use.
- Prefer loading sensitive configuration from environment variables, secret
  mounts, or secure stores — never hard-code defaults or commit them.
- Treat any committed secret as compromised; rotate it and remove it from
  history.

### 2. TLS and Certificates

- Validate certificate chains fully; do not disable `InsecureSkipVerify` in
  production paths.
- When parsing certificates, treat input as untrusted: guard against nil, empty,
  and malformed inputs before dereferencing fields.
- Respect short timeouts and context deadlines when checking revocation
  (OCSP/CRP) so a slow upstream cannot stall the service.

### 3. HTTP Surface

- Set sane security headers on web responses (e.g. `X-Content-Type-Options`,
  `Content-Security-Policy`, `Strict-Transport-Security` where applicable).
- Bind health/metrics endpoints appropriately and avoid exposing debug/metrics
  ports to untrusted networks.
- Validate and bound request bodies, and use timeouts on outbound HTTP.

### 4. Kubernetes Least Privilege

- Keep the monitor's RBAC read-only unless a phase explicitly requires more
  (see `kubernetes-monitoring` skill and `docs/K8S_INTEGRATION.md`).
- Grant only the resources and verbs actually used; prefer namespaced Roles over
  ClusterRole when possible.
- Run workload containers as a **non-root user** with a read-only root
  filesystem where supported.

### 5. Dependencies

- Keep `go.mod` tidy and verified (`go mod verify`).
- Run `govulncheck` (or equivalent) on changes that add or bump dependencies.
- Prefer well-maintained, pinned versions; avoid pulling in large transitive
  trees for small tasks ("a little copying is better than a little dependency").

## Common Anti-Patterns

- Logging PEM/keys or passing them around as ambient globals
- `tls.Config{InsecureSkipVerify: true}` in non-test code
- Broad ClusterRole with `*` verbs/resources
- Adding a dependency where a few stdlib functions would do

**Remember**: least privilege, validate all untrusted input, protect secrets,
and keep dependencies verified — applied consistently to every change.
