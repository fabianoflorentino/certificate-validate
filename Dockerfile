FROM python:3.9-alpine as build

LABEL maintainer="Fabiano Florentino"
LABEL email="fabianoflorentino@outlook.com"
LABEL image version="v0.30"

COPY certificate.py api.py settings.py requirements.txt entrypoint.sh /app/

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
    && mkdir -p /app/config \
    && chown -R python:python /app \
    && chmod +x /app/entrypoint.sh

COPY config/settings.yml /app/config/settings.yml

USER python

WORKDIR /app

VOLUME ["/app/config"]

ENTRYPOINT ["/app/entrypoint.sh"]