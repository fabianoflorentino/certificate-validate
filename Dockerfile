# syntax=docker/dockerfile:1
#
# Multi-stage Dockerfile for certificate-validate.
#
# Two named build targets (select with `docker build --target ...` or
# `build.target` in docker-compose.yml):
#
#   production (default) — the runtime image for `check` / `serve`.
#   dev                  — the disposable Kubernetes dev image (kind, kubectl,
#                          helm, Go) used to test the `k8s monitor` agent.

# ---------- Stage: build (compile the Go binary, shared) ----------
FROM golang:1.27-alpine3.24 AS build

WORKDIR /src

COPY . .

RUN go mod download \
  && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/certificate-validate ./cmd/certificate-validate

# ---------- Stage: dev (Kubernetes tooling for the disposable dev environment) --
FROM golang:1.27-alpine3.24 AS dev

RUN apk add --no-cache \
        bash \
        curl \
        git \
        make \
        docker-cli \
        jq \
        yq

# kubectl (matched to the kind default node image Kubernetes 1.36)
ARG KUBECTL_VERSION=v1.36.1
RUN curl -fsSL -o /usr/local/bin/kubectl \
        "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    && chmod +x /usr/local/bin/kubectl

# helm (v3 for compatibility with the project's existing chart; Helm 4 also exists)
ARG HELM_VERSION=v3.21.4
RUN curl -fsSL "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz" | tar -xz -C /tmp \
    && mv /tmp/linux-amd64/helm /usr/local/bin/helm \
    && rm -rf /tmp/linux-amd64

# kind (defaults to Kubernetes 1.36.1)
ARG KIND_VERSION=v0.32.0
RUN curl -fsSL -o /usr/local/bin/kind \
        "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-linux-amd64" \
    && chmod +x /usr/local/bin/kind

WORKDIR /workspace

COPY scripts/ /usr/local/bin/dev-scripts/

ENTRYPOINT ["/bin/bash"]

# ---------- Stage: production (runtime image) --------------------------------
FROM alpine:3.24 AS production

RUN adduser -D -u 1000 appuser

COPY --from=build /bin/certificate-validate /usr/local/bin/certificate-validate
COPY config/settings.yml /app/config/settings.yml

RUN mkdir -p /app/data && chown appuser:appuser /app/data /app/config

USER appuser
WORKDIR /app

VOLUME ["/app/config", "/app/data"]

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:5000/health > /dev/null 2>&1 || exit 1

ENTRYPOINT ["certificate-validate"]
CMD ["check"]
