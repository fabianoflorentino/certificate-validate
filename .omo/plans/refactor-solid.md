# SOLID Refactor + Tests

## Goal
Apply SOLID across the whole codebase and unlock real unit tests.

## Identified Problems

| # | Problem | Impact |
|---|---------|--------|
| 1 | `toCheckerHosts` duplicated in 3 files | Error-prone maintenance |
| 2 | `Checker` coupled to format (`CheckAll` returns `[][]byte`) | Consumers re-parse JSON |
| 3 | `history.Record()` receives `json.RawMessage` | Format leak |
| 4 | Concrete dependencies instead of interfaces | No unit tests possible |
| 5 | `RunWatchLoop` in the checker (wrong responsibility) | CLI code in the domain |
| 6 | No tests | Any refactor is risky |

## Execution Plan

### Phase A — Foundation (0 breaking changes)

1. **config**: Move `toCheckerHosts` → `config.ToCheckerHosts()`, remove duplicates
2. **checker**: Extract `CertChecker` interface (Check, CheckAll) — consumers depend on it
3. **history**: Extract `Store` interface (Record, GetHistory)

### Phase B — Domain types (breaks format)

4. **checker**: `CheckAll` returns `[]*certificate.Certicate` instead of `[][]byte`
5. **history**: `Record` accepts `[]certificate.Certificate` instead of `[]json.RawMessage`
6. **metrics**: Update `UpdateFromJSON` → `Update([]certificate.Certificate)`

### Phase C — Service layer

7. **service/**: New package with `CertService` that orchestrates checker + history + metrics
8. **api**: Handler depends on `service.CertService` and interfaces, not concretes
9. **cmd/**: `RunWatchLoop` moves to `cmd/serve.go`

### Phase D — Tests

10. **config**: Tests for `ToCheckerHosts`
11. **history**: Tests for `Record`, `GetHistory`, `rotate` (with `t.TempDir`)
12. **checker**: Tests with mock `Fetcher`
13. **api**: HTTP tests with `httptest` + mocks

---

## Implementation Order

```
Phase A1 → A2 → A3 → B4 → B5 → B6 → C7 → C8 → C9 → D10 → D11 → D12 → D13
```

Each step keeps `go build ./...` and `go vet ./...` clean before moving on.
