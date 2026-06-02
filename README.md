# Certificate Validate

A modern, extensible SSL/TLS certificate validation tool written in Go. Fetches and inspects certificate information from remote hosts via CLI or HTTP API.

## Features

- **CLI Mode**: Check certificates from command line with watch mode support
- **HTTP API**: RESTful API for certificate inspection
- **Extensible Architecture**: SOLID principles with clean interfaces
- **Concurrent Processing**: Parallel certificate fetching
- **Type Safety**: Compile-time error checking
- **Minimal Dependencies**: Only 2 external packages (cobra, yaml.v3)
- **Production Ready**: Small Docker image (~10MB), proper error handling

## Architecture

The project follows Clean Architecture with SOLID principles:

```bash
certificate-validate/
├── cmd/certificate-validate/
│   └── main.go                    # Entry point with dependency injection
├── internal/
│   ├── certificate/               # DOMAIN LAYER (S, D)
│   │   ├── certificate.go         # Value object + extraction logic
│   │   └── errors.go              # Domain-specific errors
│   ├── config/                    # INFRASTRUCTURE (D)
│   │   └── config.go              # YAML configuration loader
│   ├── fetcher/                   # PROVIDER (D, I)
│   │   └── fetcher.go             # Interface + TLS implementation
│   ├── formatter/                 # PROVIDER (D, I)
│   │   └── formatter.go           # Interface + JSON implementation
│   ├── checker/                   # USE CASE (O, L)
│   │   └── checker.go             # Orchestration logic
│   ├── api/                       # INTERFACE (I)
│   │   └── api.go                 # HTTP handlers
│   └── cmd/                       # INTERFACE (I)
│       ├── root.go                # Cobra root command
│       ├── check.go               # CLI check command
│       └── serve.go               # CLI serve command
├── config/settings.yml            # Configuration file
├── Dockerfile                     # Multi-stage build
└── docker-compose.yml             # Container orchestration
```

### SOLID Principles Applied

| Principle | Implementation |
| ----------- | ---------------- |
| **S** - Single Responsibility | Each package has one responsibility: `certificate` = domain, `fetcher` = TLS connection, `formatter` = output, `checker` = orchestration |
| **O** - Open/Closed | `Fetcher` and `Formatter` interfaces allow new implementations without modifying existing code |
| **L** - Liskov Substitution | Explicit `(Certificate, error)` returns - no `sys.exit()`, no inconsistent types |
| **I** - Interface Segregation | Minimal interfaces: `Fetcher` has 1 method, `Formatter` has 1 method |
| **D** - Dependency Inversion | `checker` defines interfaces, providers implement them. `main.go` injects dependencies |

## Installation

### From Source

```bash
go install github.com/fabianoflorentino/certificate-validate/cmd/certificate-validate@latest
```

### Build Locally

```bash
git clone https://github.com/fabianoflorentino/certificate-validate.git
cd certificate-validate
go build -o certificate-validate ./cmd/certificate-validate
```

### Docker

```bash
docker build -t certificate-validate .
```

## Configuration

Create a `config/settings.yml` file:

```yaml
check_time: 3600  # Watch interval in seconds

app_configs:
  - name: 'certificate-validate'
    host: '0.0.0.0'
    port: '5000'
    environment: 'production'
    debug: false

hosts:
  - name: "github"
    url: 'github.com'
    port: '443'
  - name: "gitlab"
    url: 'gitlab.com'
    port: '443'
```

## Usage

### CLI Mode

Check certificates once:

```bash
./certificate-validate check
```

Watch mode (continuous checking):

```bash
./certificate-validate check --watch
```

Custom config file:

```bash
./certificate-validate -c /path/to/config.yml check
```

### HTTP API Mode

Start the API server:

```bash
./certificate-validate serve
```

### Docker (Terminal)

```bash
# CLI mode
docker run -v $(pwd)/config:/app/config certificate-validate check

# API mode
docker run -p 5000:5000 -v $(pwd)/config:/app/config certificate-validate serve
```

### Docker Compose

```bash
docker-compose up -d
```

## API Endpoints

| Endpoint | Method | Description |
| ---------- | -------- | ------------- |
| `/api/v1/cert/info/all` | GET | Get all certificates |
| `/api/v1/cert/info/{hostname}` | GET | Get certificate by hostname |
| `/api/v1/cert/info/commonName` | GET | Get all common names |
| `/api/v1/cert/info/subjectAltName` | GET | Get all subject alternative names |

### Example Response

```json
{
  "commonName": "github.com",
  "subjectAltName": ["github.com", "www.github.com"],
  "issuer": "Sectigo Public Server Authentication CA DV E36",
  "type": "Domain Validation (DV) Web Server SSL Digital Certificate",
  "notBefore": "2024-01-01 00:00:00",
  "notAfter": "2025-01-01 23:59:59",
  "daysLeft": 365,
  "crl": null,
  "hostname": "github.com",
  "port": 443
}
```

## Extending the Project

### Add a New Fetcher

Create a new implementation of the `Fetcher` interface:

```go
// internal/fetcher/file.go
package fetcher

import (
    "context"
    "github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

type fileFetcher struct {
    path string
}

func NewFileFetcher(path string) Fetcher {
    return &fileFetcher{path: path}
}

func (f *fileFetcher) Fetch(ctx context.Context, host string, port int) (*certificate.Certificate, error) {
    // Read certificate from PEM file
    // Return certificate.Certificate
}
```

### Add a New Formatter

Create a new implementation of the `Formatter` interface:

```go
// internal/formatter/prometheus.go
package formatter

import (
    "github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

type prometheusFormatter struct{}

func NewPrometheus() Formatter {
    return &prometheusFormatter{}
}

func (f *prometheusFormatter) Format(cert *certificate.Certificate) ([]byte, error) {
    // Format as Prometheus metrics
    // Return formatted bytes
}
```

### Add a New CLI Command

Create a new Cobra command:

```go
// internal/cmd/export.go
package cmd

import (
    "github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
    Use:   "export",
    Short: "Export certificates to file",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Implementation
        return nil
    },
}

func init() {
    rootCmd.AddCommand(exportCmd)
}
```

## Development

### Run Tests

```bash
go test ./...
```

### Build

```bash
go build -o certificate-validate ./cmd/certificate-validate
```

### Lint

```bash
go vet ./...
```

## Migration from Python

This project was migrated from Python to Go for:

- **Performance**: Native concurrency, no GIL
- **Deployment**: Single binary, no runtime dependencies
- **Type Safety**: Compile-time error checking
- **Maintainability**: Clean architecture, SOLID principles
- **Size**: Docker image reduced from ~180MB to ~10MB

### Key Improvements

| Aspect | Python (Old) | Go (New) |
|--------|--------------|----------|
| Dependencies | 23 packages | 2 packages |
| Docker Image | ~180MB | ~10MB |
| Concurrency | ThreadPoolExecutor | Goroutines + channels |
| Error Handling | `sys.exit()` in functions | `error` as value |
| Testability | Impossible (sys.exit) | 100% mockable |
| Extensibility | Modify existing code | Add implementations |
| Type System | Dynamic (runtime errors) | Static (compile-time) |

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Author

Fabiano Florentino
