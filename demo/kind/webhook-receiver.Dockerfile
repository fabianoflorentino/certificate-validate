# syntax=docker/dockerfile:1
#
# Small in-cluster image for the demo webhook receiver. Kept separate from the
# production Dockerfile so the runtime image stays limited to certificate-validate.
FROM golang:1.27-alpine3.24 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/webhook-receiver ./scripts/webhook-receiver

FROM alpine:3.24

COPY --from=build /out/webhook-receiver /usr/local/bin/webhook-receiver

USER 65532:65532
ENTRYPOINT ["webhook-receiver"]
CMD ["-addr", "0.0.0.0:9099", "-out", "/data/alerts.jsonl"]
