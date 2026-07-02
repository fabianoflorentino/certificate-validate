# Architecture

Este documento descreve a arquitetura, fluxos e componentes do **certificate-validate**,
uma ferramenta moderna e extensível de validação SSL/TLS escrita em Go.

---

## Índice

1. [Visão Geral da Arquitetura](#1-visão-geral-da-arquitetura)
2. [Hierarquia de Comandos CLI](#2-hierarquia-de-comandos-cli)
3. [Fluxo de Checagem de Certificado](#3-fluxo-de-checagem-de-certificado)
4. [Fluxo de Requisição HTTP (API)](#4-fluxo-de-requisição-http-api)
5. [Serviços de Background](#5-serviços-de-background)
6. [Pipeline de Carregamento de Configuração](#6-pipeline-de-carregamento-de-configuração)
7. [Injeção de Dependências e Inicialização](#7-injeção-de-dependências-e-inicialização)
8. [Exportação de Dados](#8-exportação-de-dados)
9. [Checagem de Revogação](#9-checagem-de-revogação)

---

## 1. Visão Geral da Arquitetura

O projeto segue **Clean Architecture** com princípios **SOLID**,
separando o código em camadas concêntricas:

```mermaid
flowchart TB
    subgraph Entry["Entry Point"]
        main["cmd/certificate-validate/main.go"]
    end

    subgraph Interface["INTERFACE - I"]
        cmdLayer["internal/cmd/\n(root, check, serve, export, version, completion)"]
        apiLayer["internal/api/\n(HTTP handlers, middleware, rate limiter)"]
    end

    subgraph UseCase["USE CASE - O, L"]
        checkerLayer["internal/checker/\n(Checker: orquestra fetch + format)"]
        serviceLayer["internal/service/\n(CertService: CheckAll, CheckSingle, GetHistory)"]
    end

    subgraph Domain["DOMAIN - S, D"]
        certLayer["internal/certificate/\n(VO Certificate, FromX509, BuildChain, TLSVersionString)"]
    end

    subgraph Providers["PROVIDERS - D, I"]
        fetcherLayer["internal/fetcher/\n(interface + implementação TLS)"]
        formatterLayer["internal/formatter/\n(JSONFormatter, FormatTable, FormatJSON, FormatCSV)"]
        notifierLayer["internal/notifier/\n(webhook alerts)"]
        historyLayer["internal/history/\n(Recorder, Store, JSONL rotation)"]
        metricsLayer["internal/metrics/\n(Prometheus gauges)"]
        revocationLayer["internal/revocation/\n(OCSP + CRL checks)"]
    end

    subgraph Config["CONFIG"]
        configLayer["internal/config/\n(YAML loader + env var overrides + validação)"]
        yamlFile["config/settings.yml"]
    end

    main --> cmdLayer
    cmdLayer --> configLayer
    cmdLayer --> serviceLayer
    cmdLayer --> checkerLayer
    cmdLayer --> apiLayer
    cmdLayer --> fetcherLayer
    cmdLayer --> formatterLayer
    cmdLayer --> notifierLayer
    cmdLayer --> historyLayer
    cmdLayer --> metricsLayer
    configLayer --> yamlFile
    checkerLayer --> fetcherLayer
    checkerLayer --> formatterLayer
    checkerLayer --> certLayer
    fetcherLayer --> certLayer
    fetcherLayer --> revocationLayer
    serviceLayer --> checkerLayer
    serviceLayer --> historyLayer
    serviceLayer --> metricsLayer
    apiLayer --> serviceLayer
    apiLayer --> configLayer
    apiLayer --> metricsLayer
    notifierLayer --> checkerLayer
    historyLayer --> checkerLayer
    metricsLayer --> checkerLayer
```

### Princípios SOLID Aplicados

| Princípio | Implementação |
|---|---|
| **S** - Single Responsibility | Cada pacote tem uma responsabilidade: `certificate` = domínio, `fetcher` = TLS, `formatter` = saída, `checker` = orquestração |
| **O** - Open/Closed | Interfaces `Fetcher` e `Formatter` permitem novas implementações sem modificar código existente |
| **L** - Liskov Substitution | Retornos explícitos `(Certificate, error)` — sem `sys.Exit()` ou tipos inconsistentes |
| **I** - Interface Segregation | Interfaces mínimas: `Fetcher` tem 1 método, `Formatter` tem 1 método |
| **D** - Dependency Inversion | `checker` define interfaces, providers implementam. `main.go` injeta dependências |

---

## 2. Hierarquia de Comandos CLI

```mermaid
flowchart LR
    root["certificate-validate"]
    subgraph globalFlags["Global Flags"]
        cfg["-c, --config\n(caminho do YAML)"]
        logFile["--log-file\n(arquivo de log)"]
    end

    checkCmd["check"]
    serveCmd["serve"]
    exportCmd["export"]
    versionCmd["version"]
    completionCmd["completion"]

    root --> globalFlags
    root --> checkCmd
    root --> serveCmd
    root --> exportCmd
    root --> versionCmd
    root --> completionCmd

    subgraph checkFlags["check Flags"]
        cw["-w, --watch\n(modo contínuo)"]
        co["-o, --output\n(json ou table)"]
        ch["--host\n(host único)"]
        cp["--port\n(padrão: 443)"]
        cm["--min-days\n(filtro por dias)"]
    end
    checkCmd --> checkFlags

    subgraph serveFlags["serve Flags"]
        st["--tls-cert\n(certificado TLS)"]
        sk["--tls-key\n(chave TLS)"]
        sa["--api-key\n(chave de API)"]
    end
    serveCmd --> serveFlags

    subgraph exportFlags["export Flags"]
        ef["-f, --format\n(json ou csv)"]
        eo["-o, --output-file\n(arquivo de saída)"]
    end
    exportCmd --> exportFlags

    subgraph completionShells["completion Shells"]
        bash["bash"]
        zsh["zsh"]
        fish["fish"]
        pwsh["powershell"]
    end
    completionCmd --> completionShells
```

### Funcionamento de Cada Comando

| Comando | Descrição | Fluxo Principal |
|---|---|---|
| `check` | Checa certificados dos hosts configurados ou de um host único via `--host` | Carrega config → constrói `Checker` via `buildApp` → `CheckAll`/`Check` → filtra por `--min-days` → imprime JSON ou tabela |
| `check --watch` | Loop contínuo de checagem com intervalo configurável | `signal.NotifyContext` → loop com `time.Sleep(checkTime)` → checa e imprime → reinicia |
| `serve` | Inicia servidor HTTP/HTTPS com hot-reload via SIGHUP | Carrega config → `buildDeps` (injeta tudo) → mux com rotas + middleware → background services → aguarda SIGHUP ou SIGTERM |
| `export` | Checa todos os hosts e exporta em JSON ou CSV | Carrega config → `buildApp` → `CheckAll` → `FormatJSON`/`FormatCSV` → stdout ou arquivo |
| `version` | Exibe versão, commit, data de build e runtime Go | Imprime vars injetadas via `ldflags` |
| `completion` | Gera script de completão para shell | Delega para `cobra.Gen*Completion` |

---

## 3. Fluxo de Checagem de Certificado

```mermaid
sequenceDiagram
    participant User as Usuário
    participant CLI as check.go
    participant Cfg as config
    participant App as buildApp
    participant Checker as checker.Checker
    participant Fetcher as fetcher (TLS)
    participant Cert as certificate
    participant Revoc as revocation
    participant Fmt as formatter

    User->>CLI: certificate-validate check
    CLI->>Cfg: config.Load(cfgPath)
    Cfg-->>CLI: *Config
    CLI->>Cfg: cfg.Validate()
    Cfg-->>CLI: warnings, err

    CLI->>App: buildApp(cfg)
    App->>Fetcher: fetcher.NewWithPerHostCAs(timeout, rootCAs, perHostCAs)
    App->>Fmt: formatter.New()
    App->>Checker: checker.New(fetcher, formatter)
    App-->>CLI: *Checker

    CLI->>Checker: CheckAll(ctx, hosts, maxParallel=10)

    par Concurrente (semáforo)
        Checker->>Fetcher: Fetch(ctx, host, port)
        Fetcher->>Fetcher: tls.DialWithDialer (handshake TLS)
        Fetcher->>Cert: certificate.FromX509(certs[0], host, port)
        Fetcher->>Cert: certificate.TLSVersionString(cs.Version)
        Fetcher->>Cert: certificate.BuildChain(certs)
        Fetcher->>Revoc: revocation.Check(leaf, issuer, ocsp, crl)
        Revoc-->>Fetcher: RevocationStatus
        Fetcher-->>Checker: *Certificate, nil

        Checker-->>CLI: []*Certificate, []error
    end

    CLI->>CLI: filterByMinDays(certs)
    alt output == "table"
        CLI->>Fmt: formatter.FormatTable(certs)
        Fmt-->>CLI: []byte (tabela formatada)
    else output == "json"
        CLI->>CLI: json.MarshalIndent(cert)
    end
    CLI-->>User: resultado
```

### Etapas Detalhadas da Checagem

```mermaid
flowchart TB
    start(["certificate-validate check"]) --> loadConfig["config.Load(path)\nLê YAML + aplica env vars"]
    loadConfig --> validate["cfg.Validate()\nValida hosts, portas, webhook, prometheus"]
    validate --> buildApp["buildApp(cfg)"]

    subgraph buildAppFlow["buildApp"]
        direction TB
        l1["fetcher.LoadRootCAs(cfg.TrustedCAs)\nCarrega CAs confiáveis globais"]
        l2["config.LoadPerHostCAs(cfg.Hosts)\nCarrega CAs por host"]
        l3["fetcher.NewWithPerHostCAs(timeout, rootCAs, perHostCAs)\nCria fetcher TLS"]
        l4["formatter.New()\nCria formatador JSON"]
        l5["checker.New(fetcher, formatter)\nCria orquestrador"]
        l1 --> l2 --> l3 --> l5
        l4 --> l5
    end

    buildApp --> buildAppFlow
    buildAppFlow --> resolveHosts["config.ToCheckerHosts(cfg.Hosts)\nExpande portas em Host structs"]
    resolveHosts --> checkAll["app.CheckAll(ctx, hosts, 10)\nConcorrente com semáforo"]

    subgraph perHost["Para cada host"]
        tlsDial["tls.DialWithDialer\nHandshake TLS 1.2/1.3"]
        extract["FromX509(leaf, hostname, port)\nExtrai: CN, SAN, Issuer, Validade, Tipo"]
        tlsVersion["TLSVersionString(cs.Version)\nCipherSuiteName(cs.CipherSuite)"]
        chain["BuildChain(peerCerts)\nCadeia com fingerprints SHA256"]
        revocCheck["revocation.Check(leaf, issuer, OCSP, CRL)\nOCSP primeiro, CRL como fallback"]
        resultCert["*certificate.Certificate"]
        tlsDial --> extract --> tlsVersion --> chain --> revocCheck --> resultCert
    end

    checkAll --> perHost
    perHost --> filter["filterByMinDays(certs)\nRemove certificados com daysLeft > minDays"]

    filter --> print["printCerts(output, certs, errs)"]
    print --> format{"Formato?"}
    format -->|"json (padrão)"| printJSON["json.MarshalIndent(cada cert)"]
    format -->|"table"| printTable["formatter.FormatTable(certs)\nTabela alinhada com status"]
```

---

## 4. Fluxo de Requisição HTTP (API)

```mermaid
sequenceDiagram
    participant Client as Cliente HTTP
    participant Mux as http.ServeMux
    participant MW as Middleware Chain
    participant RL as Rate Limiter
    participant Auth as API Auth
    participant Handler as api.Handler
    participant Svc as service.CertService
    participant Checker as checker.Checker
    participant Hist as history.Store
    participant Metrics as metrics

    Client->>Mux: GET /api/v1/cert/info/all
    Mux->>MW: withMiddleware(h, mux)

    MW->>MW: Security Headers\nX-Content-Type-Options: nosniff\nX-Frame-Options: DENY

    alt API Key configurada && rota != /health
        MW->>Auth: r.Header.Get("X-API-Key") == h.apiToken?
        Auth-->>MW: match?
        alt Não coincide
            MW-->>Client: 401 {"error":"unauthorized"}
        end
    end

    MW->>RL: defaultLimiter.allow()?
    alt Taxa excedida
        RL-->>MW: false
        MW-->>Client: 429 {"error":"too many requests"} + Retry-After: 1
    end

    MW->>MW: slog.Info(method, path, remote)
    MW->>Handler: h.handleAll(w, r)

    Handler->>Svc: h.svc.CheckAll(ctx, h.cfg.Hosts)
    Svc->>Checker: s.checker.CheckAll(ctx, hosts, 10)
    Checker-->>Svc: []*Certificate, []error
    Svc->>Svc: Filtra nils
    alt metrics != nil
        Svc->>Metrics: s.metrics(certs)
        Metrics->>Metrics: setGauges(host, port, daysLeft)
    end
    alt recorder != nil
        Svc->>Hist: s.recorder.Record(certs)
        Hist->>Hist: Append JSONL + rotate
    end
    Svc-->>Handler: CheckResult{Certificates, Errors}

    Handler->>Handler: writeJSON(200, {certificates, errors})
    Handler-->>Client: 200 JSON
```

### Rotas da API

| Método | Rota | Handler | Descrição |
|---|---|---|---|
| GET | `/health` | `handleHealth` | Health check com ping TCP em cada host |
| GET | `/api/v1/cert/info/all` | `handleAll` | Certificados de todos os hosts |
| GET | `/api/v1/cert/info/{hostname}` | `handleByHostname` | Certificado de um host específico |
| GET | `/api/v1/cert/info/commonName` | `handleCommonName` | Mapa hostname → Common Name |
| GET | `/api/v1/cert/info/subjectAltName` | `handleSubjectAltName` | Mapa hostname → SANs |
| GET | `/api/v1/cert/export/json` | `handleExportJSON` | Download JSON de todos certificados |
| GET | `/api/v1/cert/export/csv` | `handleExportCSV` | Download CSV de todos certificados |
| GET | `/api/v1/cert/history/{hostname}` | `handleHistory` | Histórico de checagens de um host |
| GET | `/metrics` | `metrics.Handler()` | Métricas Prometheus (se habilitado) |
| GET | `/` | `http.FileServer(staticFS)` | Frontend estático embutido |

### Middleware Chain

```mermaid
flowchart LR
    Req["Requisição HTTP"] --> Headers["Security Headers\nX-Content-Type-Options\nX-Frame-Options"]

    Headers --> Auth{"apiToken configurada\n&& rota != /health?"}
    Auth -->|"Sim"| CheckKey{"X-API-Key\n== apiToken?"}
    CheckKey -->|"Não"| Resp401["401 Unauthorized"]
    CheckKey -->|"Sim"| RateLimit

    Auth -->|"Não"| RateLimit

    RateLimit{"Rate Limiter\n(token bucket)\n100 req/s, burst 200"}
    RateLimit -->|"Excedido"| Resp429["429 Too Many Requests\nRetry-After: 1"]
    RateLimit -->|"OK"| Log["slog.Info(method, path, remote)"]

    Log --> Route["Roteamento\nhttp.ServeMux"]
    Route --> Handler["Handler específico"]
```

### Rate Limiter (Token Bucket)

```mermaid
flowchart TB
    allow(["defaultLimiter.allow()"]) --> lock["h.mu.Lock()"]
    lock --> calc["elapsed = now - lastFill\ntokens += elapsed * rate\n(taxa: 100 tokens/s)"]
    calc --> cap{"tokens > burst?"}
    cap -->|"Sim"| capTokens["tokens = burst\n(burst: 200)"]
    cap -->|"Não"| checkTokens{"tokens >= 1?"}
    capTokens --> checkTokens
    checkTokens -->|"Sim"| consume["tokens--\nreturn true (allow)"]
    checkTokens -->|"Não"| returnFalse["return false (deny)"]
    consume --> unlock["h.mu.Unlock()"]
    returnFalse --> unlock
```

---

## 5. Serviços de Background

Quando o servidor `serve` está rodando, três serviços concorrentes podem operar em background:

```mermaid
flowchart TB
    Serve["certificate-validate serve"] --> startBackground["startBackground(ctx, cfg, deps)"]

    startBackground --> prom{"cfg.Prometheus.Enabled?"}
    prom -->|"Sim"| PromUpdater["metrics.StartUpdater(ctx, checker, hosts, interval)"]

    startBackground --> hist{"deps.registry != nil\n(history.Enabled?)"}
    hist -->|"Sim"| HistRecorder["history.StartRecorder(ctx, registry, checker, hosts, interval)"]

    startBackground --> webhook{"cfg.Webhook.URL != ''?"}
    webhook -->|"Sim"| Notifier["notifier.New(cfg, checker, hosts).Start(ctx)"]

    subgraph promLoop["Prometheus Updater - Loop a cada check_time"]
        direction TB
        p1["updateFromChecker:\nCheckAll com timeout 30s"]
        p2["Update(certs):\nsetGauges(host, port, daysLeft)"]
        p1 --> p2
    end

    subgraph histLoop["History Recorder - Loop a cada check_time"]
        direction TB
        h1["updateAndRecord:\nCheckAll com timeout 30s"]
        h2["r.Record(certs):\nAppend JSONL"]
        h3["r.rotate():\nRemove entradas > maxDays\ntruncate para maxEntries"]
        h1 --> h2 --> h3
    end

    subgraph webhookLoop["Webhook Notifier - Loop a cada webhook.interval"]
        direction TB
        w1["checkAndAlert:\nPara cada host:"]
        w2["checker.Check(host)"]
        w3{"daysLeft <= threshold?"}
        w4{"já alertou\nneste interval?"}
        w5["sendAlert:\nPOST JSON para webhook.URL"]
        w6["atualiza lastAlerted[key]"]
        w1 --> w2 --> w3
        w3 -->|"Sim"| w4
        w3 -->|"Não"| next["próximo host"]
        w4 -->|"Não"| w5 --> w6
    end

    PromUpdater --> promLoop
    HistRecorder --> histLoop
    Notifier --> webhookLoop

    subgraph reload["Hot-Reload (SIGHUP)"]
        direction TB
        sig["Sinal SIGHUP recebido"] --> bgCancel["bgCancel()\n(para loops antigos)"]
        bgCancel --> loadNew["config.Load(cfgPath)"]
        loadNew --> valNew["cfg.Validate()"]
        valNew --> rebuild["buildDeps(newCfg)"]
        rebuild --> store["currentHandler.Store(newHandler)"]
        store --> restart["restartBackground()\n(começa novos loops)"]
    end
```

### Detalhamento dos Serviços

| Serviço | Ativação | Intervalo | Função |
|---|---|---|---|
| **Prometheus Updater** | `prometheus.enabled: true` | `check_time` | Checa todos os hosts e atualiza gauges `certificate_days_left` e `certificate_expired` |
| **History Recorder** | `history.enabled: true` | `check_time` | Checa todos os hosts e registra entrada JSONL com rotação automática (max_dias + max_entries) |
| **Webhook Notifier** | `webhook.url` definido | `webhook.interval` | Checa cada host individualmente e envia alerta POST se `daysLeft <= threshold`, com rate limiting próprio |

---

## 6. Pipeline de Carregamento de Configuração

```mermaid
flowchart TB
    Start["config.Load(cfgPath)"] --> read["os.ReadFile(path)"]
    read --> yaml["yaml.Unmarshal(data, &cfg)"]

    yaml --> defaults["Defaults:\ncheck_time = 86400 (se <= 0)"]

    defaults --> env["cfg.applyEnvOverrides()"]

    subgraph EnvOverrides["Variáveis de Ambiente (prefixo CV_)"]
        direction TB
        e1["CV_CHECK_TIME"]
        e2["CV_API_KEY"]
        e3["CV_APP_HOST"]
        e4["CV_APP_PORT"]
        e5["CV_PROMETHEUS_ENABLED"]
        e6["CV_PROMETHEUS_ADDRESS"]
        e7["CV_WEBHOOK_URL"]
        e8["CV_WEBHOOK_THRESHOLD"]
        e9["CV_WEBHOOK_INTERVAL"]
        e10["CV_HISTORY_ENABLED"]
        e11["CV_HISTORY_FILE_PATH"]
        e12["CV_HISTORY_MAX_ENTRIES"]
        e13["CV_HISTORY_MAX_DAYS"]
        e14["CV_TRUSTED_CAS"]
    end

    env --> EnvOverrides
    EnvOverrides --> cfgValido["*Config (pronto para uso)"]

    cfgValido --> validate["cfg.Validate()"]
    validate --> checks{"Validações:"}

    subgraph validations["Validações"]
        v1["hosts vazio → erro"]
        v2["host.url vazio → erro"]
        v3["host.name vazio → warning"]
        v4["host.port inválido → warning"]
        v5["host.ports fora 1-65535 → warning"]
        v6["host.timeout negativo → warning"]
        v7["webhook.threshold <= 0 → warning"]
        v8["prometheus sem address → warning"]
    end

    checks --> validations
    validations --> result["(*Config, warnings, err)"]
```

### Estrutura do Config

```yaml
check_time: 30                    # Intervalo de checagem em segundos
api_key: "..."                    # Chave de API para autenticação

app_configs:                      # Configuração do servidor HTTP
  - host: '0.0.0.0'
    port: '5000'
    environment: 'production'
    debug: false

hosts:                            # Hosts para checar certificados
  - name: "GitHub"
    url: 'github.com'
    port: '443'
    ports: [443]                  # Múltiplas portas (alternativa a port)
    timeout: 10                   # Timeout específico por host (segundos)
    trusted_cas:                  # CAs específicas por host
      - '/certs/internal-ca.pem'

prometheus:                       # Métricas Prometheus
  enabled: false
  address: ':9090'

webhook:                          # Alertas via webhook
  url: 'https://hooks.example.com/alert'
  threshold: 15                   # Dias mínimos para alertar
  interval: 1800                  # Intervalo entre checagens (segundos)

history:                          # Histórico local (JSONL)
  enabled: true
  file_path: "data/history.jsonl"
  max_entries: 10000
  max_days: 90

trusted_cas:                      # CAs confiáveis globais
  - '/etc/certs/my-ca.pem'
```

---

## 7. Injeção de Dependências e Inicialização

### `serve` — Inicialização Completa

```mermaid
flowchart TB
    serveCmd["serve.RunE"] --> load["config.Load(cfgPath)"]
    load --> val["cfg.Validate()"]
    val --> build["buildDeps(cfg)"]

    subgraph buildDeps["buildDeps(cfg)"]
        direction TB
        b1["fetcher.LoadRootCAs(cfg.TrustedCAs)"]
        b2["config.LoadPerHostCAs(cfg.Hosts)"]
        b3["fetcher.NewWithPerHostCAs(10s, rootCAs, perHostCAs)"]
        b4["formatter.New()"]
        b5["checker.New(fetcher, formatter)"]
        b6{"history.Enabled?"}
        b7["history.New(cfg.History)"]
        b8{"prometheus.Enabled?"}
        b9["metrics.Update (fn)"]
        b10["service.NewCertService(checker, recorder, metrics)"]
        b11{"apiKeyFlag != ''?"}
        b12["usa apiKeyFlag"]
        b13["usa cfg.APIKey"]
        b14["api.New(svc, cfg, apiToken)"]
        b15["h.Router()"]

        b1 --> b3
        b2 --> b3
        b3 --> b5
        b4 --> b5
        b5 --> b10
        b6 -->|"Sim"| b7 --> b10
        b6 -->|"Não"| b10
        b8 -->|"Sim"| b9 --> b10
        b8 -->|"Não"| b10
        b10 --> b14
        b11 -->|"Sim"| b12 --> b14
        b11 -->|"Não"| b13 --> b14
        b14 --> b15
    end

    build --> mux["http.ServeMux + withMiddleware"]
    mux --> addr["fmt.Sprintf('%s:%s', host, port)"]
    addr --> server["http.Server{Handler, ReadTimeout, WriteTimeout, IdleTimeout}"]

    server --> bg["restartBackground()"]
    bg --> startBG["startBackground(ctx, cfg, deps)"]

    startBG --> services["Prometheus / History / Webhook\n(ver seção 5)"]

    services --> listen{"TLS configurado?"}
    listen -->|"Sim"| tls["server.ListenAndServeTLS()"]
    listen -->|"Não"| http["server.ListenAndServe()"]

    tls --> loop["Loop principal:\nselect { SIGTERM → shutdown | SIGHUP → reload }"]
    http --> loop
```

### `check` — Inicialização Simplificada

```mermaid
flowchart TB
    checkCmd["check"] --> host{"--host definido?"}
    host -->|"Sim"| single["runCheckHost(cmd, host, port)"]
    host -->|"Não"| load["config.Load(cfgPath)"]

    single --> fetcher1["fetcher.New(10s)"]
    fetcher1 --> formatter1["formatter.New()"]
    formatter1 --> checker1["checker.New(fetcher, formatter)"]

    checker1 --> check1["checker.Check(ctx, host, port)"]
    check1 --> filter1{"minDays > 0 && daysLeft > minDays?"}
    filter1 -->|"Sim"| exit["return nil (sem output)"]
    filter1 -->|"Não"| output1{"output == 'table'?"}
    output1 -->|"Sim"| table1["formatter.FormatTable"]
    output1 -->|"Não"| json1["json.MarshalIndent"]

    load --> val["cfg.Validate()"]
    val --> app["buildApp(cfg)"]
    app --> hosts["config.ToCheckerHosts(cfg.Hosts)"]
    hosts --> watch{"--watch?"}

    watch -->|"Não"| checkAll["app.CheckAll(ctx, hosts, 10)"]
    checkAll --> filterAll["filterByMinDays(certs)"]
    filterAll --> print["printCerts(certs, errs)"]
    print --> output2{"output == 'table'?"}
    output2 -->|"Sim"| table2["formatter.FormatTable"]
    output2 -->|"Não"| json2["json.MarshalIndent"]

    watch -->|"Sim"| webhook{"webhook.URL configurado?"}
    webhook -->|"Sim"| notifier["notifier.New(cfg, app, hosts).Start(ctx)"]
    webhook -->|"Não"| loopWatch["runWatchLoop(ctx, app, hosts, interval)"]
    notifier --> loopWatch
    loopWatch --> loopBody["loop:\napp.CheckAll(ctx, hosts, 0)\nfilterByMinDays\njson.MarshalIndent\nsleep(checkTime)"]
```

---

## 8. Exportação de Dados

```mermaid
flowchart TB
    subgraph CLI["Export via CLI"]
        cli["certificate-validate export"] --> loadCfg["config.Load(cfgPath)"]
        loadCfg --> buildApp["buildApp(cfg)"]
        buildApp --> checkAll["app.CheckAll(ctx, hosts, 10)"]
        checkAll --> formatChoice{"Formato?"}
        formatChoice -->|"json (padrão)"| fmtJSON["formatter.FormatJSON(certs)\n[]byte JSON array"]
        formatChoice -->|"csv"| fmtCSV["formatter.FormatCSV(certs)\n[]byte CSV com header"]
        fmtJSON --> output{"--output-file?"}
        fmtCSV --> output
        output -->|"Sim"| file["os.WriteFile(path, data, 0644)"]
        output -->|"Não"| stdout["fmt.Println(string(data))"]
    end

    subgraph API["Export via API"]
        apiJson["GET /api/v1/cert/export/json"] --> svcAll["svc.CheckAll(ctx, hosts)"]
        svcAll --> jsonResp["writeJSON(200, {certificates, errors})\nContent-Disposition: attachment"]
        svcAll --> csvHeader["Content-Type: text/csv\nBOM UTF-8\nContent-Disposition: attachment"]
        csvHeader --> csvWrite["csv.NewWriter:\nescreve header + linhas"]
    end

    subgraph Formats["Formatos de Saída"]
        direction TB
        f1["FormatJSON:\njson.MarshalIndent(certs, '', '  ')"]
        f2["FormatCSV:\ncsv.Writer com header:\nhostname,port,commonName,issuer,\nnotBefore,notAfter,daysLeft,\nrevocationStatus,tlsVersion,cipherSuite"]
        f3["FormatTable (CLI only):\nTabela com colunas:\nHost, Port, Days, Status, Revoc,\nIssuer, TLS Version"]
        f4["JSON individual:\njson.MarshalIndent(cert, '', '  ')"]
    end

    Formats --> f1
    Formats --> f2
    Formats --> f3
    Formats --> f4
```

### Diferenças de Formatação

| Formato | Onde | Saída |
|---|---|---|
| `FormatJSON` | CLI `export -f json` | `[{...}, {...}]` — array de certificados |
| `FormatCSV` | CLI `export -f csv` | CSV com header + 10 colunas |
| `FormatTable` | CLI `check -o table` | Tabela alinhada com status colorido |
| JSON individual | CLI `check` (padrão) | Um JSON object por certificado |

---

## 9. Checagem de Revogação

```mermaid
flowchart TB
    revocation["revocation.Check(leaf, issuer, ocspServers, crlURLs)"]

    revocation --> hasOCSP{"Possui servidores\nOCSP?"}
    hasOCSP -->|"Sim"| ocsp["CheckOCSP(leaf, issuer, servers)"]

    ocsp --> ocspLoop["Para cada servidor OCSP:"]
    ocspLoop --> createReq["ocsp.CreateRequest(leaf, issuer)"]
    createReq --> post["POST application/ocsp-request\n(10s timeout)"]

    post --> parseResp["ocsp.ParseResponse(bytes, issuer)"]
    parseResp --> ocspStatus{"Status OCSP:"}
    ocspStatus -->|"Good"| good["RevocationGood"]
    ocspStatus -->|"Revoked"| revoked["RevocationRevoked"]
    ocspStatus -->|"Unknown"| tryNext["Tenta próximo servidor\n(ou retorna Unknown)"]

    hasOCSP -->|"Não"| crl["CheckCRL(leaf, crlURLs)"]

    crl --> crlLoop["Para cada URL CRL:"]
    crlLoop --> download["GET (10s timeout)"]
    download --> parseCRL["x509.ParseRevocationList(bytes)"]

    parseCRL --> search{"SerialNumber\nestá na lista de\nrevogados?"}
    search -->|"Sim"| crlRevoked["RevocationRevoked"]
    search -->|"Não"| crlGood["RevocationGood"]

    good --> statusFinal["Status Final"]
    revoked --> statusFinal
    crlRevoked --> statusFinal
    crlGood --> statusFinal

    subgraph fallback["Fallback"]
        direction LR
        notReady["Nenhum servidor OCSP\nnem URL CRL disponível"]
        notReady --> resultNotReady["RevocationNotReady"]
    end

    hasOCSP -->|"Não"| fallback
    crl -->|sem CRLs| fallback
    fallback --> statusFinal
    statusFinal --> log["LogRevocation(cert, status)\nWarn se revogado"]
```

### Prioridade da Checagem

1. **OCSP** (Online Certificate Status Protocol) — tentativa primária, mais rápida e precisa
2. **CRL** (Certificate Revocation List) — fallback quando OCSP não está disponível ou retorna `NotReady`
3. **NotReady** — quando não há servidores OCSP nem URLs CRL no certificado

---

## Modelo de Dados — Certificate

```mermaid
classDiagram
    class Certificate {
        +string CommonName
        +[]string SubjectAltNames
        +string Issuer
        +string Type
        +string NotBefore
        +string NotAfter
        +int DaysLeft
        +[]string CRLDistributionPoints
        +[]string OCSPServer
        +RevocationStatus RevocationStatus
        +string Hostname
        +int Port
        +string TLSVersion
        +string CipherSuite
        +[]ChainEntry Chain
    }

    class ChainEntry {
        +string Subject
        +string Issuer
        +string NotAfter
        +string Fingerprint
    }

    class RevocationStatus {
        <<enumeration>>
        unknown
        good
        revoked
        not_ready
    }

    Certificate "1" --> "*" ChainEntry
    Certificate --> RevocationStatus
```

---

## Estrutura de Diretórios

```
certificate-validate/
├── cmd/certificate-validate/
│   └── main.go                    # Entry point
├── config/
│   └── settings.yml               # Configuração YAML
├── docs/
│   ├── swagger.yaml               # OpenAPI 3.0
│   └── ARCHITECTURE.md            # Este documento
├── internal/
│   ├── api/                       # Interface HTTP
│   │   ├── api.go                 # Handlers + middleware + rate limiter
│   │   └── static/                # Frontend embutido (embed)
│   ├── certificate/               # Domínio
│   │   ├── certificate.go         # VO + FromX509 + BuildChain
│   │   └── errors.go              # Erros de domínio
│   ├── checker/                   # Caso de uso
│   │   └── checker.go             # Checker (orquestração)
│   ├── cmd/                       # CLI (Cobra)
│   │   ├── root.go                # Comando raiz + flags globais
│   │   ├── check.go               # check + watch
│   │   ├── serve.go               # serve + buildDeps + startBackground
│   │   ├── export.go              # export JSON/CSV
│   │   └── version.go             # version
│   ├── config/                    # Configuração
│   │   └── config.go              # Load + Validate + applyEnvOverrides
│   ├── fetcher/                   # Provedor TLS
│   │   └── fetcher.go             # Fetcher interface + tlsFetcher
│   ├── formatter/                 # Provedor de formatação
│   │   └── formatter.go           # Formatter, FormatTable, FormatJSON, FormatCSV
│   ├── history/                   # Histórico
│   │   └── history.go             # Store, Recorder, rotacao JSONL
│   ├── metrics/                   # Métricas Prometheus
│   │   └── metrics.go             # Gauges + Handler
│   ├── notifier/                  # Webhook
│   │   └── notifier.go            # Notifier + alerta
│   ├── revocation/                # Revogação
│   │   └── revocation.go          # OCSP + CRL checks
│   └── service/                   # Serviço (facade)
│       └── service.go             # CertService
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```
