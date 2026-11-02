#!/bin/sh
# Serves a TLS endpoint for the demo. If /certs contains a certificate/key pair
# it is used as-is; otherwise a short-lived self-signed certificate is generated.
# To use real certificates, drop server.crt (full chain) and server.key into
# demo/compose/certs/ and set DEMO_HOST to the certificate's DNS name.
set -eu

host="${DEMO_HOST:-tls-origin}"
days="${TLS_DAYS:-14}"

mkdir -p /tls

if [ -s /certs/server.crt ] && [ -s /certs/server.key ]; then
    echo "tls-origin: using provided certificate for ${host}"
    cp /certs/server.crt /tls/server.crt
    cp /certs/server.key /tls/server.key
elif [ ! -s /tls/server.crt ] || [ ! -s /tls/server.key ]; then
    echo "tls-origin: generating self-signed certificate for ${host} (${days} days)"
    openssl req -x509 -newkey rsa:2048 -sha256 -nodes \
        -days "${days}" \
        -subj "/CN=${host}" \
        -addext "subjectAltName=DNS:${host},DNS:localhost,IP:127.0.0.1" \
        -keyout /tls/server.key \
        -out /tls/server.crt
else
    echo "tls-origin: reusing existing certificate in /tls"
fi

chmod 0644 /tls/server.crt
chmod 0600 /tls/server.key

exec nginx -g 'daemon off;'
