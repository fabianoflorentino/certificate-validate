---
name: golang-patterns
description: Idiomatic Go patterns, best practices, and conventions for building robust, efficient, and maintainable Go applications.
origin: ECC
---

# Go Development Patterns

Idiomatic Go patterns and best practices for building robust, efficient, and maintainable applications.

## When to Activate

- Writing new Go code
- Reviewing Go code
- Refactoring existing Go code
- Designing Go packages/modules

## Project Conventions (Mandatory)

These apply to all work in this repository and are enforced by CI/git hooks.

1. **English only.** All code (identifiers, comments), commit messages, docs, and
   site content must be written in English. No Portuguese or other languages in
   any file.

2. **SOLID, applied lightly.** Keep responsibilities focused, depend on
   abstractions, and compose small pieces. Do not over-engineer; state this style
   explicitly as a design goal (see the SOLID section below).

3. **Always validate with lefthook.** Do not commit or push without ensuring the
   `pre-commit` (lint, vet) and `pre-push` (verify-deps, lint, vet, test -race,
   coverage ≥ 80%) hooks pass. Run `lefthook run pre-commit` / `lefthook run
   pre-push` before finalizing any change.

## Core Principles

### 1. Simplicity and Clarity

Go favors simplicity over cleverness. Code should be obvious and easy to read.

```go
// Good: Clear and direct
func GetUser(id string) (*User, error) {
    user, err := db.FindUser(id)
    if err != nil {
        return nil, fmt.Errorf("get user %s: %w", id, err)
    }
    return user, nil
}

// Bad: Overly clever
func GetUser(id string) (*User, error) {
    return func() (*User, error) {
        if u, e := db.FindUser(id); e == nil {
            return u, nil
        } else {
            return nil, e
        }
    }()
}
```

### 2. Make the Zero Value Useful

Design types so their zero value is immediately usable without initialization.

```go
// Good: Zero value is useful
type Counter struct {
    mu    sync.Mutex
    count int // zero value is 0, ready to use
}

func (c *Counter) Inc() {
    c.mu.Lock()
    c.count++
    c.mu.Unlock()
}

// Good: bytes.Buffer works with zero value
var buf bytes.Buffer
buf.WriteString("hello")

// Bad: Requires initialization
type BadCounter struct {
    counts map[string]int // nil map will panic
}
```

### 3. Accept Interfaces, Return Structs

Functions should accept interface parameters and return concrete types.

```go
// Good: Accepts interface, returns concrete type
func ProcessData(r io.Reader) (*Result, error) {
    data, err := io.ReadAll(r)
    if err != nil {
        return nil, err
    }
    return &Result{Data: data}, nil
}

// Bad: Returns interface (hides implementation details unnecessarily)
func ProcessData(r io.Reader) (io.Reader, error) {
    // ...
}
```

## Error Handling Patterns

### Error Wrapping with Context

```go
// Good: Wrap errors with context
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("load config %s: %w", path, err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config %s: %w", path, err)
    }

    return &cfg, nil
}
```

### Custom Error Types

```go
// Define domain-specific errors
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}

// Sentinel errors for common cases
var (
    ErrNotFound     = errors.New("resource not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrInvalidInput = errors.New("invalid input")
)
```

### Error Checking with errors.Is and errors.As

```go
func HandleError(err error) {
    // Check for specific error
    if errors.Is(err, sql.ErrNoRows) {
        log.Println("No records found")
        return
    }

    // Check for error type
    var validationErr *ValidationError
    if errors.As(err, &validationErr) {
        log.Printf("Validation error on field %s: %s",
            validationErr.Field, validationErr.Message)
        return
    }

    // Unknown error
    log.Printf("Unexpected error: %v", err)
}
```

### Never Ignore Errors

```go
// Bad: Ignoring error with blank identifier
result, _ := doSomething()

// Good: Handle or explicitly document why it's safe to ignore
result, err := doSomething()
if err != nil {
    return err
}

// Acceptable: When error truly doesn't matter (rare)
_ = writer.Close() // Best-effort cleanup, error logged elsewhere
```

## Concurrency Patterns

### Worker Pool

```go
func WorkerPool(jobs <-chan Job, results chan<- Result, numWorkers int) {
    var wg sync.WaitGroup

    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs {
                results <- process(job)
            }
        }()
    }

    wg.Wait()
    close(results)
}
```

### Context for Cancellation and Timeouts

```go
func FetchWithTimeout(ctx context.Context, url string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fetch %s: %w", url, err)
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}
```

### Graceful Shutdown

```go
func GracefulShutdown(server *http.Server) {
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    <-quit
    log.Println("Shutting down server...")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }

    log.Println("Server exited")
}
```

### errgroup for Coordinated Goroutines

```go
import "golang.org/x/sync/errgroup"

func FetchAll(ctx context.Context, urls []string) ([][]byte, error) {
    g, ctx := errgroup.WithContext(ctx)
    results := make([][]byte, len(urls))

    for i, url := range urls {
        i, url := i, url // Capture loop variables
        g.Go(func() error {
            data, err := FetchWithTimeout(ctx, url)
            if err != nil {
                return err
            }
            results[i] = data
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        return nil, err
    }
    return results, nil
}
```

### Avoiding Goroutine Leaks

```go
// Bad: Goroutine leak if context is cancelled
func leakyFetch(ctx context.Context, url string) <-chan []byte {
    ch := make(chan []byte)
    go func() {
        data, _ := fetch(url)
        ch <- data // Blocks forever if no receiver
    }()
    return ch
}

// Good: Properly handles cancellation
func safeFetch(ctx context.Context, url string) <-chan []byte {
    ch := make(chan []byte, 1) // Buffered channel
    go func() {
        data, err := fetch(url)
        if err != nil {
            return
        }
        select {
        case ch <- data:
        case <-ctx.Done():
        }
    }()
    return ch
}
```

## SOLID (Applied Lightly)

Use SOLID as a gentle design compass, not dogma. The goal is consistency,
focused responsibilities, and easy maintenance without over-engineering.

### S — Single Responsibility

Each type/function/package should have one clear reason to change. If a struct is
growing unrelated concerns, extract them.

```go
// Good: One responsibility per type
type Reporter struct { dashboard *Dashboard }
func (r *Reporter) Emit(metrics []Metric) error { ... }

// Bad: The same struct also knows how to persist, email, and format JSON
type UberReporter struct{}
```

### O — Open/Closed

Prefer extending behavior through interfaces/embedding rather than modifying
working code.

```go
// Good: New output formats add a new implementation, not an if/else chain
type Output interface{ Write(io.Writer, []Metric) error }

// Bad: Every new format edits this switch
func write(w io.Writer, f string, m []Metric) error {
    switch f {
    case "json": ...
    case "csv": ...
    case "yaml": ... // add another case here
    }
}
```

### L — Liskov Substitution

Implementations passed where an interface is expected should behave according to
the interface contract:

```go
// Good: FakeWebhook and RealWebhook both satisfy Notifier correctly
type Notifier interface{ Notify(ctx context.Context, m Message) error }
```

### I — Interface Segregation

Keep interfaces small and focused; consumers depend only on what they use. This
is already Go's recommended style (see "Small, Focused Interfaces" below).

```go
// Good: Sized consumer only needs Size()
type Sizer interface{ Size() int }

// Bad: Consumer is forced to know about unrelated methods
type Everything interface { Size() int; Write(p []byte) error; Close() error }
```

### D — Dependency Inversion

Depend on abstractions, inject concrete dependencies — never reach for package
globals or construct dependencies internally (see "Avoid Package-Level State").

```go
// Good: Dependency injected through the constructor
type Handler struct { validate Validator }
func NewHandler(v Validator) *Handler { return &Handler{validate: v} }
```

## Interface Design

### Small, Focused Interfaces

```go
// Good: Single-method interfaces
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

type Closer interface {
    Close() error
}

// Compose interfaces as needed
type ReadWriteCloser interface {
    Reader
    Writer
    Closer
}
```

### Define Interfaces Where They're Used

```go
// In the consumer package, not the provider
package service

// UserStore defines what this service needs
type UserStore interface {
    GetUser(id string) (*User, error)
    SaveUser(user *User) error
}

type Service struct {
    store UserStore
}

// Concrete implementation can be in another package
// It doesn't need to know about this interface
```

### Optional Behavior with Type Assertions

```go
type Flusher interface {
    Flush() error
}

func WriteAndFlush(w io.Writer, data []byte) error {
    if _, err := w.Write(data); err != nil {
        return err
    }

    // Flush if supported
    if f, ok := w.(Flusher); ok {
        return f.Flush()
    }
    return nil
}
```

## Package Organization

### Standard Project Layout

```text
myproject/
├── cmd/
│   └── myapp/
│       └── main.go           # Entry point
├── internal/
│   ├── handler/              # HTTP handlers
│   ├── service/              # Business logic
│   ├── repository/           # Data access
│   └── config/               # Configuration
├── pkg/
│   └── client/               # Public API client
├── api/
│   └── v1/                   # API definitions (proto, OpenAPI)
├── testdata/                 # Test fixtures
├── go.mod
├── go.sum
└── Makefile
```

### Package Naming

```go
// Good: Short, lowercase, no underscores
package http
package json
package user

// Bad: Verbose, mixed case, or redundant
package httpHandler
package json_parser
package userService // Redundant 'Service' suffix
```

### Avoid Package-Level State

```go
// Bad: Global mutable state
var db *sql.DB

func init() {
    db, _ = sql.Open("postgres", os.Getenv("DATABASE_URL"))
}

// Good: Dependency injection
type Server struct {
    db *sql.DB
}

func NewServer(db *sql.DB) *Server {
    return &Server{db: db}
}
```

## Documentation (Godoc)

Go documentation is written as comments and extracted by godoc to generate
documentation. Follow these conventions for idiomatic Go documentation.

### Package Documentation (doc.go)

Every package should have a `doc.go` file with a package-level comment that
explains the package's purpose, key types, and usage examples.

```go
// Package certificate provides types and utilities for parsing, analyzing,
// and representing SSL/TLS certificate information.
//
// The package defines the core Certificate type that holds extracted information
// from X.509 certificates, including subject, issuer, validity period, revocation
// status, and the certificate chain.
//
// # Certificate Model
//
// The Certificate struct is the central type, containing all relevant information
// extracted from a TLS connection:
//
//   - CommonName and SubjectAltNames for identity
//   - Issuer and chain information
//   - Validity period (NotBefore, NotAfter, DaysLeft)
//   - Revocation status (OCSP/CRL)
//   - TLS version and cipher suite
//
// # Usage
//
// Build a Certificate from an x509.Certificate:
//
//	cert := certificate.FromX509(x509Cert, "example.com", 443)
//	cert.Chain = certificate.BuildChain(peerCerts)
package certificate
```

**Key points:**
- Start with `// Package <name> ...` to identify the package
- Use `#` for section headers (rendered as headings in godoc)
- Include usage examples with proper indentation (godoc recognizes code blocks)
- Explain the "why" and "what", not the "how" (code shows the how)

### Exported Types and Functions

Every exported type, function, method, and constant should have a doc comment
starting with the name of the thing being documented.

```go
// Analyzer parses and validates Kubernetes TLS certificates.
type Analyzer struct {
    checkRevocation bool
}

// NewAnalyzer creates an Analyzer. When checkRevocation is true, OCSP/CRL
// checks are performed using the configured short timeout.
func NewAnalyzer(checkRevocation bool) *Analyzer {
    return &Analyzer{checkRevocation: checkRevocation}
}

// ParseBundle parses the tls.crt PEM bytes and returns the leaf certificate
// and its chain.
func (a *Analyzer) ParseBundle(pemBytes []byte) (*x509.Certificate, []*x509.Certificate, error) {
    // ...
}
```

**Key points:**
- Start with the name: `// TypeName ...` or `// FuncName ...`
- For methods: `// MethodName ...` (the receiver is implicit)
- Explain what it does, not how (the code shows the how)
- Document edge cases, zero values, and error conditions

### Interface Documentation

Document the contract, not the implementation:

```go
// Store is the interface for recording and querying certificate history.
// Consumers (api) depend on this, not on the concrete Recorder.
type Store interface {
    Record(results []*certificate.Certificate)
    GetHistory(hostname string) ([]Entry, error)
}
```

### Constants and Variables

Group related constants and document the group:

```go
// Revocation status values returned by OCSP and CRL checks.
const (
    // RevocationUnknown indicates the revocation status could not be determined.
    RevocationUnknown RevocationStatus = "unknown"

    // RevocationGood indicates the certificate is not revoked.
    RevocationGood RevocationStatus = "good"

    // RevocationRevoked indicates the certificate has been revoked.
    RevocationRevoked RevocationStatus = "revoked"

    // RevocationNotReady indicates no OCSP or CRL endpoints are available.
    RevocationNotReady RevocationStatus = "not_ready"
)
```

**Key points:**
- Document the group with a comment before the `const (` or `var (` block
- Document each constant individually when the meaning is not obvious from the name
- Use full sentences for complex constants

### Sentinel Errors

Document sentinel errors with their meaning and usage:

```go
// Sentinel errors returned by certificate fetching operations.
// Use errors.Is to check for specific error conditions.
var (
    // ErrHostUnreachable indicates the host could not be reached (DNS failure,
    // connection refused, or network timeout).
    ErrHostUnreachable = errors.New("host unreachable")

    // ErrInvalidHostname indicates the hostname is invalid or could not be resolved.
    ErrInvalidHostname = errors.New("invalid hostname")

    // ErrCertificateFetch indicates a general failure during certificate fetching.
    ErrCertificateFetch = errors.New("failed to fetch certificate")

    // ErrNoCertificate indicates the server did not present a certificate during
    // the TLS handshake.
    ErrNoCertificate = errors.New("no peer certificate presented")
)
```

**Key points:**
- Group related errors in a `var (...)` block
- Document the group with a comment explaining the category
- Document each error with its meaning and when it's returned
- Mention how to check for the error (e.g., "Use errors.Is")

### Function Documentation Patterns

#### Constructor Functions (New*)

Document what the function creates and any important parameters:

```go
// New creates a new Checker with the given dependencies.
// The fetcher retrieves certificates, the formatter formats output.
func New(fetcher Fetcher, formatter Formatter) *Checker {
    return &Checker{
        fetcher:   fetcher,
        formatter: formatter,
    }
}
```

#### Methods

Document what the method does, parameters, and return values:

```go
// Check fetches certificate info for a single host.
// Returns the certificate or an error if the fetch fails.
func (c *Checker) Check(ctx context.Context, hostname string, port int) (*certificate.Certificate, error) {
    return c.fetcher.Fetch(ctx, hostname, port)
}
```

#### Complex Functions

For functions with complex behavior, document edge cases and error conditions:

```go
// CheckOCSP queries OCSP responders to verify certificate revocation status.
// Tries each server in order, returning the first definitive result.
// Returns RevocationNotReady if no OCSP servers are available.
// Returns RevocationUnknown if all queries fail or return unknown status.
func CheckOCSP(leaf *x509.Certificate, issuer *x509.Certificate, servers []string) certificate.RevocationStatus {
    // ...
}
```

#### Background Goroutines

Document the lifecycle and cancellation behavior:

```go
// StartUpdater periodically fetches certificates in the background and updates Prometheus gauges.
// The goroutine stops when the context is cancelled.
func StartUpdater(ctx context.Context, c checker.CertChecker, hosts []checker.Host, interval time.Duration) {
    // ...
}
```

### Examples

For complex packages, add example functions that godoc can render:

```go
// ExampleNew demonstrates creating and using a Fetcher.
func ExampleNew() {
    f := fetcher.New(10 * time.Second)
    cert, err := f.Fetch(context.Background(), "example.com", 443)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Certificate expires in %d days\n", cert.DaysLeft)
    // Output: Certificate expires in 90 days
}
```

### Documentation Checklist

When reviewing Go code, verify:

- [ ] Every package has a `doc.go` with package-level documentation
- [ ] All exported types have doc comments starting with the type name
- [ ] All exported functions/methods have doc comments
- [ ] All exported constants/variables are documented
- [ ] Interfaces document the contract, not implementation
- [ ] Error conditions are documented
- [ ] Usage examples are included for complex packages
- [ ] Comments explain "why", not "what" (code shows the what)
- [ ] Constructor functions document what they create
- [ ] Methods document parameters and return values
- [ ] Background goroutines document lifecycle and cancellation
- [ ] Sentinel errors document their meaning and usage

## Struct Design

### Functional Options Pattern

```go
type Server struct {
    addr    string
    timeout time.Duration
    logger  *log.Logger
}

type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) {
        s.timeout = d
    }
}

func WithLogger(l *log.Logger) Option {
    return func(s *Server) {
        s.logger = l
    }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{
        addr:    addr,
        timeout: 30 * time.Second, // default
        logger:  log.Default(),    // default
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
server := NewServer(":8080",
    WithTimeout(60*time.Second),
    WithLogger(customLogger),
)
```

### Embedding for Composition

```go
type Logger struct {
    prefix string
}

func (l *Logger) Log(msg string) {
    fmt.Printf("[%s] %s\n", l.prefix, msg)
}

type Server struct {
    *Logger // Embedding - Server gets Log method
    addr    string
}

func NewServer(addr string) *Server {
    return &Server{
        Logger: &Logger{prefix: "SERVER"},
        addr:   addr,
    }
}

// Usage
s := NewServer(":8080")
s.Log("Starting...") // Calls embedded Logger.Log
```

## Memory and Performance

### Preallocate Slices When Size is Known

```go
// Bad: Grows slice multiple times
func processItems(items []Item) []Result {
    var results []Result
    for _, item := range items {
        results = append(results, process(item))
    }
    return results
}

// Good: Single allocation
func processItems(items []Item) []Result {
    results := make([]Result, 0, len(items))
    for _, item := range items {
        results = append(results, process(item))
    }
    return results
}
```

### Use sync.Pool for Frequent Allocations

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func ProcessRequest(data []byte) []byte {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()

    buf.Write(data)
    // Process...
    return buf.Bytes()
}
```

### Avoid String Concatenation in Loops

```go
// Bad: Creates many string allocations
func join(parts []string) string {
    var result string
    for _, p := range parts {
        result += p + ","
    }
    return result
}

// Good: Single allocation with strings.Builder
func join(parts []string) string {
    var sb strings.Builder
    for i, p := range parts {
        if i > 0 {
            sb.WriteString(",")
        }
        sb.WriteString(p)
    }
    return sb.String()
}

// Best: Use standard library
func join(parts []string) string {
    return strings.Join(parts, ",")
}
```

## Go Tooling Integration

### Essential Commands

```bash
# Build and run
go build ./...
go run ./cmd/myapp

# Testing
go test ./...
go test -race ./...
go test -cover ./...

# Static analysis
go vet ./...
staticcheck ./...
golangci-lint run

# Module management
go mod tidy
go mod verify

# Formatting
gofmt -w .
goimports -w .
```

### Recommended Linter Configuration (.golangci.yml)

```yaml
linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - misspell
    - unconvert
    - unparam

linters-settings:
  errcheck:
    check-type-assertions: true
  govet:
    check-shadowing: true

issues:
  exclude-use-default: false
```

## Quick Reference: Go Idioms

| Idiom | Description |
|-------|-------------|
| English everywhere | All code, comments, commits, and docs in English |
| SOLID lightly | Focused responsibilities + abstractions, without over-engineering |
| Accept interfaces, return structs | Functions accept interface params, return concrete types |
| Errors are values | Treat errors as first-class values, not exceptions |
| Don't communicate by sharing memory | Use channels for coordination between goroutines |
| Make the zero value useful | Types should work without explicit initialization |
| A little copying is better than a little dependency | Avoid unnecessary external dependencies |
| Clear is better than clever | Prioritize readability over cleverness |
| gofmt is no one's favorite but everyone's friend | Always format with gofmt/goimports |
| Return early | Handle errors first, keep happy path unindented |
| Document everything exported | Every package, type, function, and constant needs godoc |
| Validate with lefthook | Run pre-commit/pre-push hooks before committing/pushing |

## Anti-Patterns to Avoid

```go
// Bad: Naked returns in long functions
func process() (result int, err error) {
    // ... 50 lines ...
    return // What is being returned?
}

// Bad: Using panic for control flow
func GetUser(id string) *User {
    user, err := db.Find(id)
    if err != nil {
        panic(err) // Don't do this
    }
    return user
}

// Bad: Passing context in struct
type Request struct {
    ctx context.Context // Context should be first param
    ID  string
}

// Good: Context as first parameter
func ProcessRequest(ctx context.Context, id string) error {
    // ...
}

// Bad: Mixing value and pointer receivers
type Counter struct{ n int }
func (c Counter) Value() int { return c.n }    // Value receiver
func (c *Counter) Increment() { c.n++ }        // Pointer receiver
// Pick one style and be consistent
```

**Remember**: Go code should be boring in the best way - predictable, consistent, and easy to understand. When in doubt, keep it simple. Keep everything in English, apply SOLID lightly, and always validate with lefthook before committing or pushing.
