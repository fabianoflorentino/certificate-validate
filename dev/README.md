# Ambiente de Desenvolvimento Kubernetes (descartável)

Este ambiente permite testar a integração Kubernetes (`k8s monitor`) com
**cert-manager** sem instalar nenhuma ferramenta de Kubernetes na sua máquina
além do Docker.

## Arquitetura

```
Máquina host
└── Docker engine (único pré-requisito no host)
    └── Container "dev" (descartável, imagem versionada no repo)
        ├── kind (cluster Kubernetes)
        │   └── cert-manager (via Helm)
        │   └── certificate-validate k8s monitor (DaemonSet)
        ├── kubectl
        ├── helm
        └── Go toolchain
```

- A imagem do container contém **todo** o tooling (kind, kubectl, helm, Go).
- O container dev usa `network_mode: host` — necessário porque o `kubeconfig` do cluster
  kind aponta para o API server em `127.0.0.1:<porta>` (bound no loopback do host), e o
  container precisa da rede do host para alcançá-lo.
- O cluster kind vive nos containers do Docker do host, mas toda a
  configuração (`kubeconfig`, estado do cert-manager) fica num **volume
  nomeado** descartável.
- Nada é instalado no host além do Docker engine.

## Pré-requisitos

- Docker engine funcionando no host — **inclusive o plugin `docker compose`**
  (no Arch: `sudo pacman -S docker-compose`).
- Seu usuário precisa ter permissão no socket do Docker (`sudo usermod -aG docker $USER`
  + re-login, ou rodar via `newgrp docker`).
- **DNS funcional no daemon Docker.** Se o host resolver via DNS local (ex. Cloudflare
  WARP com `127.x`), o daemon pode injetar nameservers inalcançáveis (`8.8.8.8`) nos
  containers e o `apk add`/respectivas descargas do build falham com `DNS: transient
  error`. Desative ferramentas de DNS local (WARP) **ou** reinicie o daemon
  (`sudo systemctl restart docker`) para que ele releia o `/etc/resolv.conf` do host.

## Uso

A partir da raiz do repositório:

```bash
# 1. Subir o container dev (builda a imagem descartável)
make dev/up

# 2. Entrar no container
make dev/shell

# 3. Dentro do container: criar o cluster kind + cert-manager + deploy do monitor
setup-cluster.sh
```

Todos os comandos Makefile:

| Alvo | Descrição |
|------|-----------|
| `make dev/up` | Builda e inicia o container dev |
| `make dev/shell` | Abre um shell no container dev |
| `make dev/setup` | Cria cluster kind + cert-manager + deploy do monitor |
| `make dev/scan` | Roda um scan único do `k8s monitor` através do DaemonSet |
| `make dev/logs` | Acompanha os logs do monitor |
| `make dev/down` | Para o container dev |
| `make dev/destroy` | Destrói container e todo o estado do cluster |

## Comandos úteis dentro do container

```bash
# Verificar o cluster e o cert-manager
kubectl get nodes
kubectl -n cert-manager get pods

# Rodar um scan único do monitor
kubectl -n cert-manager exec daemonset/cert-validate-monitor -- \
    certificate-validate k8s monitor

# Acompanhar logs do monitor
monitor-logs.sh
```

## Exemplo de teste rápido

A forma mais simples de o monitor encontrar um certificado é emitir um via **cert-manager**
(que já vem instalado pelo `setup-cluster.sh`) e referenciá-lo num Ingress:

```bash
kubectl apply -f - <<'EOF'
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned
  namespace: default
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: example-tls
  namespace: default
spec:
  secretName: example-tls-secret
  duration: 2160h
  renewBefore: 360h
  privateKey:
    algorithm: RSA
    encoding: PKCS1
    size: 2048
  dnsNames:
    - example.com
  issuerRef:
    name: selfsigned
    kind: Issuer
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: example-ingress
  namespace: default
spec:
  tls:
    - hosts: [example.com]
      secretName: example-tls-secret
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: example-svc
                port:
                  number: 80
EOF

kubectl wait --for=condition=Ready certificate/example-tls -n default --timeout=60s
```

Depois rode um scan — o monitor deve descobrir o Secret TLS **e** o Ingress:

```bash
make dev/scan
```

> Alternativa sem cert-manager: `kubectl create secret tls example-tls \
> --cert=/path/to/tls.crt --key=/path/to/tls.key` (precisa de um par de chaves reais).

## Limpeza completa

```bash
make dev/down        # para o container
make dev/destroy     # remove container + volumes (zero resíduo)
```

## Personalização

Versões via variáveis de ambiente (no `setup-cluster.sh`):

- `CLUSTER_NAME` (padrão `cert-validate`)
- `CERT_MANAGER_VERSION` (padrão `v1.21.1`)
- `KIND_IMAGE` (padrão `kindest/node:v1.36.1`)

Imagem base (`Dockerfile` stage `dev`):

- `KUBECTL_VERSION` (padrão `v1.36.1`)
- `HELM_VERSION` (padrão `v3.21.4`)
- `KIND_VERSION` (padrão `v0.32.0`)

> **Layout:** a imagem dev é o alvo `dev` do **mesmo** `Dockerfile` de produção, e o
> serviço `dev` fica no **mesmo** `docker-compose.yml` da raiz (profile `dev`, para não
> subir por padrão). Os scripts (`setup-cluster.sh`, `teardown.sh`, `monitor-logs.sh`)
> ficam em `scripts/` na raiz e são copiados para `/usr/local/bin/dev-scripts/` dentro do
> container — por isso os comandos `setup-cluster.sh` / `monitor-logs.sh` funcionam
> direto no shell do container.

## Notas

- O manifesto do monitor (`kubernetes/monitor/`) usa RBAC **read-only**
  (monitoramento). A permissão `update` em Secrets e a criação de eventos são
  adicionadas na **Fase 2** (auto-renovação via anotação `cert-manager.io/force-renew`).
- O `ServiceMonitor` só é aplicado se o Prometheus Operator estiver instalado
  no cluster.
