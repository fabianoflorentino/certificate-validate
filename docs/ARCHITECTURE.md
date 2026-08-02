# Architecture

This document describes the architecture, flows, and components of **certificate-validate**,
a modern, extensible SSL/TLS validation tool written in Go.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [CLI Command Hierarchy](#2-cli-command-hierarchy)
3. [Certificate Check Flow](#3-certificate-check-flow)
4. [HTTP Request Flow (API)](#4-http-request-flow-api)
5. [Background Services](#5-background-services)
6. [Configuration Loading Pipeline](#6-configuration-loading-pipeline)
7. [Dependency Injection and Initialization](#7-dependency-injection-and-initialization)
8. [Data Export](#8-data-export)
9. [Revocation Check](#9-revocation-check)

---

## 1. Architecture Overview

The project follows **Clean Architecture** with **SOLID** principles,
separating the code into concentric layers:

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
        checkerLayer["internal/checker/\n(Checker: orchestrates fetch + format)"]
        serviceLayer["internal/service/\n(CertService: CheckAll, CheckSingle, GetHistory)"]
    end

    subgraph Domain["DOMAIN - S, D"]
        certLayer["internal/certificate/\n(VO Certificate, FromX509, BuildChain, TLSVersionString)"]
    end

    subgraph Providers["PROVIDERS - D, I"]
        fetcherLayer["internal/fetcher/\n(interface + TLS implementation)"]
        formatterLayer["internal/formatter/\n(JSONFormatter, FormatTable, FormatJSON, FormatCSV)"]
        notifierLayer["internal/notifier/\n(webhook alerts)"]
        historyLayer["internal/history/\n(Recorder, Store, JSONL rotation)"]
        metricsLayer["internal/metrics/\n(Prometheus gauges)"]
        revocationLayer["internal/revocation/\n(OCSP + CRL checks)"]
    end

    subgraph Config["CONFIG"]
        configLayer["internal/config/\n(YAML loader + env var overrides + validation)"]
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

### SOLID Principles Applied

| Principle | Implementation |
|---|---|
| **S** - Single Responsibility | Each package has a single responsibility: `certificate` = domain, `fetcher` = TLS, `formatter` = output, `checker` = orchestration |
| **O** - Open/Closed | `Fetcher` and `Formatter` interfaces allow new implementations without modifying existing code |
| **L** - Liskov Substitution | Explicit `(Certificate, error)` returns — no `sys.Exit()` or inconsistent types |
| **I** - Interface Segregation | Minimal interfaces: `Fetcher` has 1 method, `Formatter` has 1 method |
| **D** - Dependency Inversion | `checker` defines interfaces, providers implement them. `main.go` injects dependencies |

---

## 2. CLI Command Hierarchy

```mermaid
flowchart LR
    root["certificate-validate"]
    subgraph globalFlags["Global Flags"]
        cfg["-c, --config\n(YAML path)"]
        logFile["--log-file\n(log file)"]
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
        cw["-w, --watch\n(continuous mode)"]
        co["-o, --output\n(json or table)"]
        ch["--host\n(single host)"]
        cp["--port\n(default: 443)"]
        cm["--min-days\n(filter by days)"]
    end
    checkCmd --> checkFlags

    subgraph serveFlags["serve Flags"]
        st["--tls-cert\n(TLS certificate)"]
        sk["--tls-key\n(TLS key)"]
        sa["--api-key\n(API key)"]
    end
    serveCmd --> serveFlags

    subgraph exportFlags["export Flags"]
        ef["-f, --format\n(json or csv)"]
        eo["-o, --output-file\n(output file)"]
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

### How Each Command Works

| Command | Description | Main Flow |
|---|---|---|
| `check` | Checks certificates from configured hosts or a single host via `--host` | Loads config → builds `Checker` via `buildApp` → `CheckAll`/`Check` → filters by `--min-days` → prints JSON or table |
| `check --watch` | Continuous check loop with configurable interval | `signal.NotifyContext` → loop with `time.Sleep(checkTime)` → checks and prints → restarts |
| `serve` | Starts HTTP/HTTPS server with hot-reload via SIGHUP | Loads config → `buildDeps` (injects everything) → mux with routes + middleware → background services → waits for SIGHUP or SIGTERM |
| `export` | Checks all hosts and exports in JSON or CSV | Loads config → `buildApp` → `CheckAll` → `FormatJSON`/`FormatCSV` → stdout or file |
| `version` | Shows version, commit, build date, and Go runtime | Prints variables injected via `ldflags` |
| `completion` | Generates shell completion script | Delegates to `cobra.Gen*Completion` |

---

## 3. Certificate Check Flow

```mermaid
sequenceDiagram
    participant User as User
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

    par Concurrent (semaphore)
        Checker->>Fetcher: Fetch(ctx, host, port)
        Fetcher->>Fetcher: tls.DialWithDialer (TLS handshake)
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
        Fmt-->>CLI: []byte (formatted table)
    else output == "json"
        CLI->>CLI: json.MarshalIndent(cert)
    end
    CLI-->>User: result
```

### Detailed Check Steps

```mermaid
flowchart TB
    start(["certificate-validate check"]) --> loadConfig["config.Load(path)\nReads YAML + applies env vars"]
    loadConfig --> validate["cfg.Validate()\nValidates hosts, ports, webhook, prometheus"]
    validate --> buildApp["buildApp(cfg)"]

    subgraph buildAppFlow["buildApp"]
        direction TB
        l1["fetcher.LoadRootCAs(cfg.TrustedCAs)\nLoads trusted global CAs"]
        l2["config.LoadPerHostCAs(cfg.Hosts)\nLoads per-host CAs"]
        l3["fetcher.NewWithPerHostCAs(timeout, rootCAs, perHostCAs)\nCreates TLS fetcher"]
        l4["formatter.New()\nCreates JSON formatter"]
        l5["checker.New(fetcher, formatter)\nCreates orchestrator"]
        l1 --> l2 --> l3 --> l5
        l4 --> l5
    end

    buildApp --> buildAppFlow
    buildAppFlow --> resolveHosts["config.ToCheckerHosts(cfg.Hosts)\nExpands ports into Host structs"]
    resolveHosts --> checkAll["app.CheckAll(ctx, hosts, 10)\nConcurrent with semaphore"]

    subgraph perHost["For each host"]
        tlsDial["tls.DialWithDialer\nTLS 1.2/1.3 handshake"]
        extract["FromX509(leaf, hostname, port)\nExtracts: CN, SAN, Issuer, Validity, Type"]
        tlsVersion["TLSVersionString(cs.Version)\nCipherSuiteName(cs.CipherSuite)"]
        chain["BuildChain(peerCerts)\nChain with SHA256 fingerprints"]
        revocCheck["revocation.Check(leaf, issuer, OCSP, CRL)\nOCSP first, CRL as fallback"]
        resultCert["*certificate.Certificate"]
        tlsDial --> extract --> tlsVersion --> chain --> revocCheck --> resultCert
    end

    checkAll --> perHost
    perHost --> filter["filterByMinDays(certs)\nRemoves certificates with daysLeft > minDays"]

    filter --> print["printCerts(output, certs, errs)"]
    print --> format{"Format?"}
    format -->|"json (default)"| printJSON["json.MarshalIndent(each cert)"]
    format -->|"table"| printTable["formatter.FormatTable(certs)\nAligned table with status"]
```

---

## 4. HTTP Request Flow (API)

```mermaid
sequenceDiagram
    participant Client as HTTP Client
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

    alt API key configured && route != /health
        MW->>Auth: r.Header.Get("X-API-Key") == h.apiToken?
        Auth-->>MW: match?
        alt No match
            MW-->>Client: 401 {"error":"unauthorized"}
        end
    end

    MW->>RL: defaultLimiter.allow()?
    alt Rate exceeded
        RL-->>MW: false
        MW-->>Client: 429 {"error":"too many requests"} + Retry-After: 1
    end

    MW->>MW: slog.Info(method, path, remote)
    MW->>Handler: h.handleAll(w, r)

    Handler->>Svc: h.svc.CheckAll(ctx, h.cfg.Hosts)
    Svc->>Checker: s.checker.CheckAll(ctx, hosts, 10)
    Checker-->>Svc: []*Certificate, []error
    Svc->>Svc: Filters nils
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

### API Routes

| Method | Route | Handler | Description |
|---|---|---|---|
| GET | `/health` | `handleHealth` | Health check with TCP ping to each host |
| GET | `/api/v1/cert/info/all` | `handleAll` | Certificates from all hosts |
| GET | `/api/v1/cert/info/{hostname}` | `handleByHostname` | Certificate for a specific host |
| GET | `/api/v1/cert/info/commonName` | `handleCommonName` | Map hostname → Common Name |
| GET | `/api/v1/cert/info/subjectAltName` | `handleSubjectAltName` | Map hostname → SANs |
| GET | `/api/v1/cert/export/json` | `handleExportJSON` | Download JSON of all certificates |
| GET | `/api/v1/cert/export/csv` | `handleExportCSV` | Download CSV of all certificates |
| GET | `/api/v1/cert/history/{hostname}` | `handleHistory` | Check history for a host |
| GET | `/metrics` | `metrics.Handler()` | Prometheus metrics (if enabled) |
| GET | `/` | `http.FileServer(staticFS)` | Embedded static frontend |

### Middleware Chain

```mermaid
flowchart LR
    Req["HTTP Request"] --> Headers["Security Headers\nX-Content-Type-Options\nX-Frame-Options"]

    Headers --> Auth{"apiToken configured\n&& route != /health?"}
    Auth -->|"No"| CheckKey{"X-API-Key\n== apiToken?"}
    CheckKey -->|"No"| Resp401["401 Unauthorized"]
    CheckKey -->|"Yes"| RateLimit

    Auth -->|"No"| RateLimit

    RateLimit{"Rate Limiter\n(token bucket)\n100 req/s, burst 200"}
    RateLimit -->|"Exceeded"| Resp429["429 Too Many Requests\nRetry-After: 1"]
    RateLimit -->|"OK"| Log["slog.Info(method, path, remote)"]

    Log --> Route["Routing\nhttp.ServeMux"]
    Route --> Handler["Specific handler"]
```

### Rate Limiter (Token Bucket)

```mermaid
flowchart TB
    allow(["defaultLimiter.allow()"]) --> lock["h.mu.Lock()"]
    lock --> calc["elapsed = now - lastFill\ntokens += elapsed * rate\n(rate: 100 tokens/s)"]
    calc --> cap{"tokens > burst?"}
    cap -->|"Yes"| capTokens["tokens = burst\n(burst: 200)"]
    cap -->|"No"| checkTokens{"tokens >= 1?"}
    capTokens --> checkTokens
    checkTokens -->|"Yes"| consume["tokens--\nreturn true (allow)"]
    checkTokens -->|"No"| returnFalse["return false (deny)"]
    consume --> unlock["h.mu.Unlock()"]
    returnFalse --> unlock
```

---

## 5. Background Services

When the `serve` server is running, three concurrent services may operate in the background:

```mermaid
flowchart TB
    Serve["certificate-validate serve"] --> startBackground["startBackground(ctx, cfg, deps)"]

    startBackground --> prom{"cfg.Prometheus.Enabled?"}
    prom -->|"Yes"| PromUpdater["metrics.StartUpdater(ctx, checker, hosts, interval)"]

    startBackground --> hist{"deps.registry != nil\n(history.Enabled?)"}
    hist -->|"Yes"| HistRecorder["history.StartRecorder(ctx, registry, checker, hosts, interval)"]

    startBackground --> webhook{"cfg.Webhook.URL != ''?"}
    webhook -->|"Yes"| Notifier["notifier.New(cfg, checker, hosts).Start(ctx)"]

    subgraph promLoop["Prometheus Updater - Loop every check_time"]
        direction TB
        p1["updateFromChecker:\nCheckAll with 30s timeout"]
        p2["Update(certs):\nsetGauges(host, port, daysLeft)"]
        p1 --> p2
    end

    subgraph histLoop["History Recorder - Loop every check_time"]
        direction TB
        h1["updateAndRecord:\nCheckAll with 30s timeout"]
        h2["r.Record(certs):\nAppend JSONL"]
        h3["r.rotate():\nRemoves entries > maxDays\ntruncates to maxEntries"]
        h1 --> h2 --> h3
    end

    subgraph webhookLoop["Webhook Notifier - Loop every webhook.interval"]
        direction TB
        w1["checkAndAlert:\nFor each host:"]
        w2["checker.Check(host)"]
        w3{"daysLeft <= threshold?"}
        w4{"already alerted\nin this interval?"}
        w5["sendAlert:\nPOST JSON to webhook.URL"]
        w6["updates lastAlerted[key]"]
        w1 --> w2 --> w3
        w3 -->|"Yes"| w4
        w3 -->|"No"| next["next host"]
        w4 -->|"No"| w5 --> w6
    end

    PromUpdater --> promLoop
    HistRecorder --> histLoop
    Notifier --> webhookLoop

    subgraph reload["Hot-Reload (SIGHUP)"]
        direction TB
        sig["SIGHUP signal received"] --> bgCancel["bgCancel()\n(stops old loops)"]
        bgCancel --> loadNew["config.Load(cfgPath)"]
        loadNew --> valNew["cfg.Validate()"]
        valNew --> rebuild["buildDeps(newCfg)"]
        rebuild --> store["currentHandler.Store(newHandler)"]
        store --> restart["restartBackground()\n(starts new loops)"]
    end
```

### Service Details

| Service | Activation | Interval | Function |
|---|---|---|---|
| **Prometheus Updater** | `prometheus.enabled: true` | `check_time` | Checks all hosts and updates the `certificate_days_left` and `certificate_expired` gauges |
| **History Recorder** | `history.enabled: true` | `check_time` | Checks all hosts and records a JSONL entry with automatic rotation (max_days + max_entries) |
| **Webhook Notifier** | `webhook.url` set | `webhook.interval` | Checks each host individually and sends a POST alert if `daysLeft <= threshold`, with its own rate limiting |

---

## 6. Configuration Loading Pipeline

```mermaid
flowchart TB
    Start["config.Load(cfgPath)"] --> read["os.ReadFile(path)"]
    read --> yaml["yaml.Unmarshal(data, &cfg)"]

    yaml --> defaults["Defaults:\ncheck_time = 86400 (if <= 0)"]

    defaults --> env["cfg.applyEnvOverrides()"]

    subgraph EnvOverrides["Environment Variables (CV_ prefix)"]
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
    EnvOverrides --> cfgValido["*Config (ready to use)"]

    cfgValido --> validate["cfg.Validate()"]
    validate --> checks{"Validations:"}

    subgraph validations["Validations"]
        v1["empty hosts → error"]
        v2["empty host.url → error"]
        v3["empty host.name → warning"]
        v4["invalid host.port → warning"]
        v5["host.ports outside 1-65535 → warning"]
        v6["negative host.timeout → warning"]
        v7["webhook.threshold <= 0 → warning"]
        v8["prometheus without address → warning"]
    end

    checks --> validations
    validations --> result["(*Config, warnings, err)"]
```

### Config Structure

```yaml
check_time: 30                    # Check interval in seconds
api_key: "..."                    # API key for authentication

app_configs:                      # HTTP server configuration
  - host: '0.0.0.0'
    port: '5000'
    environment: 'production'
    debug: false

hosts:                            # Hosts to check certificates
  - name: "GitHub"
    url: 'github.com'
    port: '443'
    ports: [443]                  # Multiple ports (alternative to port)
    timeout: 10                   # Per-host timeout (seconds)
    trusted_cas:                  # Per-host CAs
      - '/certs/internal-ca.pem'

prometheus:                       # Prometheus metrics
  enabled: false
  address: ':9090'

webhook:                          # Webhook alerts
  url: 'https://hooks.example.com/alert'
  threshold: 15                   # Minimum days to alert
  interval: 1800                  # Interval between checks (seconds)

history:                          # Local history (JSONL)
  enabled: true
  file_path: "data/history.jsonl"
  max_entries: 10000
  max_days: 90

trusted_cas:                      # Trusted global CAs
  - '/etc/certs/my-ca.pem'
```

---

## 7. Dependency Injection and Initialization

### `serve` — Full Initialization

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
        b12["uses apiKeyFlag"]
        b13["uses cfg.APIKey"]
        b14["api.New(svc, cfg, apiToken)"]
        b15["h.Router()"]

        b1 --> b3
        b2 --> b3
        b3 --> b5
        b4 --> b5
        b5 --> b10
        b6 -->|"Yes"| b7 --> b10
        b6 -->|"No"| b10
        b8 -->|"Yes"| b9 --> b10
        b8 -->|"No"| b10
        b10 --> b14
        b11 -->|"Yes"| b12 --> b14
        b11 -->|"No"| b13 --> b14
        b14 --> b15
    end

    build --> mux["http.ServeMux + withMiddleware"]
    mux --> addr["fmt.Sprintf('%s:%s', host, port)"]
    addr --> server["http.Server{Handler, ReadTimeout, WriteTimeout, IdleTimeout}"]

    server --> bg["restartBackground()"]
    bg --> startBG["startBackground(ctx, cfg, deps)"]

    startBG --> services["Prometheus / History / Webhook\n(see section 5)"]

    services --> listen{"TLS configured?"}
    listen -->|"Yes"| tls["server.ListenAndServeTLS()"]
    listen -->|"No"| http["server.ListenAndServe()"]

    tls --> loop["Main loop:\nselect { SIGTERM → shutdown | SIGHUP → reload }"]
    http --> loop
```

### `check` — Simplified Initialization

```mermaid
flowchart TB
    checkCmd["check"] --> host{"--host set?"}
    host -->|"Yes"| single["runCheckHost(cmd, host, port)"]
    host -->|"No"| load["config.Load(cfgPath)"]

    single --> fetcher1["fetcher.New(10s)"]
    fetcher1 --> formatter1["formatter.New()"]
    formatter1 --> checker1["checker.New(fetcher, formatter)"]

    checker1 --> check1["checker.Check(ctx, host, port)"]
    check1 --> filter1{"minDays > 0 && daysLeft > minDays?"}
    filter1 -->|"Yes"| exit["return nil (no output)"]
    filter1 -->|"No"| output1{"output == 'table'?"}
    output1 -->|"Yes"| table1["formatter.FormatTable"]
    output1 -->|"No"| json1["json.MarshalIndent"]

    load --> val["cfg.Validate()"]
    val --> app["buildApp(cfg)"]
    app --> hosts["config.ToCheckerHosts(cfg.Hosts)"]
    hosts --> watch{"--watch?"}

    watch -->|"No"| checkAll["app.CheckAll(ctx, hosts, 10)"]
    checkAll --> filterAll["filterByMinDays(certs)"]
    filterAll --> print["printCerts(certs, errs)"]
    print --> output2{"output == 'table'?"}
    output2 -->|"Yes"| table2["formatter.FormatTable"]
    output2 -->|"No"| json2["json.MarshalIndent"]

    watch -->|"Yes"| webhook{"webhook.URL configured?"}
    webhook -->|"Yes"| notifier["notifier.New(cfg, app, hosts).Start(ctx)"]
    webhook -->|"No"| loopWatch["runWatchLoop(ctx, app, hosts, interval)"]
    notifier --> loopWatch
    loopWatch --> loopBody["loop:\napp.CheckAll(ctx, hosts, 0)\nfilterByMinDays\njson.MarshalIndent\nsleep(checkTime)"]
```

---

## 8. Data Export

```mermaid
flowchart TB
    subgraph CLI["Export via CLI"]
        cli["certificate-validate export"] --> loadCfg["config.Load(cfgPath)"]
        loadCfg --> buildApp["buildApp(cfg)"]
        buildApp --> checkAll["app.CheckAll(ctx, hosts, 10)"]
        checkAll --> formatChoice{"Format?"}
        formatChoice -->|"json (default)"| fmtJSON["formatter.FormatJSON(certs)\n[]byte JSON array"]
        formatChoice -->|"csv"| fmtCSV["formatter.FormatCSV(certs)\n[]byte CSV with header"]
        fmtJSON --> output{"--output-file?"}
        fmtCSV --> output
        output -->|"Yes"| file["os.WriteFile(path, data, 0644)"]
        output -->|"No"| stdout["fmt.Println(string(data))"]
    end

    subgraph API["Export via API"]
        apiJson["GET /api/v1/cert/export/json"] --> svcAll["svc.CheckAll(ctx, hosts)"]
        svcAll --> jsonResp["writeJSON(200, {certificates, errors})\nContent-Disposition: attachment"]
        svcAll --> csvHeader["Content-Type: text/csv\nBOM UTF-8\nContent-Disposition: attachment"]
        csvHeader --> csvWrite["csv.NewWriter:\nwrites header + rows"]
    end

    subgraph Formats["Output Formats"]
        direction TB
        f1["FormatJSON:\njson.MarshalIndent(certs, '', '  ')"]
        f2["FormatCSV:\ncsv.Writer with header:\nhostname,port,commonName,issuer,\nnotBefore,notAfter,daysLeft,\nrevocationStatus,tlsVersion,cipherSuite"]
        f3["FormatTable (CLI only):\nTable with columns:\nHost, Port, Days, Status, Revoc,\nIssuer, TLS Version"]
        f4["Individual JSON:\njson.MarshalIndent(cert, '', '  ')"]
    end

    Formats --> f1
    Formats --> f2
    Formats --> f3
    Formats --> f4
```

### Formatting Differences

| Format | Where | Output |
|---|---|---|
| `FormatJSON` | CLI `export -f json` | `[{...}, {...}]` — array of certificates |
| `FormatCSV` | CLI `export -f csv` | CSV with header + 10 columns |
| `FormatTable` | CLI `check -o table` | Aligned table with colored status |
| Individual JSON | CLI `check` (default) | One JSON object per certificate |

---

## 9. Revocation Check

```mermaid
flowchart TB
    revocation["revocation.Check(leaf, issuer, ocspServers, crlURLs)"]

    revocation --> hasOCSP{"Has OCSP\nservers?"}
    hasOCSP -->|"Yes"| ocsp["CheckOCSP(leaf, issuer, servers)"]

    ocsp --> ocspLoop["For each OCSP server:"]
    ocspLoop --> createReq["ocsp.CreateRequest(leaf, issuer)"]
    createReq --> post["POST application/ocsp-request\n(10s timeout)"]

    post --> parseResp["ocsp.ParseResponse(bytes, issuer)"]
    parseResp --> ocspStatus{"OCSP status:"}
    ocspStatus -->|"Good"| good["RevocationGood"]
    ocspStatus -->|"Revoked"| revoked["RevocationRevoked"]
    ocspStatus -->|"Unknown"| tryNext["Try next server\n(or return Unknown)"]

    hasOCSP -->|"No"| crl["CheckCRL(leaf, crlURLs)"]

    crl --> crlLoop["For each CRL URL:"]
    crlLoop --> download["GET (10s timeout)"]
    download --> parseCRL["x509.ParseRevocationList(bytes)"]

    parseCRL --> search{"SerialNumber\nis on the\nrevoked list?"}
    search -->|"Yes"| crlRevoked["RevocationRevoked"]
    search -->|"No"| crlGood["RevocationGood"]

    good --> statusFinal["Final Status"]
    revoked --> statusFinal
    crlRevoked --> statusFinal
    crlGood --> statusFinal

    subgraph fallback["Fallback"]
        direction LR
        notReady["No OCSP server\nor CRL URL available"]
        notReady --> resultNotReady["RevocationNotReady"]
    end

    hasOCSP -->|"No"| fallback
    crl -->|no CRLs| fallback
    fallback --> statusFinal
    statusFinal --> log["LogRevocation(cert, status)\nWarn if revoked"]
```

### Check Priority

1. **OCSP** (Online Certificate Status Protocol) — primary attempt, faster and more accurate
2. **CRL** (Certificate Revocation List) — fallback when OCSP is unavailable or returns `NotReady`
3. **NotReady** — when the certificate has no OCSP servers or CRL URLs

---

## Data Model — Certificate

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

## Directory Structure

```
certificate-validate/
├── cmd/certificate-validate/
│   └── main.go                    # Entry point
├── config/
│   └── settings.yml               # YAML configuration
├── docs/
│   ├── swagger.yaml               # OpenAPI 3.0
│   └── ARCHITECTURE.md            # This document
├── internal/
│   ├── api/                       # HTTP interface
│   │   ├── api.go                 # Handlers + middleware + rate limiter
│   │   └── static/                # Embedded frontend (embed)
│   ├── certificate/               # Domain
│   │   ├── certificate.go         # VO + FromX509 + BuildChain
│   │   └── errors.go              # Domain errors
│   ├── checker/                   # Use case
│   │   └── checker.go             # Checker (orchestration)
│   ├── cmd/                       # CLI (Cobra)
│   │   ├── root.go                # Root command + global flags
│   │   ├── check.go               # check + watch
│   │   ├── serve.go               # serve + buildDeps + startBackground
│   │   ├── export.go              # export JSON/CSV
│   │   └── version.go             # version
│   ├── config/                    # Configuration
│   │   └── config.go              # Load + Validate + applyEnvOverrides
│   ├── fetcher/                   # TLS provider
│   │   └── fetcher.go             # Fetcher interface + tlsFetcher
│   ├── formatter/                 # Formatting provider
│   │   └── formatter.go           # Formatter, FormatTable, FormatJSON, FormatCSV
│   ├── history/                   # History
│   │   └── history.go             # Store, Recorder, JSONL rotation
│   ├── metrics/                   # Prometheus metrics
│   │   └── metrics.go             # Gauges + Handler
│   ├── notifier/                  # Webhook
│   │   └── notifier.go            # Notifier + alerts
│   ├── revocation/                # Revocation
│   │   └── revocation.go          # OCSP + CRL checks
│   └── service/                   # Service (facade)
│       └── service.go             # CertService
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```
