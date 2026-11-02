FROM nginx:1.27-alpine

RUN apk add --no-cache openssl

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY entrypoint.sh /usr/local/bin/tls-entrypoint.sh

RUN chmod +x /usr/local/bin/tls-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/tls-entrypoint.sh"]
