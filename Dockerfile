
FROM python:3.9-alpine as build

COPY certificate.py requirements.txt entrypoint.sh /app/

ENV CERTIFICATE_URL \
    CERTIFICATE_PORT \
    CERTIFICATE_TIME_TO_WAIT

RUN adduser --disabled-password --gecos "" python \
    && apk add --no-cache \
        make \
        sshpass \
        openssh \
        gcc \
        g++ \
        libffi-dev \
        openssl \
        openssl-dev \
    && rm -vrf /var/cache/apk/* \
    && pip install --upgrade pip wheel setuptools \
    && pip install -r /app/requirements.txt \
    && chown -R python:python /app \
    && chmod +x /app/entrypoint.sh

USER python

ENTRYPOINT ["/app/entrypoint.sh"]