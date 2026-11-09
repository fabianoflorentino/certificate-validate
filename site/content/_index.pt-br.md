---
title: "certificate-validate"
description: "Monitore certificados SSL/TLS pela CLI, API HTTP ou Kubernetes com verificações OCSP/CRL, métricas Prometheus e alertas via webhook."
---

<a id="problems"></a>
## O que quebra sem validação

<div class="problem-grid">
    <article class="problem-card">
        <span class="problem-kicker">01 · Expiração</span>
        <h3>A indisponibilidade com data marcada</h3>
        <p>Um certificado chega a zero dias e o TLS começa a falhar. Valide os dias restantes continuamente e alerte antes do prazo, não depois do incidente.</p>
    </article>
    <article class="problem-card">
        <span class="problem-kicker">02 · Revogação</span>
        <h3>Revogado, mas ainda confiável</h3>
        <p>Um certificado pode ser revogado e ainda parecer válido. As verificações OCSP e CRL detectam o que a data de expiração sozinha não detecta.</p>
    </article>
    <article class="problem-card">
        <span class="problem-kicker">03 · Kubernetes</span>
        <h3>TLS espalhado por namespaces</h3>
        <p>Secrets e Ingresses se multiplicam entre equipes. Descubra todos os recursos TLS em uma única varredura, com labels e métricas associadas.</p>
    </article>
    <article class="problem-card">
        <span class="problem-kicker">04 · Confiança</span>
        <h3>Cadeias internas falham em silêncio</h3>
        <p>CAs privadas, certificados autoassinados e cadeias incompletas quebram de formas que checks de uptime não percebem. Inspecione a cadeia e o caminho de confiança.</p>
    </article>
    <article class="problem-card">
        <span class="problem-kicker">05 · Observabilidade</span>
        <h3>Nenhum sinal para o plantão</h3>
        <p>Um dashboard que ninguém olha não é monitoramento. Exporte métricas Prometheus e dispare alertas por webhook com limite de frequência para as ferramentas que sua equipe já usa.</p>
    </article>
    <article class="problem-card">
        <span class="problem-kicker">06 · Operações</span>
        <h3>Renovações e auditorias são manuais</h3>
        <p>Histórico, rotação de JSONL e renovação opcional via cert-manager transformam o trabalho recorrente com certificados em um processo repetível.</p>
    </article>
</div>

<a id="quickstart"></a>
## Início rápido

Instale o binário:

```bash
go install github.com/fabianoflorentino/certificate-validate/cmd/certificate-validate@latest
```

Verifique qualquer host público sem arquivo de configuração:

```bash
certificate-validate check --host github.com --port 443 -o table
```

Depois escolha seu caminho:

<div class="quick-paths">
    <a href="#cli-usage">
        <span>CLI</span>
        <strong>Faça uma verificação avulsa por script</strong>
        <em>Saída em tabela, JSON e CSV</em>
    </a>
    <a href="#api-endpoints">
        <span>API</span>
        <strong>Execute o dashboard</strong>
        <em>REST, Swagger e /metrics</em>
    </a>
    <a href="#docker--kubernetes">
        <span>K8S</span>
        <strong>Monitore um cluster</strong>
        <em>Secrets, Ingresses e renovação</em>
    </a>
</div>

Prefere contêineres? O mesmo binário vem em uma imagem de ~10 MB:
`docker pull fabianoflorentino/certificate-validate:latest`.

<a id="features"></a>
## Recursos

<div class="feature-grid">
    <article class="feature-card">
        <span class="feature-kicker">01 · CLI</span>
        <h3>CLI e exportação</h3>
        <p>Verifique um host ou todos os endpoints configurados. Emita JSON ou tabelas alinhadas e exporte para CSV pela CLI ou API.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">02 · API</span>
        <h3>API HTTP e dashboard</h3>
        <p>Endpoints REST, Swagger UI, autenticação por API key, rate limiting e um dashboard embutido com gráficos de histórico.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">03 · MONITOR</span>
        <h3>Modo watch</h3>
        <p>Reveja em intervalos e destaque certificados que cruzam seu limite de dias restantes.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">04 · REVOGAÇÃO</span>
        <h3>OCSP + CRL</h3>
        <p>Verifique a revogação com OCSP primeiro e recorra aos pontos de distribuição de CRL quando necessário.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">05 · KUBERNETES</span>
        <h3>Monitor de cluster</h3>
        <p>Descubra Secrets e Ingresses TLS, analise cadeias, exporte métricas e renove automaticamente via cert-manager quando habilitado.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">06 · MÉTRICAS</span>
        <h3>Pronto para Prometheus</h3>
        <p>Exponha <code>certificate_days_left</code>, <code>certificate_expired</code>, <code>certificate_revoked</code> e contadores de renovação.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">07 · ALERTAS</span>
        <h3>Alertas por webhook</h3>
        <p>Envie JSON estruturado via POST quando um certificado se aproxima da expiração, com limite de frequência por recurso para evitar spam de alertas.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">08 · OPS</span>
        <h3>Histórico e hot reload</h3>
        <p>Registre resultados de verificação em JSONL com rotação automática e recarregue a configuração com <code>SIGHUP</code> sem reiniciar.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">09 · CONFIANÇA</span>
        <h3>CAs privadas</h3>
        <p>Confie em bundles de CA globais ou por host para certificados internos, autoassinados e assinados por CAs privadas.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">10 · RUNTIME</span>
        <h3>Busca concorrente</h3>
        <p>Verifique muitos hosts em paralelo com um semáforo limitado, mantendo as varreduras rápidas e previsíveis.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">11 · CONFIGURAÇÃO</span>
        <h3>Sobrescrita por ambiente</h3>
        <p>Sobrescreva qualquer campo de configuração com variáveis de ambiente com prefixo <code>CV_</code>.</p>
    </article>
    <article class="feature-card">
        <span class="feature-kicker">12 · IMAGEM</span>
        <h3>Pegada mínima</h3>
        <p>Entregue uma imagem de contêiner de ~10 MB sem root com a CLI, a API e o monitor Kubernetes em um único binário.</p>
    </article>
</div>

## Como funciona

<div class="steps-grid">
    <div class="step-card">
        <span class="step-number">01</span>
        <h3>Descobrir</h3>
        <p>Leia hosts de um arquivo YAML ou escaneie Secrets e Ingresses do Kubernetes em todos os namespaces.</p>
    </div>
    <div class="step-card">
        <span class="step-number">02</span>
        <h3>Validar</h3>
        <p>Complete o handshake TLS, inspecione a cadeia, calcule os dias restantes e verifique a revogação via OCSP/CRL.</p>
    </div>
    <div class="step-card">
        <span class="step-number">03</span>
        <h3>Agir</h3>
        <p>Imprima tabelas ou JSON, sirva a API e o dashboard, exponha métricas, envie webhooks e renove via cert-manager.</p>
    </div>
</div>

<a id="demo"></a>
## Demonstração

Saída real de execuções da ferramenta.

<div class="demo-gallery">
    <figure class="demo-shot demo-shot-wide">
        <div class="demo-frame">
            {{< img src="demo/dashboard.png" alt="Dashboard do certificate-validate mostrando dez certificados TLS com dias restantes e status de validação" >}}
        </div>
        <figcaption><strong>Dashboard</strong> — busca, ordenação, badges de expiração, status de revogação e histórico na interface web embutida.</figcaption>
    </figure>
    <div class="demo-grid">
        <figure class="demo-shot">
            <div class="demo-frame">
                {{< img src="demo/cli.png" alt="Tabela da CLI do certificate-validate mostrando dez hosts, dias restantes, status, revogação, emissor e versão de TLS" >}}
            </div>
            <figcaption><strong>CLI</strong> — um comando, uma tabela completa com hosts, expiração, revogação, emissor e versão de TLS.</figcaption>
        </figure>
        <figure class="demo-shot">
            <div class="demo-frame">
                {{< img src="demo/kubernetes.png" alt="Monitor k8s do certificate-validate mostrando métricas Prometheus e um payload de alerta por webhook" >}}
            </div>
            <figcaption><strong>Kubernetes</strong> — descoberta de Secrets e Ingresses TLS, métricas Prometheus e alertas por webhook estruturados.</figcaption>
        </figure>
    </div>
</div>

## Para quem é

<div class="use-case-grid">
    <div class="use-case-card">
        <span class="use-case-label">SRE / Plantão</span>
        <h3>Acabe com os acionamentos por expiração</h3>
        <p>Veja o próximo prazo, alerte nos seus limites e mantenha um histórico de verificações para inspecionar depois de um incidente.</p>
    </div>
    <div class="use-case-card">
        <span class="use-case-label">Plataforma / Kubernetes</span>
        <h3>Veja todos os recursos TLS</h3>
        <p>Inventarie Secrets e Ingresses em todos os namespaces, rotule métricas por recurso e opte pela renovação via cert-manager.</p>
    </div>
    <div class="use-case-card">
        <span class="use-case-label">Segurança / Compliance</span>
        <h3>Comprove revogação e confiança</h3>
        <p>Valide o status OCSP/CRL e as cadeias de certificados, incluindo CAs privadas, e exporte as evidências em JSON ou CSV.</p>
    </div>
</div>

<a id="cli-usage"></a>
## Uso da CLI

```bash
# Verifica todos os hosts da configuração
certificate-validate check

# Saída em tabela
certificate-validate check -o table

# Um único host, sem configuração
certificate-validate check --host github.com --port 443

# Filtra por dias restantes
certificate-validate check --min-days 30

# Modo de observação contínua
certificate-validate check --watch

# Exporta para CSV
certificate-validate export -f csv -o certs.csv

# Inicia o servidor da API
certificate-validate serve

# Inicia o servidor da API com TLS
certificate-validate serve --tls-cert cert.pem --tls-key key.pem

# Kubernetes: varredura única de Secrets + Ingresses TLS
certificate-validate k8s monitor

# Kubernetes: modo watch com métricas + alertas por webhook
certificate-validate k8s monitor --watch-interval=300 --metrics-addr=:9102 --webhook-url https://hooks.example.com/alert

# Kubernetes: habilita a renovação automática de certificados expirando via cert-manager
certificate-validate k8s monitor --renew-threshold=15
```

<a id="api-endpoints"></a>
## Endpoints da API

| Método | Rota | Descrição |
|--------|------|-----------|
| GET | `/` | Dashboard web |
| GET | `/swagger/` | Swagger UI (documentação interativa da API) |
| GET | `/health` | Verificação de saúde |
| GET | `/api/v1/cert/info/all` | Todos os certificados |
| GET | `/api/v1/cert/info/{hostname}` | Certificado de um host específico |
| GET | `/api/v1/cert/export/json` | Baixar como JSON |
| GET | `/api/v1/cert/export/csv` | Baixar como CSV |
| GET | `/api/v1/cert/history/{hostname}` | Histórico de verificações |
| GET | `/metrics` | Métricas Prometheus |

## Observabilidade

As métricas Prometheus são expostas em `/metrics` (servidor principal) ou no
endereço de métricas do `k8s monitor`, usando o prefixo `certificate_`:

| Métrica | Rótulos | Significado |
|---------|---------|-------------|
| `certificate_days_left` | `host`, `port` (principal); `namespace`, `name`, `kind` (K8s) | Dias restantes antes da expiração |
| `certificate_expired` | os mesmos | `1` se expirado, `0` se válido |
| `certificate_revoked` | `namespace`, `name`, `kind` | `1` se OCSP/CRL indicar revogado |
| `certificate_renewal_total` | `namespace`, `name`, `status` | Tentativas de renovação (`success`/`failure`) |

```promql
# Certificados que expiram em até 15 dias
certificate_days_left < 15

# Certificados expirados
certificate_expired == 1
```

Certificados prestes a expirar enviam um alerta JSON via POST para o webhook
configurado. Uma demonstração ponta a ponta executável (kind + cert-manager +
receptor de webhook) está disponível com `make dev/up`, `make dev/setup` e
`make dev/alert-demo`; veja o
[guia de desenvolvimento](https://github.com/fabianoflorentino/certificate-validate/blob/main/dev/README.md#end-to-end-monitoring-and-alerting-demo).
Para o deploy local completo no kind — dashboard, ingress/TLS, monitor e
Prometheus/Grafana — execute `make demo/kind-full`.

## Configuração

```yaml
check_time: 30                    # Intervalo de verificação (segundos)
api_key: "sk-1234"                # Autenticação opcional por API key

app_configs:
  - name: 'certificate-validate'
    host: '0.0.0.0'
    port: '5000'

hosts:
  - name: "GitHub"
    url: 'github.com'
    port: '443'
    # ports: [443, 8443]          # Múltiplas portas
    # timeout: 10                 # Timeout por host
    # trusted_cas:                # CAs por host
    #   - '/certs/internal-ca.pem'

prometheus:
  enabled: false                  # Habilita o endpoint /metrics

webhook:
  url: 'https://hooks.example.com/alert'
  threshold: 15                   # Alerta quando os dias restantes < limite
  interval: 1800

history:
  enabled: true
  file_path: "data/history.jsonl"
  max_entries: 10000
  max_days: 90

trusted_cas:
  - '/etc/certificates/my-ca.pem'
```

<a id="docker--kubernetes"></a>
## Docker e Kubernetes

```bash
# CLI no Docker
docker run -v $(pwd)/config:/app/config certificate-validate check

# Servidor da API no Docker
docker run -p 5000:5000 -v $(pwd)/config:/app/config certificate-validate serve

# Docker Compose
docker-compose up -d
```

Os manifestos Kubernetes para o agente `k8s monitor` (DaemonSet, RBAC, Service,
ServiceMonitor) ficam em [`kubernetes/monitor/`](https://github.com/fabianoflorentino/certificate-validate/tree/main/kubernetes/monitor).
Um ambiente de desenvolvimento descartável (kind + cert-manager) está documentado em
[`dev/README.md`](https://github.com/fabianoflorentino/certificate-validate/blob/main/dev/README.md)
e é acionado pelos alvos `make dev/*`.

### Demonstração local no Kubernetes (kind)

Quer o stack completo rodando localmente? Com Docker, `kind`, `kubectl`, `helm`
e Go instalados:

```bash
make demo/kind-full
```

Isso cria um cluster kind descartável com cert-manager e ingress-nginx, faz o
deploy do dashboard via HTTPS (TLS emitido pelo cert-manager), executa o
DaemonSet `k8s monitor` e entrega alertas a um receptor de webhook no cluster.
Prometheus e Grafana são instalados por padrão; use `WITH_PROMETHEUS=false` para
uma execução mais rápida sem o stack de monitoramento.

```bash
# Dashboard (TLS autoassinado)
curl -k --resolve certificate-validate.localhost:8443:127.0.0.1 \
  https://certificate-validate.localhost:8443/health
```

Passo a passo completo (pré-requisitos, acesso, variáveis e troubleshooting):
[`demo/README.md`](https://github.com/fabianoflorentino/certificate-validate/blob/main/demo/README.md#full-kubernetes-example-kind).

## Saiba mais

- [Documentação de arquitetura](https://github.com/fabianoflorentino/certificate-validate/blob/main/docs/ARCHITECTURE.md)
- [Guia de integração com Kubernetes](https://github.com/fabianoflorentino/certificate-validate/blob/main/docs/K8S_INTEGRATION.md)
- [Documentação Go](https://pkg.go.dev/github.com/fabianoflorentino/certificate-validate)
- [Docker Hub](https://hub.docker.com/r/fabianoflorentino/certificate-validate)
- [Releases no GitHub](https://github.com/fabianoflorentino/certificate-validate/releases)
