FROM golang:1.26-alpine3.24 AS build

WORKDIR /src

COPY . .

RUN go mod download \
  && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/certificate-validate ./cmd/certificate-validate

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
