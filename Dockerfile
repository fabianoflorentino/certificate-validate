FROM golang:1.26-alpine3.23 AS build

WORKDIR /src

COPY . .

RUN go mod download \
  && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/certificate-validate ./cmd/certificate-validate

FROM alpine:3.23 AS production

RUN adduser -D -u 1000 appuser

COPY --from=build /bin/certificate-validate /usr/local/bin/certificate-validate
COPY config/settings.yml /app/config/settings.yml

USER appuser
WORKDIR /app

VOLUME ["/app/config"]

ENTRYPOINT ["certificate-validate"]
CMD ["check"]
