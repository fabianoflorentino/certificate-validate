#!/usr/bin/env bash
# Verifies the deterministic compose demo end to end: API, certificate data,
# Prometheus metrics, webhook delivery, Prometheus scraping, and the CLI.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -f "${ROOT}/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    . "${ROOT}/.env"
    set +a
fi

DEMO_HOST="${DEMO_HOST:-tls-origin}"
DEMO_PORT="${DEMO_PORT:-443}"
APP_URL="${APP_URL:-http://127.0.0.1:${APP_PORT:-5000}}"
PROM_URL="${PROM_URL:-http://127.0.0.1:${PROMETHEUS_PORT:-9090}}"
ARTIFACTS_DIR="${ARTIFACTS_DIR:-}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

COMPOSE=(docker compose -f "${ROOT}/docker-compose.yml")
if [[ -f "${ROOT}/.env" ]]; then
    COMPOSE+=(--env-file "${ROOT}/.env")
fi

if [[ -n "${ARTIFACTS_DIR}" ]]; then
    mkdir -p "${ARTIFACTS_DIR}"
    : > "${ARTIFACTS_DIR}/verification.txt"
fi

log() {
    printf '%s\n' "$*"
    if [[ -n "${ARTIFACTS_DIR}" ]]; then
        printf '%s\n' "$*" >> "${ARTIFACTS_DIR}/verification.txt"
    fi
}

fail() {
    log "ERROR: $*" >&2
    exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v jq >/dev/null 2>&1 || fail "jq is required"

log "==> Waiting for API health at ${APP_URL}/health"
ready=0
for _ in $(seq 1 60); do
    if curl -fsS "${APP_URL}/health" -o "${TMP_DIR}/health.json" 2>/dev/null &&
        jq -e '.status == "ok"' "${TMP_DIR}/health.json" >/dev/null 2>&1; then
        ready=1
        break
    fi
    sleep 1
done
[[ "${ready}" == "1" ]] || fail "API health did not become ok"

log "==> Checking certificate API for ${DEMO_HOST}"
ready=0
for _ in $(seq 1 30); do
    if curl -fsS "${APP_URL}/api/v1/cert/info/${DEMO_HOST}" -o "${TMP_DIR}/cert.json" 2>/dev/null &&
        jq -e --arg host "${DEMO_HOST}" '.hostname == $host and .daysLeft > 0' "${TMP_DIR}/cert.json" >/dev/null 2>&1; then
        ready=1
        break
    fi
    sleep 1
done
[[ "${ready}" == "1" ]] || fail "certificate API did not return a valid certificate"
days_left="$(jq -r '.daysLeft' "${TMP_DIR}/cert.json")"

log "==> Checking /metrics for certificate_days_left"
ready=0
for _ in $(seq 1 30); do
    if curl -fsS "${APP_URL}/metrics" -o "${TMP_DIR}/metrics.txt" 2>/dev/null &&
        grep -q "certificate_days_left{host=\"${DEMO_HOST}\",port=\"${DEMO_PORT}\"}" "${TMP_DIR}/metrics.txt"; then
        ready=1
        break
    fi
    sleep 1
done
[[ "${ready}" == "1" ]] || fail "certificate_days_left metric not found"

log "==> Waiting for webhook alert"
ready=0
for _ in $(seq 1 30); do
    if alerts="$("${COMPOSE[@]}" exec -T webhook-receiver cat /data/alerts.jsonl 2>/dev/null)" &&
        [[ -n "${alerts}" ]]; then
        printf '%s\n' "${alerts}" > "${TMP_DIR}/webhook-alerts.jsonl"
        if jq -s -e --arg host "${DEMO_HOST}" \
            'any(.[]; .payload.hostname == $host and .payload.threshold >= .payload.daysLeft)' \
            "${TMP_DIR}/webhook-alerts.jsonl" >/dev/null 2>&1; then
            ready=1
            break
        fi
    fi
    sleep 1
done
[[ "${ready}" == "1" ]] || fail "webhook alert was not delivered"

log "==> Querying Prometheus for certificate_days_left"
ready=0
for _ in $(seq 1 30); do
    if curl -fsS -G "${PROM_URL}/api/v1/query" \
        --data-urlencode "query=certificate_days_left{host=\"${DEMO_HOST}\"}" \
        -o "${TMP_DIR}/prometheus.json" 2>/dev/null &&
        jq -e '.data.result | length > 0' "${TMP_DIR}/prometheus.json" >/dev/null 2>&1; then
        ready=1
        break
    fi
    sleep 1
done
[[ "${ready}" == "1" ]] || fail "Prometheus did not return certificate_days_left"

log "==> Running the CLI against the demo config"
cli_out="$("${COMPOSE[@]}" exec -T certificate-validate certificate-validate check -c /tmp/settings.yml)"
printf '%s\n' "${cli_out}" > "${TMP_DIR}/cli-check.json"
jq -e --arg host "${DEMO_HOST}" '.hostname == $host and .daysLeft > 0' \
    "${TMP_DIR}/cli-check.json" >/dev/null 2>&1 || fail "CLI check did not return a valid certificate"

summary_content="$(cat <<EOF
## Compose demo

| Check | Result | Detail |
| --- | --- | --- |
| API health | ✅ | \`ok\` |
| Certificate API | ✅ | \`${DEMO_HOST}:${DEMO_PORT}\` |
| Prometheus metrics | ✅ | \`certificate_days_left\` exposed |
| Webhook alert | ✅ | payload delivered |
| Prometheus query | ✅ | \`certificate_days_left\` returned data |
| CLI check | ✅ | config check passed |

**Target:** \`${DEMO_HOST}:${DEMO_PORT}\` · **Days left:** \`${days_left}\`
EOF
)"

if [[ -n "${ARTIFACTS_DIR}" ]]; then
    cp "${TMP_DIR}/cert.json" "${ARTIFACTS_DIR}/certificate.json"
    cp "${TMP_DIR}/metrics.txt" "${ARTIFACTS_DIR}/metrics.txt"
    cp "${TMP_DIR}/webhook-alerts.jsonl" "${ARTIFACTS_DIR}/webhook-alerts.jsonl"
    cp "${TMP_DIR}/prometheus.json" "${ARTIFACTS_DIR}/prometheus.json"
    cp "${TMP_DIR}/cli-check.json" "${ARTIFACTS_DIR}/cli-check.json"
    printf '%s\n' "${summary_content}" > "${ARTIFACTS_DIR}/summary.md"
fi

printf '\n%s\n' "${summary_content}"
