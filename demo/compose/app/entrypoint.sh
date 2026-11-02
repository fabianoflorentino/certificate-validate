#!/bin/sh
# Renders the demo configuration from environment variables and starts the API.
set -eu

: "${DEMO_HOST:=tls-origin}"
: "${DEMO_PORT:=443}"
: "${CHECK_TIME:=5}"
: "${ALERT_THRESHOLD:=30}"
: "${ALERT_INTERVAL:=5}"

# Generated self-signed certificates need an explicit trust anchor. Provided
# certificates use the system roots unless a private CA bundle is supplied.
trusted_ca=""
if [ -s /certs/ca.crt ]; then
    trusted_ca="/certs/ca.crt"
elif [ ! -s /certs/server.crt ]; then
    trusted_ca="/tls/server.crt"
fi

sed \
    -e "s|__DEMO_HOST__|${DEMO_HOST}|g" \
    -e "s|__DEMO_PORT__|${DEMO_PORT}|g" \
    -e "s|__CHECK_TIME__|${CHECK_TIME}|g" \
    -e "s|__ALERT_THRESHOLD__|${ALERT_THRESHOLD}|g" \
    -e "s|__ALERT_INTERVAL__|${ALERT_INTERVAL}|g" \
    -e "s|__TRUSTED_CA__|${trusted_ca}|g" \
    /demo/config/settings.template.yml > /tmp/settings.yml

echo "certificate-validate: demo target ${DEMO_HOST}:${DEMO_PORT} (trust ${trusted_ca:-system roots})"
exec certificate-validate serve -c /tmp/settings.yml
