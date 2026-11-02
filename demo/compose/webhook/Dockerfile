FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY scripts/webhook-receiver ./scripts/webhook-receiver

RUN CGO_ENABLED=0 go build -o /out/webhook-receiver ./scripts/webhook-receiver

FROM alpine:3.20

RUN adduser -D -u 10001 appuser \
    && mkdir -p /data \
    && chown appuser:appuser /data

COPY --from=build /out/webhook-receiver /usr/local/bin/webhook-receiver

USER appuser

EXPOSE 9099

ENTRYPOINT ["webhook-receiver", "-addr", "0.0.0.0:9099", "-out", "/data/alerts.jsonl"]
