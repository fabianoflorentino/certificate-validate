# Feature Plan — Certificate Validate

## Philosophy

- **Single binary** — no external runtime, separate database, or extra containers
- **Zero or 1 new dependency per feature** — each feature may add **at most 1** external dependency
- **Optional frontend** — every dashboard feature must have CLI/API support first
- **Deploy unchanged** — same Dockerfile, same `docker-compose.yml`

---

## Phase 0 — Immediate Dashboard (frontend only, 0 backend)

**Effort**: very low. Only JS/CSS in `internal/api/static/`. No Go lines change.

| # | Feature | Files | What changes |
|---|---------|-------|--------------|
| 0.1 | **Search field** | `app.js`, `style.css` | Input that filters cards by hostname/CN/issuer in real time |
| 0.2 | **Sorting** | `app.js` | Dropdown "Sort by: days left ↑ / hostname / issuer" |
| 0.3 | **Summary badges** | `app.js`, `index.html` | "3 critical · 2 attention · 5 ok" in the header, next to the clock |
| 0.4 | **Card tooltip** | `style.css` | Show full issuer on hover (already has `title`, just ensure it) |

### Dependencies
None.

### Validation
- `go build ./cmd/certificate-validate` clean
- Open `http://localhost:5000/` and test search, sort, badges

---

## Phase 1 — Export + Data (1 new endpoint)

**Effort**: low. 1 endpoint + 1 button.

| # | Feature | Files | What changes |
|---|---------|-------|--------------|
| 1.1 | **Export JSON** | `api.go`, `app.js` | Button "Export JSON" → `GET /api/v1/cert/export/json` → download |
| 1.2 | **Export CSV** | `api.go`, `app.js` | Button "Export CSV" → `GET /api/v1/cert/export/csv` → download |

### New endpoints
```
GET /api/v1/cert/export/json  → application/json (Content-Disposition: attachment)
GET /api/v1/cert/export/csv   → text/csv (Content-Disposition: attachment)
```

### Dependencies
None (CSV uses stdlib `encoding/csv`).

### Validation
- `curl` on the endpoints → correct header + body
- Dashboard button → file download

---

## Phase 2 — Observability (1 new dep)

**Effort**: medium.

### 2.1 — Prometheus Metrics

Add dependency: `github.com/prometheus/client_golang`.

| What | Detail |
|------|--------|
| Endpoint | `GET /metrics` (separate port or same, via `--metrics-addr`) |
| Metrics | `certificate_days_left{host="...",port="..."}` gauge |
|          | `certificate_expired{host="...",port="..."}` 0/1 gauge |
| Update | On every request or with a fixed value (check at startup) |
| Config | `prometheus_metrics: true/false` in `settings.yml` |

### Dependencies
1 external: `prometheus/client_golang`.

### 2.2 — Alert Webhook

**Zero** new dependencies (uses `net/http`).

| What | Detail |
|------|--------|
| Trigger | `daysLeft < threshold` (configurable per host) |
| Payload | JSON with hostname, daysLeft, issuer, commonName |
| Destination | Configurable URL in `settings.yml` |
| Format | Slack Webhook, Discord, or generic JSON |
| When it runs | In `serve` + watch loop |

### New config (`settings.yml`)
```yaml
webhook:
  url: "https://hooks.slack.com/..."
  threshold: 30
  interval: 3600  # re-alert every N seconds
```

### Dependencies
None.

---

## Phase 3 — Deep Analysis (extends fetcher)

**Effort**: medium-high. Changes the core `fetcher` + `certificate.Certificate` struct.

### 3.1 — Certificate Chain

| What | Detail |
|------|--------|
| Where | `fetcher.Fetch()` already has access to the chain (`VerifiedChains`) |
| What to expose | Array of certificates: subject, issuer, notAfter, fingerprint |
| New struct | `certificate.ChainEntry` with summarized data from each level |
| Frontend | "Chain" tab in the modal, showing Root → Intermediate → Leaf |

### Changes
- `certificate.Certificate` gains a `Chain []ChainEntry` field
- `fetcher` extracts the chain from `tls.ConnectionState.PeerCertificates`
- Dashboard modal shows a breadcrumb of the chain

### 3.2 — TLS Version + Cipher Suites

| What | Detail |
|------|--------|
| Where | `tls.Config` already negotiates version/cipher |
| What to expose | TLS version (1.2, 1.3), cipher suite name |
| Struct | `certificate.Certificate` gains `TLSVersion`, `CipherSuite` |
| Frontend | Show in the modal, "Connection Security" section |

### Dependencies
None (stdlib `crypto/tls` exposes everything).

---

## Phase 4 — Persistence + History

**Effort**: medium. Depends on a format decision.

### 4.1 — Local History (JSONL)

No dependencies. File `data/history.jsonl` — one line per check per host.

| What | Detail |
|------|--------|
| Format | `{"host":"github.com","daysLeft":45,"ts":"2026-06-18T12:00:00Z"}` |
| Rotation | `max_entries: 10000` or `max_days: 90` in config |
| Update | On every `serve` check, append to the file |

### 4.2 — Dashboard Chart

| What | Detail |
|------|--------|
| Technique | Canvas API or SVG — no Chart.js (zero deps) |
| What to show | Line of `daysLeft` per host over the last N checks |
| Where | Modal or separate page `/history` |
| Interaction | Hover shows date + value |

### New API
```
GET /api/v1/cert/history/{hostname} → [{ts, daysLeft}, ...]
```

### Dependencies
None.

---

## Phase 5 — Polish

**Effort**: low-medium.

| # | Feature | What changes |
|---|---------|--------------|
| 5.1 | **HTTPS on the server** | Flags `--tls-cert` + `--tls-key`. Serves API+frontend with TLS |
| 5.2 | **Multiple ports** | Config `port: [443, 8443]` → checker does N checks per host |
| 5.3 | **Health check** | `GET /health` → 200 with `{"status":"ok"}` |

---

## Recommended Execution Order

```
Phase 0 ────→ Phase 1 ───→ Phase 2 ───→ Phase 3 ───→ Phase 4 ───→ Phase 5
(frontend only) (export)  (observability) (analysis) (history) (polish)
                                │
                                ├→ 2.1 Prometheus
                                └→ 2.2 Webhook (parallel)
```

**Real dependencies between phases**: none. Each phase can be done on its own. The order is by **cost-benefit**: what ships fastest and adds the most value first.

---

## Estimated Effort per Phase

| Phase | Files changed | Estimated lines | New deps |
|-------|---------------|-----------------|----------|
| 0 | 2-3 (JS/CSS) | ~60 | 0 |
| 1 | 2 (Go + JS) | ~50 | 0 |
| 2.1 | 2 (Go + config) | ~80 | 1 (prometheus) |
| 2.2 | 2 (Go + config) | ~100 | 0 |
| 3.1 | 3 (Go: cert, fetcher, api) | ~80 | 0 |
| 3.2 | 2 (Go: cert, fetcher) | ~40 | 0 |
| 4 | 3 (Go: history, api + JS) | ~150 | 0 |
| 5 | 2-3 (Go + config) | ~60 | 0 |
