# Plano de Features — Certificate Validate

## Filosofia

- **Binário único** — nada de runtime externo, banco separado ou containers extras
- **Zero ou 1 dependência nova por feature** — cada feature pode adicionar **no máximo 1** dependency externa
- **Frontend opcional** — toda feature de dashboard tem que ter suporte CLI/API primeiro
- **Deploy inalterado** — mesmo Dockerfile, mesmo `docker-compose.yml`

---

## Fase 0 — Dashboard Imediato (só frontend, 0 backend)

**Esforço**: muito baixo. Só JS/CSS no `internal/api/static/`. Nenhuma linha em Go muda.

| # | Feature | Arquivos | O que muda |
|---|---------|----------|------------|
| 0.1 | **Campo de busca** | `app.js`, `style.css` | Input que filtra cards por hostname/CN/issuer em tempo real |
| 0.2 | **Ordenação** | `app.js` | Dropdown "Sort by: days left ↑ / hostname / issuer" |
| 0.3 | **Badges de resumo** | `app.js`, `index.html` | "3 críticos · 2 atenção · 5 ok" no header, ao lado do relógio |
| 0.4 | **Tooltip no card** | `style.css` | Mostrar issuer completo no hover (já tem `title`, só garantir) |

### Dependências
Nenhuma.

### Validação
- `go build ./cmd/certificate-validate` limpo
- Abrir `http://localhost:5000/` e testar busca, sort, badges

---

## Fase 1 — Export + Dados (1 endpoint novo)

**Esforço**: baixo. 1 endpoint + 1 botão.

| # | Feature | Arquivos | O que muda |
|---|---------|----------|------------|
| 1.1 | **Export JSON** | `api.go`, `app.js` | Botão "Export JSON" → `GET /api/v1/cert/export/json` → download |
| 1.2 | **Export CSV** | `api.go`, `app.js` | Botão "Export CSV" → `GET /api/v1/cert/export/csv` → download |

### Endpoints novos
```
GET /api/v1/cert/export/json  → application/json (Content-Disposition: attachment)
GET /api/v1/cert/export/csv   → text/csv (Content-Disposition: attachment)
```

### Dependências
Nenhuma (CSV usa `encoding/csv` da stdlib).

### Validação
- `curl` nos endpoints → header + body corretos
- Botão no dashboard → download do arquivo

---

## Fase 2 — Observabilidade (1 dep nova)

**Esforço**: médio.

### 2.1 — Métricas Prometheus

Adicionar dependência: `github.com/prometheus/client_golang`.

| O que | Detalhe |
|-------|---------|
| Endpoint | `GET /metrics` (porta separada ou mesma, via `--metrics-addr`) |
| Métricas | `certificate_days_left{host="...",port="..."}` gauge |
|           | `certificate_expired{host="...",port="..."}` 0/1 gauge |
| Atualização | A cada request ou com valor fixo (check no startup) |
| Config | `prometheus_metrics: true/false` no `settings.yml` |

### Dependências
1 externa: `prometheus/client_golang`.

### 2.2 — Webhook de Alerta

**Zero** dependências novas (usa `net/http`).

| O que | Detalhe |
|-------|---------|
| Gatilho | `daysLeft < threshold` (configurável por host) |
| Payload | JSON com hostname, daysLeft, issuer, commonName |
| Destino | URL configurável no `settings.yml` |
| Formato | Slack Webhook, Discord, ou JSON genérico |
| Quando executa | No `serve` + watch loop |

### Config nova (`settings.yml`)
```yaml
webhook:
  url: "https://hooks.slack.com/..."
  threshold: 30
  interval: 3600  # re-alerta a cada N segundos
```

### Dependências
Nenhuma.

---

## Fase 3 — Análise Profunda (estende fetcher)

**Esforço**: médio-alto. Muda o core `fetcher` + `certificate.Certificate` struct.

### 3.1 — Cadeia do Certificado

| O que | Detalhe |
|-------|---------|
| Onde | `fetcher.Fetch()` já tem acesso à chain (`VerifiedChains`) |
| O que expor | Array de certificados: subject, issuer, notAfter, fingerprint |
| Struct nova | `certificate.ChainEntry` com dados resumidos de cada nível |
| Frontend | Aba "Chain" no modal, mostrando Root → Intermediate → Leaf |

### Mudanças
- `certificate.Certificate` ganha campo `Chain []ChainEntry`
- `fetcher` extrai chain do `tls.ConnectionState.PeerCertificates`
- Modal no dashboard mostra breadcrumb da cadeia

### 3.2 — TLS Version + Cipher Suites

| O que | Detalhe |
|-------|---------|
| Onde | `tls.Config` já negocia version/cipher |
| O que expor | TLS version (1.2, 1.3), cipher suite name |
| Struct | `certificate.Certificate` ganha `TLSVersion`, `CipherSuite` |
| Frontend | Mostrar no modal, seção "Connection Security" |

### Dependências
Nenhuma (stdlib `crypto/tls` expõe tudo).

---

## Fase 4 — Persistência + Histórico

**Esforço**: médio. Depende de decisão de formato.

### 4.1 — Histórico Local (JSONL)

Sem dependências. Arquivo `data/history.jsonl` — uma linha por check por host.

| O que | Detalhe |
|-------|---------|
| Formato | `{"host":"github.com","daysLeft":45,"ts":"2026-06-18T12:00:00Z"}` |
| Rotação | `max_entries: 10000` ou `max_days: 90` no config |
| Atualização | A cada check no `serve`, append ao arquivo |

### 4.2 — Gráfico no Dashboard

| O que | Detalhe |
|-------|---------|
| Técnica | Canvas API ou SVG — sem Chart.js (zero deps) |
| O que mostrar | Linha de `daysLeft` por host nos últimos N checks |
| Onde | Modal ou página separada `/history` |
| Interação | Hover mostra data + valor |

### API nova
```
GET /api/v1/cert/history/{hostname} → [{ts, daysLeft}, ...]
```

### Dependências
Nenhuma.

---

## Fase 5 — Polimento

**Esforço**: baixo-médio.

| # | Feature | O que muda |
|---|---------|------------|
| 5.1 | **HTTPS no servidor** | Flags `--tls-cert` + `--tls-key`. Serve API+frontend com TLS |
| 5.2 | **Portas múltiplas** | Config `port: [443, 8443]` → checker faz N checks por host |
| 5.3 | **Health check** | `GET /health` → 200 com `{"status":"ok"}` |

---

## Ordem de Execução Recomendada

```
Fase 0 ────→ Fase 1 ───→ Fase 2 ───→ Fase 3 ───→ Fase 4 ───→ Fase 5
(só frontend)  (export)   (observabilidade) (análise) (histórico) (polimento)
                               │
                               ├→ 2.1 Prometheus
                               └→ 2.2 Webhook (paralelo)
```

**Dependências reais entre fases**: nenhuma. Cada fase pode ser feita sozinha. A ordem é por **custo-benefício**: o que sai mais rápido e agrega mais valor primeiro.

---

## Esforço Estimado por Fase

| Fase | Arquivos alterados | Linhas estimadas | Deps novas |
|------|-------------------|-------------------|------------|
| 0 | 2-3 (JS/CSS) | ~60 | 0 |
| 1 | 2 (Go + JS) | ~50 | 0 |
| 2.1 | 2 (Go + config) | ~80 | 1 (prometheus) |
| 2.2 | 2 (Go + config) | ~100 | 0 |
| 3.1 | 3 (Go: cert, fetcher, api) | ~80 | 0 |
| 3.2 | 2 (Go: cert, fetcher) | ~40 | 0 |
| 4 | 3 (Go: history, api + JS) | ~150 | 0 |
| 5 | 2-3 (Go + config) | ~60 | 0 |
