# Relatório de Auditoria de Segurança

## certificate-validate

**Data:** 30 de agosto de 2026  
**Escopo:** Auditoria de segurança completa adaptada à stack Go/Cobra/net/http  
**Versão auditada:** v1.0.5  
**Status:** ✅ **Todos os achados corrigidos**

---

## Metodologia

Esta auditoria foi realizada adaptando as 5 categorias padrão de segurança à stack específica do projeto:

- **Linguagem:** Go 1.27
- **Framework CLI:** Cobra
- **Framework HTTP:** net/http (stdlib)
- **Autenticação:** API Key via header X-API-Key
- **Frontend:** Vanilla JS embutido (app.js)
- **Deploy:** Docker, Kubernetes, Helm

### Categorias adaptadas:

1. **BANCO SEM TRANCA** → Não aplicável (projeto não tem multi-tenancy)
2. **PERMISSÃO DEFINIDA NO NAVEGADOR** → Não aplicável (não há sistema de roles/admin)
3. **IDOR** → Não aplicável (API apenas lê dados de configuração, não há objetos por ID)
4. **CHAVES EXPOSTAS** → Verificado: API keys, secrets em configs, CI/CD
5. **INPUTS SEM TRATAMENTO (XSS)** → Verificado: frontend JS, templates, outputs

---

## Resumo Executivo

### Total de achados: 5 (todos corrigidos)

```mermaid
pie title Distribuição por Severidade
    "Média (1)" : 1
    "Baixa (3)" : 3
    "Informativa (1)" : 1
```

```mermaid
xychart-beta
    title "Distribuição por Categoria"
    x-axis ["CHAVES EXPOSTAS", "INPUTS SEM TRATAMENTO"]
    y-axis "Número de achados" 0 --> 5
    bar [3, 2]
```

### Severidades:
- 🔴 **Crítica:** 0
- 🟠 **Alta:** 0
- 🟡 **Média:** 1 ✅ Corrigida
- 🔵 **Baixa:** 3 ✅ Corrigidas
- ⚪ **Informativa:** 1 ✅ Corrigida

### Status das Correções:
- ✅ **100% dos achados resolvidos**
- 📝 **3 issues criadas e fechadas no GitHub**
- 🔒 **Commit de correção: f3a5a15**

---

## Pontos Fortes ✅

O projeto demonstra implementação sólida de várias práticas de segurança:

1. **Rate limiting implementado**
   - Token bucket com 100 req/s e burst 200 previne abuso e DoS básico
   - _Evidência:_ `internal/api/api.go:24-58`

2. **Security headers configurados**
   - X-Content-Type-Options: nosniff e X-Frame-Options: DENY
   - _Evidência:_ `internal/api/api.go:327-328`

3. **XSS mitigado com esc() consistente**
   - Função esc() usa createTextNode() do DOM para escapar HTML
   - _Evidência:_ `internal/api/static/app.js:553-557`

4. **Container Docker roda como non-root**
   - Usuário appuser (UID 1000) reduz impacto de comprometimento
   - _Evidência:_ `Dockerfile:12, 19`

5. **Timeout de contexto em health checks**
   - context.WithTimeout(5s) previne hang em hosts inacessíveis
   - _Evidência:_ `internal/api/api.go:260-261`

6. **Hot-reload seguro com atomic.Value**
   - Swap atômico do handler previne race conditions
   - _Evidência:_ `internal/cmd/serve.go:62-63, 151`

7. **CSV export com UTF-8 BOM**
   - BOM UTF-8 garante compatibilidade com Excel
   - _Evidência:_ `internal/api/api.go:187-190`

8. **Comparação de API key segura contra timing attacks** ✅ NOVO
   - Uso de crypto/subtle.ConstantTimeCompare
   - _Evidência:_ `internal/api/api.go:332`

9. **Validação de startup para autenticação** ✅ NOVO
   - Warning quando API server inicia sem autenticação
   - Flag --allow-insecure para suppressir warning
   - _Evidência:_ `internal/cmd/serve.go:202-215`

---

## Achados Detalhados (Todos Corrigidos)

### SEC-001: Comparação de API key vulnerável a timing attack ✅ CORRIGIDO

| Campo | Valor |
|-------|-------|
| **Severidade** | 🟡 Média |
| **Categoria** | CHAVES EXPOSTAS |
| **Arquivo** | `internal/api/api.go:332` |
| **Status** | ✅ **Corrigido no commit f3a5a15** |
| **Issue** | [#121](https://github.com/fabianoflorentino/certificate-validate/issues/121) |

**Descrição Original:**  
A comparação de strings usando `!=` em Go não é constante no tempo, permitindo que um atacante determine o valor correto da API key medindo o tempo de resposta de múltiplas requisições.

**Código Original:**
```go
if r.Header.Get("X-API-Key") != h.apiToken {
```

**Correção Aplicada:**
```go
import "crypto/subtle"

// Uso de comparação constante no tempo
if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(h.apiToken)) != 1 {
```

---

### SEC-002: Função escAttr() não escapa caracteres < e > ✅ CORRIGIDO

| Campo | Valor |
|-------|-------|
| **Severidade** | 🔵 Baixa |
| **Categoria** | INPUTS SEM TRATAMENTO (XSS) |
| **Arquivo** | `internal/api/static/app.js:559-561` |
| **Status** | ✅ **Corrigido no commit f3a5a15** |
| **Issue** | [#122](https://github.com/fabianoflorentino/certificate-validate/issues/122) |

**Descrição Original:**  
A função escAttr() apenas escapa aspas, mas não escapa < e >. Se um valor de certificado (issuer, commonName) contiver tags HTML, pode ser injetado em atributos title.

**Código Original:**
```javascript
function escAttr(str) {
    return str.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}
```

**Correção Aplicada:**
```javascript
function escAttr(str) {
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}
```

---

### SEC-003: Valor cert.port não é escapado no modal ✅ CORRIGIDO

| Campo | Valor |
|-------|-------|
| **Severidade** | 🔵 Baixa |
| **Categoria** | INPUTS SEM TRATAMENTO (XSS) |
| **Arquivo** | `internal/api/static/app.js:149, 251` |
| **Status** | ✅ **Corrigido no commit f3a5a15** |
| **Issue** | [#122](https://github.com/fabianoflorentino/certificate-validate/issues/122) |

**Descrição Original:**  
O valor cert.port é inserido diretamente no HTML sem escaping. Embora port seja um número inteiro vindo do backend, a falta de escaping consistente é uma prática insegura.

**Código Original:**
```javascript
'<span class="card-port">:' + cert.port + '</span>'
'<span class="detail-value">' + cert.port + '</span>'
```

**Correção Aplicada:**
```javascript
'<span class="card-port">:' + esc(String(cert.port)) + '</span>'
'<span class="detail-value">' + esc(String(cert.port)) + '</span>'
```

---

### SEC-004: Ausência de validação de startup para API key padrão ✅ CORRIGIDO

| Campo | Valor |
|-------|-------|
| **Severidade** | 🔵 Baixa |
| **Categoria** | CHAVES EXPOSTAS |
| **Arquivo** | `internal/cmd/serve.go:202-215` |
| **Status** | ✅ **Corrigido no commit f3a5a15** |
| **Issue** | [#123](https://github.com/fabianoflorentino/certificate-validate/issues/123) |

**Descrição Original:**  
O servidor inicia sem API key se nenhuma for configurada. Não há validação de startup que avise ou rejeite configuração sem autenticação em ambientes de produção.

**Correção Aplicada:**
```go
// Adicionada flag --allow-insecure
serveCmd.Flags().BoolVarP(&allowInsecureFlag, "allow-insecure", "", false, 
    "suppress warning when starting without API key authentication")

// Warning de segurança no startup
if apiToken == "" && !allowInsecureFlag {
    slog.Warn("API server starting WITHOUT authentication. " +
        "Set api_key in config or use --api-key flag. " +
        "Use --allow-insecure to suppress this warning.")
}
```

---

### SEC-005: Kubernetes Deployment não configura API key como Secret ✅ CORRIGIDO

| Campo | Valor |
|-------|-------|
| **Severidade** | ⚪ Informativa |
| **Categoria** | CHAVES EXPOSTAS |
| **Arquivo** | `kubernetes/Deployment.yml:1-62` |
| **Status** | ✅ **Corrigido no commit f3a5a15** |
| **Issue** | [#123](https://github.com/fabianoflorentino/certificate-validate/issues/123) |

**Descrição Original:**  
O manifest Kubernetes não demonstra como configurar a API key via Secret. Usuários podem não perceber que precisam criar um Secret separado.

**Correção Aplicada:**
```yaml
# Adicionado exemplo de criação de Secret no topo do arquivo
# Create API key secret before deploying:
# kubectl create secret generic certificate-validate-api-key \
#   --from-literal=API_KEY=your-secret-key-here \
#   -n certificate-validate

# Adicionado exemplo comentado de uso do Secret
# Optional: Configure API key for authentication
# Uncomment and create the secret first
# - name: CV_API_KEY
#   valueFrom:
#     secretKeyRef:
#       name: certificate-validate-api-key
#       key: API_KEY
```

---

## Recomendações Implementadas

### ✅ P1 - Alta Prioridade (Implementado)
- **Implementar comparação constante para API key** usando `crypto/subtle.ConstantTimeCompare`
  - Commit: f3a5a15
  - Arquivo: internal/api/api.go

### ✅ P2 - Média Prioridade (Implementado)
- **Melhorar função escAttr()** para escapar todos os caracteres especiais HTML
  - Commit: f3a5a15
  - Arquivo: internal/api/static/app.js

### ✅ P3 - Baixa Prioridade (Implementado)
- **Adicionar escaping consistente** em todos os valores do frontend (incluindo cert.port)
  - Commit: f3a5a15
  - Arquivo: internal/api/static/app.js

- **Adicionar warning no startup** quando API não tem autenticação configurada
  - Commit: f3a5a15
  - Arquivo: internal/cmd/serve.go

### ✅ P4 - Documentação (Implementado)
- **Documentar configuração de API key** via Kubernetes Secret nos manifests de exemplo
  - Commit: f3a5a15
  - Arquivo: kubernetes/Deployment.yml

---

## Histórico de Issues

### Issue #121: Comparação de API key vulnerável a timing attack
- **Status:** ✅ Fechada
- **Labels:** security, medium
- **Correção:** Uso de crypto/subtle.ConstantTimeCompare
- **Commit:** f3a5a15

### Issue #122: Melhorar escaping de atributos HTML no frontend
- **Status:** ✅ Fechada
- **Labels:** security, low
- **Correção:** Melhoria da função escAttr() e escaping de cert.port
- **Commit:** f3a5a15

### Issue #123: Adicionar validação de startup para API sem autenticação
- **Status:** ✅ Fechada
- **Labels:** security, low, documentation
- **Correção:** Adição de warning no startup e flag --allow-insecure
- **Commit:** f3a5a15

---

## Conclusão

O projeto **certificate-validate** demonstra excelentes práticas de segurança em sua arquitetura e implementação. Todos os 5 achados identificados na auditoria foram **corrigidos com sucesso** no commit f3a5a15.

### Pontos Fortes Significativos:
- ✅ Rate limiting implementado
- ✅ Security headers configurados
- ✅ XSS mitigado com esc() consistente
- ✅ Container Docker roda como non-root
- ✅ Timeout de contexto em health checks
- ✅ Hot-reload seguro com atomic.Value
- ✅ CSV export com UTF-8 BOM
- ✅ **Comparação de API key segura contra timing attacks** (novo)
- ✅ **Validação de startup para autenticação** (novo)

### Melhorias Implementadas:
1. **Segurança criptográfica:** Comparação constante no tempo para API keys
2. **Defesa em profundidade:** Escaping consistente em todo o frontend
3. **Segurança operacional:** Warning de startup para deploys sem autenticação
4. **Documentação:** Exemplos claros de configuração segura no Kubernetes

### Status Final:
- 🎯 **100% dos achados corrigidos**
- 🔒 **3 issues criadas e fechadas**
- ✅ **Todos os testes passando (283 testes)**
- 📦 **Build bem-sucedido**

O projeto agora está em conformidade com as melhores práticas de segurança para aplicações Go/HTTP, pronto para uso em produção com confiança.

---

**Relatório gerado em:** 30 de agosto de 2026  
**Auditor:** Análise automatizada de segurança  
**Método:** Revisão de código estático adaptada à stack Go/HTTP  
**Status:** ✅ Auditoria completa com todas as correções implementadas
