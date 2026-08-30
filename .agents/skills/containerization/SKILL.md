---
name: containerization
description: Docker and docker-compose best practices for building, running, and maintaining container images and local dev environments. Use when editing the Dockerfile, docker-compose, or adding multi-stage builds, healthchecks, or dev containers.
origin: ECC
---

# Containerization

Patterns for keeping the project's container images and local compose dev
environment small, correct, and easy to maintain. This project uses a single
multi-stage `Dockerfile` (build / dev / production) and a single
`docker-compose.yml` (app + `dev` profile).

## When to Activate

- Editing the `Dockerfile` or `docker-compose.yml`
- Adding or changing build stages, targets, or healthchecks
- Working on the disposable dev environment (kind, cert-manager)
- Changing base images, dependencies, or exposed ports

## Multi-Stage Dockerfile

Use multiple stages so only the runtime prerequisites end up in the final image:

```dockerfile
# syntax=docker/dockerfile:1

# ---- build ----
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/app ./cmd/certificate-validate

# ---- production ----
FROM alpine:3.21 AS production
RUN adduser -D appuser
COPY --from=build /out/app /usr/local/bin/app
USER appuser
EXPOSE 8080 9102
ENTRYPOINT ["/usr/local/bin/app"]
```

Key habits:
- Order instructions by change frequency: base image and module downloads early,
  source code copy last, to maximize layer cache hits.
- Pin base image tags; prefer explicit versions over `latest`.
- Static binaries (`CGO_ENABLED=0`) let you use a small runtime image and a
  non-root user.
- Each stage has a single responsibility and a clear `AS` name.

## .dockerignore

Ignore local artifacts so they never invalidate cache or leak into the image:

```dockerignore
.git
bin/
coverage.out
*.log
**/*_test.go
site/public/
```

## Compose: Separation of Concerns

The `docker-compose.yml` separates the app from the dev environment:

- **`certificate-validate`** service: runs the application (default target).
- **`dev`** service: disposable development environment, gated behind the `dev`
  compose profile so `docker compose up` only starts the app:
  `docker compose --profile dev up`.

Model related variables and volumes with clear names, and keep credentials out
of the file (use env files or the shell).

## Dev Environment Notes

This repo's dev compose uses `network_mode: host` so the kind kubeconfig
(`127.0.0.1:<port>`) works from inside the container, and sets `KUBECONFIG` as a
compose env var. When changing the dev profile, preserve those two
requirements or the in-container `kubectl` will not reach the cluster.

## Health Checks

Prefer real healthchecks over `curl` in minimal images. For the Go app:

```yaml
healthcheck:
  test: ["CMD", "/usr/local/bin/app", "health"]
  interval: 30s
  timeout: 5s
  retries: 3
```

**Remember**: small images with pinned versions, cache-aware instruction order,
non-root runtime users, and a clear split between the app and the disposable dev
environment.
