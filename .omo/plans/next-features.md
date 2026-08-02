# Next Features — Certificate Validate

> Implementation order: decreasing priority.

---

## 🥇 Phase 1 — Quality Infrastructure

### 1.1 CI (GitHub Actions)
Automatic workflow on every PR:
```yaml
- go test -race -coverprofile=coverage.out ./...
- go tool cover -func=coverage.out | grep total | awk '...'
- golangci-lint run
- go build ./...
```
**File**: `.github/workflows/ci.yml`

### 1.2 Makefile
Standardized commands for dev:
```makefile
test    → go test -race -count=1 ./...
cover   → go tool cover -html=coverage.out
lint    → golangci-lint run
build   → go build -o certificate-validate ./cmd/certificate-validate
```
**File**: `Makefile`

### 1.3 golangci-lint + Pre-commit
Configure linters (errcheck, gosimple, govet, gofmt, misspell) and pre-commit hook.
**Files**: `.golangci.yml`, `.pre-commit-config.yaml`

---

## 🥇 Phase 2 — Resilience

### 2.1 Graceful Shutdown
`serve.go`: capture `ctx.Done()`, call `server.Shutdown()` with a 15s timeout.

### 2.2 Structured Logging (`log/slog`)
Replace `log.Printf` with `slog.Info`/`slog.Error` with attributes.
**Packages**: `api/`, `checker/`, `history/`, `metrics/`, `notifier/`, `cmd/`

---

## 🥇 Phase 3 — Integration Tests

### 3.1 HTTP Integration Tests
Test handlers against a real `httptest.NewServer`, validating JSON responses, status codes, CORS.
**File**: `internal/api/integration_test.go`

---

## 🥉 Phase 4 — Documentation

### 4.1 OpenAPI/Swagger Spec
Generate an OpenAPI 3.0 spec for the API endpoints.

---

## 🥉 Phase 5 — Deploy

### 5.1 Helm Chart
`chart/` with Deployment, Service, ConfigMap for Kubernetes.

---

## 🥉 Phase 6 — CLI Polish

### 6.1 Shell Completion
```go
rootCmd.CompletionOptions.DisableDefaultCmd = false
```

---

## 🥉 Phase 7 — Security

### 7.1 OCSP/CRL Stapling
Verify revocation via OCSP — functionality that existed in the original Python version.
