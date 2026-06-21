# Próximas Features — Certificate Validate

> Ordem de implementação: prioridade decrescente.

---

## 🥇 Fase 1 — Infraestrutura de Qualidade

### 1.1 CI (GitHub Actions)
Workflow automático em todo PR:
```yaml
- go test -race -coverprofile=coverage.out ./...
- go tool cover -func=coverage.out | grep total | awk '...'
- golangci-lint run
- go build ./...
```
**Arquivo**: `.github/workflows/ci.yml`

### 1.2 Makefile
Comandos padronizados para dev:
```makefile
test    → go test -race -count=1 ./...
cover   → go tool cover -html=coverage.out
lint    → golangci-lint run
build   → go build -o certificate-validate ./cmd/certificate-validate
```
**Arquivo**: `Makefile`

### 1.3 golangci-lint + Pre-commit
Configurar linters (errcheck, gosimple, govet, gofmt, misspell) e hook pre-commit.
**Arquivos**: `.golangci.yml`, `.pre-commit-config.yaml`

---

## 🥇 Fase 2 — Resiliência

### 2.1 Graceful Shutdown
`serve.go`: capturar `ctx.Done()`, chamar `server.Shutdown()` com timeout de 15s.

### 2.2 Structured Logging (`log/slog`)
Trocar `log.Printf` por `slog.Info`/`slog.Error` com atributos.
**Pacotes**: `api/`, `checker/`, `history/`, `metrics/`, `notifier/`, `cmd/`

---

## 🥇 Fase 3 — Testes de Integração

### 3.1 HTTP Integration Tests
Testar handler contra `httptest.NewServer` real, validando JSON responses, status codes, CORS.
**Arquivo**: `internal/api/integration_test.go`

---

## 🥉 Fase 4 — Documentação

### 4.1 OpenAPI/Swagger Spec
Gerar spec OpenAPI 3.0 dos endpoints da API.

---

## 🥉 Fase 5 — Deploy

### 5.1 Helm Chart
`chart/` com Deployment, Service, ConfigMap para Kubernetes.

---

## 🥉 Fase 6 — CLI Polish

### 6.1 Shell Completion
```go
rootCmd.CompletionOptions.DisableDefaultCmd = false
```

---

## 🥉 Fase 7 — Segurança

### 7.1 OCSP/CRL Stapling
Verificar revogação via OCSP — funcionalidade que existia no Python original.
