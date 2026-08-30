---
title: "certificate-validate"
description: "A Go library and CLI for validating X.509 certificates, chains, and TLS endpoints"
---

## Validate X.509 certificates. Check chains. Verify TLS endpoints.

A Go library and CLI for validating certificates, chains, and TLS endpoints with support for CRL, OCSP, and custom roots.

### Quickstart

Install via Go:

```bash
go install github.com/fabianoflorentino/certificate-validate/cmd/certificate-validate@latest
certificate-validate --help
```

Or use the Docker image:

```bash
docker pull fabianoflorentino/certificate-validate:latest
docker run --rm fabianoflorentino/certificate-validate --help
```

### Key features

- **X.509 validation**: Validate certificates and chains
- **TLS endpoints**: Check remote servers (HTTPS, SMTP+STARTTLS, etc.)
- **CRL support**: Check certificate revocation lists
- **OCSP support**: Online Certificate Status Protocol
- **Custom roots**: Use your own CA bundle
- **Go library**: Import and use in your own projects

### Usage

```bash
# Validate a TLS endpoint
certificate-validate check example.com

# Validate a certificate file
certificate-validate check --cert cert.pem

# With custom CA bundle
certificate-validate check --ca-bundle custom-ca.pem example.com

# Check with verbose output
certificate-validate check --verbose example.com
```

### As a Go library

```go
package main

import (
    "fmt"
    "github.com/fabianoflorentino/certificate-validate/internal/validator"
)

func main() {
    v := validator.New()
    result, err := v.CheckEndpoint("example.com:443")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Printf("Valid: %v\n", result.Valid)
}
```

### Learn more

- [Go Documentation](https://pkg.go.dev/github.com/fabianoflorentino/certificate-validate)
- [Docker Hub](https://hub.docker.com/r/fabianoflorentino/certificate-validate)
- [GitHub Releases](https://github.com/fabianoflorentino/certificate-validate/releases)
