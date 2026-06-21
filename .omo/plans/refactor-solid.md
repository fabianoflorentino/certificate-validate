# Refactor SOLID + Testes

## Objetivo
Aplicar SOLID em todo o codebase e destravar testes unitários reais.

## Problemas Identificados

| # | Problema | Impacto |
|---|----------|---------|
| 1 | `toCheckerHosts` duplicado em 3 arquivos | Manutenção propensa a erro |
| 2 | `Checker` acoplado a formato (`CheckAll` retorna `[][]byte`) | Consumidores re-parseiam JSON |
| 3 | `history.Record()` recebe `json.RawMessage` | Vazamento de formato |
| 4 | Dependências concretas em vez de interfaces | Nenhum teste unitário possível |
| 5 | `RunWatchLoop` no checker (responsabilidade errada) | Código de CLI no domínio |
| 6 | Nenhum teste | Qualquer refactor é risco |

## Plano de Execução

### Fase A — Foundation (0 breaking changes)

1. **config**: Mover `toCheckerHosts` → `config.ToCheckerHosts()`, remover duplicatas
2. **checker**: Extrair `CertChecker` interface (Check, CheckAll) — consumidores dependem dela
3. **history**: Extrair `Store` interface (Record, GetHistory)

### Fase B — Domain types (quebra formato)

4. **checker**: `CheckAll` retorna `[]*certificate.Certicate` em vez de `[][]byte`
5. **history**: `Record` aceita `[]certificate.Certificate` em vez de `[]json.RawMessage`
6. **metrics**: Atualizar `UpdateFromJSON` → `Update([]certificate.Certificate)`

### Fase C — Service layer

7. **service/**: Novo package com `CertService` que orquestra checker + history + metrics
8. **api**: Handler depende de `service.CertService` e interfaces, não concretos
9. **cmd/**: `RunWatchLoop` move para `cmd/serve.go`

### Fase D — Testes

10. **config**: Testes para `ToCheckerHosts`
11. **history**: Testes para `Record`, `GetHistory`, `rotate` (com `t.TempDir`)
12. **checker**: Testes com mock `Fetcher`
13. **api**: Testes HTTP com `httptest` + mocks

---

## Ordem de Implementação

```
Fase A1 → A2 → A3 → B4 → B5 → B6 → C7 → C8 → C9 → D10 → D11 → D12 → D13
```

Cada passo mantém `go build ./...` e `go vet ./...` limpos antes de avançar.
