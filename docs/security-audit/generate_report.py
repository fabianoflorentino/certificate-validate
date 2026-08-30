#!/usr/bin/env python3
"""
Security Audit Report Generator for certificate-validate
Generates a professional PDF report with findings, charts, and GitHub issues.
"""

import os
import sys
from datetime import datetime

try:
    from reportlab.lib.pagesizes import A4
    from reportlab.lib.units import cm, mm
    from reportlab.lib.colors import HexColor, white, black
    from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
    from reportlab.lib.enums import TA_CENTER, TA_LEFT, TA_JUSTIFY
    from reportlab.platypus import (
        SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle,
        PageBreak, Image, KeepTogether
    )
    from reportlab.platypus.flowables import HRFlowable
    import matplotlib
    matplotlib.use('Agg')
    import matplotlib.pyplot as plt
    import matplotlib.patches as mpatches
except ImportError:
    print("Installing required packages...")
    os.system(f"{sys.executable} -m pip install reportlab matplotlib --quiet")
    from reportlab.lib.pagesizes import A4
    from reportlab.lib.units import cm, mm
    from reportlab.lib.colors import HexColor, white, black
    from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
    from reportlab.lib.enums import TA_CENTER, TA_LEFT, TA_JUSTIFY
    from reportlab.platypus import (
        SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle,
        PageBreak, Image, KeepTogether
    )
    from reportlab.platypus.flowables import HRFlowable
    import matplotlib
    matplotlib.use('Agg')
    import matplotlib.pyplot as plt
    import matplotlib.patches as mpatches

# Colors
CRITICAL = HexColor('#B91C1C')
HIGH = HexColor('#EA580C')
MEDIUM = HexColor('#D97706')
LOW = HexColor('#2563EB')
INFO = HexColor('#6B7280')
STRONG = HexColor('#059669')
BG_LIGHT = HexColor('#F9FAFB')
BG_DARK = HexColor('#1F2937')
TEXT_PRIMARY = HexColor('#111827')
TEXT_SECONDARY = HexColor('#6B7280')
BORDER = HexColor('#E5E7EB')

# Project info
PROJECT_NAME = "certificate-validate"
AUDIT_DATE = datetime.now().strftime("%d de agosto de 2026")
SCOPE = "Auditoria de segurança completa adaptada à stack Go/Cobra/net/http"

# Findings data
FINDINGS = [
    {
        "id": "SEC-001",
        "category": "CHAVES EXPOSTAS",
        "severity": "media",
        "title": "Comparação de API key vulnerável a timing attack",
        "file": "internal/api/api.go",
        "line": "332",
        "code": 'if r.Header.Get("X-API-Key") != h.apiToken {',
        "description": "A comparação de strings usando != em Go não é constante no tempo, permitindo que um atacante determine o valor correto da API key medindo o tempo de resposta de múltiplas requisições.",
        "impact": "Um atacante pode recuperar a API key caractere por caractere através de análise estatística do tempo de resposta, especialmente em redes de baixa latência.",
        "fix": "Usar crypto/subtle.ConstantTimeCompare para comparação segura contra timing attacks.",
        "exploitability": "Requer rede de baixa latência e múltiplas requisições. Mais fácil em ambientes locais ou com acesso direto à rede."
    },
    {
        "id": "SEC-002",
        "category": "INPUTS SEM TRATAMENTO (XSS)",
        "severity": "baixa",
        "title": "Função escAttr() não escapa caracteres < e >",
        "file": "internal/api/static/app.js",
        "line": "559-561",
        "code": 'function escAttr(str) {\n    return str.replace(/"/g, \'&quot;\').replace(/\'/g, \'&#39;\');\n}',
        "description": "A função escAttr() apenas escapa aspas, mas não escapa < e >. Se um valor de certificado (issuer, commonName) contiver tags HTML, pode ser injetado em atributos title.",
        "impact": "XSS armazenado potencial se um certificado malicioso for monitorado. O atacante precisaria controlar um servidor com certificado adulterado.",
        "fix": "Adicionar escape de < e > na função escAttr(): return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/\"/g, '&quot;').replace(/'/g, '&#39;');",
        "exploitability": "Requer que o usuário monitore um host com certificado malicioso controlado pelo atacante. Baixa probabilidade em uso normal."
    },
    {
        "id": "SEC-003",
        "category": "INPUTS SEM TRATAMENTO (XSS)",
        "severity": "baixa",
        "title": "Valor cert.port não é escapado no modal",
        "file": "internal/api/static/app.js",
        "line": "251",
        "code": "'<span class=\"detail-value\">' + cert.port + '</span>'",
        "description": "O valor cert.port é inserido diretamente no HTML sem escaping. Embora port seja um número inteiro vindo do backend, a falta de escaping consistente é uma prática insegura.",
        "impact": "Se o backend for comprometido ou houver um bug que permita string em port, poderia resultar em XSS.",
        "fix": "Usar esc() para todos os valores: esc(String(cert.port))",
        "exploitability": "Impraticável no estado atual, pois port é validado como inteiro no backend. É uma questão de defesa em profundidade."
    },
    {
        "id": "SEC-004",
        "category": "CHAVES EXPOSTAS",
        "severity": "baixa",
        "title": "Ausência de validação de startup para API key padrão",
        "file": "internal/cmd/serve.go",
        "line": "202-205",
        "code": 'apiToken := apiKeyFlag\nif apiToken == "" {\n    apiToken = cfg.APIKey\n}',
        "description": "O servidor inicia sem API key se nenhuma for configurada. Não há validação de startup que avise ou rejeite configuração sem autenticação em ambientes de produção.",
        "impact": "Deploy acidental sem autenticação pode expor dados de certificados a qualquer pessoa com acesso à rede.",
        "fix": "Adicionar warning no startup se apiToken == \"\" em ambiente production, ou exigir configuração explícita via flag --allow-insecure.",
        "exploitability": "Requer misconfiguration no deploy. O comportamento é documentado, mas pode passar despercebido."
    },
    {
        "id": "SEC-005",
        "category": "CHAVES EXPOSTAS",
        "severity": "informativa",
        "title": "Kubernetes Deployment não configura API key como Secret",
        "file": "kubernetes/Deployment.yml",
        "line": "28-48",
        "code": "env:\n  - name: ENVIRONMENT\n    valueFrom:\n      configMapKeyRef: ...",
        "description": "O manifest Kubernetes não demonstra como configurar a API key via Secret. Usuários podem não perceber que precisam criar um Secret separado.",
        "impact": "Documentação insuficiente pode levar a deploys sem autenticação.",
        "fix": "Adicionar exemplo de Secret e volume mount no Deployment.yml ou na documentação.",
        "exploitability": "Não é uma vulnerabilidade direta, mas uma lacuna de documentação que pode levar a misconfiguration."
    },
]

STRENGTHS = [
    {
        "title": "Rate limiting implementado",
        "description": "Token bucket com 100 req/s e burst 200 previne abuso e DoS básico.",
        "evidence": "internal/api/api.go:24-58, 104"
    },
    {
        "title": "Security headers configurados",
        "description": "X-Content-Type-Options: nosniff e X-Frame-Options: DENY previnem clickjacking e MIME sniffing.",
        "evidence": "internal/api/api.go:327-328"
    },
    {
        "title": "XSS mitigado com esc() consistente",
        "description": "A função esc() usa createTextNode() do DOM para escapar HTML de forma segura na maioria dos pontos.",
        "evidence": "internal/api/static/app.js:553-557"
    },
    {
        "title": "Container Docker roda como non-root",
        "description": "Usuário appuser (UID 1000) reduz impacto de comprometimento do container.",
        "evidence": "Dockerfile:12, 19"
    },
    {
        "title": "Timeout de contexto em health checks",
        "description": "Health check usa context.WithTimeout(5s) previne hang em hosts inacessíveis.",
        "evidence": "internal/api/api.go:260-261"
    },
    {
        "title": "Hot-reload seguro com atomic.Value",
        "description": "Swap atômico do handler previne race conditions durante reload de configuração.",
        "evidence": "internal/cmd/serve.go:62-63, 151"
    },
    {
        "title": "CSV export com UTF-8 BOM",
        "description": "BOM UTF-8 (0xEF, 0xBB, 0xBF) garante compatibilidade com Excel para caracteres especiais.",
        "evidence": "internal/api/api.go:187-190"
    },
]

def color_to_hex(color):
    """Convert reportlab HexColor to matplotlib-compatible hex string."""
    return f"#{int(color.red*255):02x}{int(color.green*255):02x}{int(color.blue*255):02x}"

def create_charts(output_dir):
    """Create charts for the report."""
    # Severity distribution (donut chart)
    severity_counts = {"Crítica": 0, "Alta": 0, "Média": 1, "Baixa": 3, "Informativa": 1}
    colors = [color_to_hex(CRITICAL), color_to_hex(HIGH), color_to_hex(MEDIUM), color_to_hex(LOW), color_to_hex(INFO)]
    labels = list(severity_counts.keys())
    sizes = list(severity_counts.values())

    fig, ax = plt.subplots(figsize=(6, 4))
    wedges, texts = ax.pie(sizes, colors=colors, startangle=90,
                           wedgeprops=dict(width=0.5))
    ax.legend(wedges, [f"{l} ({s})" for l, s in zip(labels, sizes)],
              loc="center left", bbox_to_anchor=(1, 0, 0.5, 1))
    ax.set_title("Distribuição por Severidade", fontsize=12, fontweight='bold')
    plt.tight_layout()
    plt.savefig(os.path.join(output_dir, 'severity_chart.png'), dpi=150, bbox_inches='tight')
    plt.close()

    # Category distribution (bar chart)
    categories = {}
    for f in FINDINGS:
        cat = f["category"]
        categories[cat] = categories.get(cat, 0) + 1

    fig, ax = plt.subplots(figsize=(8, 4))
    bars = ax.barh(list(categories.keys()), list(categories.values()),
                   color=[color_to_hex(MEDIUM)] * len(categories))
    ax.set_xlabel("Número de Achados")
    ax.set_title("Distribuição por Categoria", fontsize=12, fontweight='bold')
    for bar, val in zip(bars, categories.values()):
        ax.text(bar.get_width() + 0.1, bar.get_y() + bar.get_height()/2,
                str(val), va='center')
    plt.tight_layout()
    plt.savefig(os.path.join(output_dir, 'category_chart.png'), dpi=150, bbox_inches='tight')
    plt.close()

    return os.path.join(output_dir, 'severity_chart.png'), os.path.join(output_dir, 'category_chart.png')

def escape_html(text):
    """Escape HTML special characters for reportlab Paragraph."""
    return (text
            .replace('&', '&amp;')
            .replace('<', '&lt;')
            .replace('>', '&gt;')
            .replace('"', '&quot;')
            .replace("'", '&#39;'))

def header_footer(canvas, doc):
    """Add header and footer to each page."""
    canvas.saveState()
    # Header
    canvas.setFont('Helvetica-Bold', 9)
    canvas.setFillColor(TEXT_SECONDARY)
    canvas.drawString(2*cm, A4[1] - 1.2*cm, f"Relatório de Auditoria de Segurança — {PROJECT_NAME}")
    canvas.drawRightString(A4[0] - 2*cm, A4[1] - 1.2*cm, AUDIT_DATE)
    canvas.setStrokeColor(BORDER)
    canvas.line(2*cm, A4[1] - 1.4*cm, A4[0] - 2*cm, A4[1] - 1.4*cm)

    # Footer
    canvas.setFont('Helvetica', 8)
    canvas.setFillColor(TEXT_SECONDARY)
    canvas.drawString(2*cm, 1.2*cm, f"certificate-validate — Auditoria de Segurança")
    canvas.drawRightString(A4[0] - 2*cm, 1.2*cm, f"Página {doc.page}")
    canvas.line(2*cm, 1.5*cm, A4[0] - 2*cm, 1.5*cm)
    canvas.restoreState()

def first_page(canvas, doc):
    """Custom first page (cover)."""
    canvas.saveState()
    # Background gradient effect
    canvas.setFillColor(HexColor('#1F2937'))
    canvas.rect(0, A4[1] - 8*cm, A4[0], 8*cm, fill=1, stroke=0)

    # Title
    canvas.setFillColor(white)
    canvas.setFont('Helvetica-Bold', 28)
    canvas.drawCentredString(A4[0]/2, A4[1] - 4*cm, "Relatório de Auditoria")
    canvas.drawCentredString(A4[0]/2, A4[1] - 5.2*cm, "de Segurança")

    # Subtitle
    canvas.setFont('Helvetica', 14)
    canvas.setFillColor(HexColor('#9CA3AF'))
    canvas.drawCentredString(A4[0]/2, A4[1] - 6.5*cm, PROJECT_NAME)

    # Date
    canvas.setFillColor(TEXT_PRIMARY)
    canvas.setFont('Helvetica', 12)
    canvas.drawCentredString(A4[0]/2, A4[1] - 10*cm, AUDIT_DATE)

    # Scope
    canvas.setFont('Helvetica', 10)
    canvas.setFillColor(TEXT_SECONDARY)
    canvas.drawCentredString(A4[0]/2, A4[1] - 11.5*cm, SCOPE)

    canvas.restoreState()

def generate_report():
    """Generate the complete PDF report."""
    output_dir = os.path.dirname(os.path.abspath(__file__))
    output_file = os.path.join(output_dir, 'relatorio-auditoria-seguranca.pdf')

    # Create charts
    severity_chart, category_chart = create_charts(output_dir)

    # Create document
    doc = SimpleDocTemplate(
        output_file,
        pagesize=A4,
        rightMargin=2*cm,
        leftMargin=2*cm,
        topMargin=2.5*cm,
        bottomMargin=2.5*cm
    )

    # Styles
    styles = getSampleStyleSheet()
    styles.add(ParagraphStyle(
        'CustomTitle',
        parent=styles['Title'],
        fontSize=24,
        spaceAfter=30,
        textColor=TEXT_PRIMARY
    ))
    styles.add(ParagraphStyle(
        'SectionTitle',
        parent=styles['Heading1'],
        fontSize=16,
        spaceBefore=20,
        spaceAfter=10,
        textColor=TEXT_PRIMARY
    ))
    styles.add(ParagraphStyle(
        'SubsectionTitle',
        parent=styles['Heading2'],
        fontSize=13,
        spaceBefore=15,
        spaceAfter=8,
        textColor=TEXT_PRIMARY
    ))
    styles.add(ParagraphStyle(
        'CustomBody',
        parent=styles['Normal'],
        fontSize=10,
        leading=14,
        spaceAfter=10,
        textColor=TEXT_PRIMARY,
        alignment=TA_JUSTIFY
    ))
    styles.add(ParagraphStyle(
        'CodeBlock',
        parent=styles['Code'],
        fontSize=8,
        leading=10,
        leftIndent=10,
        rightIndent=10,
        spaceBefore=5,
        spaceAfter=5,
        backColor=HexColor('#F3F4F6'),
        borderColor=BORDER,
        borderWidth=1,
        borderPadding=5
    ))

    story = []

    # Executive Summary
    story.append(Paragraph("Resumo Executivo", styles['SectionTitle']))
    story.append(Spacer(1, 0.3*cm))

    summary_text = f"""
    Esta auditoria de segurança foi realizada no projeto <b>{PROJECT_NAME}</b> em {AUDIT_DATE}.
    O escopo incluiu análise de código-fonte, configurações, deploy files e frontend, adaptando
    as 5 categorias de auditoria à stack específica do projeto (Go, Cobra CLI, net/http API,
    frontend vanilla JS embutido).
    <br/><br/>
    <b>Total de achados:</b> {len(FINDINGS)}
    <br/>
    <b>Críticos:</b> 0 | <b>Altos:</b> 0 | <b>Médios:</b> 1 | <b>Baixos:</b> 3 | <b>Informativos:</b> 1
    <br/><br/>
    O projeto demonstra boas práticas de segurança, incluindo rate limiting, security headers,
    container não-root, e escaping consistente no frontend. Os achados identificados são
    principalmente de severidade baixa e informativa, relacionados a melhorias de defesa em
    profundidade e documentação.
    """
    story.append(Paragraph(summary_text, styles['CustomBody']))
    story.append(Spacer(1, 0.5*cm))

    # Charts
    story.append(Image(severity_chart, width=12*cm, height=6*cm))
    story.append(Spacer(1, 0.3*cm))
    story.append(Image(category_chart, width=14*cm, height=5*cm))
    story.append(PageBreak())

    # Strengths
    story.append(Paragraph("Pontos Fortes", styles['SectionTitle']))
    story.append(Spacer(1, 0.3*cm))

    for i, strength in enumerate(STRENGTHS, 1):
        strength_text = f"""
        <b>{i}. {strength['title']}</b><br/>
        {strength['description']}<br/>
        <font color="#6B7280" size="8"><i>Evidência: {strength['evidence']}</i></font>
        """
        story.append(Paragraph(strength_text, styles['CustomBody']))
        story.append(Spacer(1, 0.2*cm))

    story.append(PageBreak())

    # Findings
    story.append(Paragraph("Achados Detalhados", styles['SectionTitle']))
    story.append(Spacer(1, 0.3*cm))

    for finding in FINDINGS:
        severity_colors = {
            "critica": CRITICAL,
            "alta": HIGH,
            "media": MEDIUM,
            "baixa": LOW,
            "informativa": INFO
        }
        sev_color = severity_colors.get(finding["severity"], INFO)

        finding_header = f"""
        <font color="{sev_color}"><b>[{finding['severity'].upper()}]</b></font>
        {finding['id']}: {finding['title']}
        """
        story.append(Paragraph(finding_header, styles['SubsectionTitle']))

        finding_details = f"""
        <b>Categoria:</b> {finding['category']}<br/>
        <b>Arquivo:</b> {finding['file']}:{finding['line']}<br/>
        <b>Descrição:</b> {finding['description']}<br/>
        <b>Impacto:</b> {finding['impact']}<br/>
        <b>Explorabilidade:</b> {finding['exploitability']}<br/>
        <b>Correção sugerida:</b> {finding['fix']}
        """
        story.append(Paragraph(finding_details, styles['CustomBody']))

        story.append(Paragraph("<b>Código:</b>", styles['CustomBody']))
        story.append(Paragraph(escape_html(finding['code']).replace('\n', '<br/>').replace(' ', '&nbsp;'), styles['CodeBlock']))
        story.append(Spacer(1, 0.4*cm))

    story.append(PageBreak())

    # Recommendations
    story.append(Paragraph("Recomendações Priorizadas", styles['SectionTitle']))
    story.append(Spacer(1, 0.3*cm))

    recommendations = [
        ("P1", "Alta", "Implementar comparação constante para API key usando crypto/subtle.ConstantTimeCompare"),
        ("P2", "Média", "Melhorar função escAttr() para escapar todos os caracteres especiais HTML"),
        ("P3", "Baixa", "Adicionar escaping consistente em todos os valores do frontend (incluindo cert.port)"),
        ("P4", "Baixa", "Adicionar warning no startup quando API não tem autenticação configurada"),
        ("P5", "Informativa", "Documentar configuração de API key via Kubernetes Secret nos manifests de exemplo"),
    ]

    for priority, severity, rec in recommendations:
        rec_text = f"<b>[{priority}]</b> <i>({severity})</i> {rec}"
        story.append(Paragraph(rec_text, styles['CustomBody']))

    story.append(PageBreak())

    # GitHub Issues
    story.append(Paragraph("Issues para o GitHub", styles['SectionTitle']))
    story.append(Spacer(1, 0.3*cm))
    story.append(Paragraph("Abaixo estão os textos completos das issues prontas para copiar e colar no GitHub.", styles['CustomBody']))
    story.append(Spacer(1, 0.5*cm))

    # Group related findings
    issues = [
        {
            "number": 1,
            "title": "[Segurança] Comparação de API key vulnerável a timing attack",
            "labels": "security, medium",
            "findings": ["SEC-001"],
            "body": """## Descrição

A comparação de API key no middleware de autenticação usa o operador `!=` diretamente, o que não é constante no tempo. Isso permite que um atacante determine o valor correto da API key medindo o tempo de resposta de múltiplas requisições (timing attack).

## Evidência

**Arquivo:** `internal/api/api.go:332`

```go
if r.Header.Get("X-API-Key") != h.apiToken {
```

## Impacto

Um atacante com acesso à rede pode recuperar a API key caractere por caractere através de análise estatística do tempo de resposta. O risco é maior em:
- Redes de baixa latência (mesma LAN)
- Ambientes com alta precisão de timing
- APIs com alto volume de requisições

## Sugestão de Correção

Usar `crypto/subtle.ConstantTimeCompare` para comparação segura:

```go
import "crypto/subtle"

// No middleware:
expected := []byte(h.apiToken)
provided := []byte(r.Header.Get("X-API-Key"))
if subtle.ConstantTimeCompare(expected, provided) != 1 {
    // unauthorized
}
```

## Critérios de Aceite

- [ ] Importar `crypto/subtle`
- [ ] Substituir comparação `!=` por `subtle.ConstantTimeCompare`
- [ ] Adicionar teste unitário verificando que tempos de resposta são constantes
- [ ] Documentar a mudança no CHANGELOG
"""
        },
        {
            "number": 2,
            "title": "[Segurança] Melhorar escaping de atributos HTML no frontend",
            "labels": "security, low",
            "findings": ["SEC-002", "SEC-003"],
            "body": """## Descrição

A função `escAttr()` no frontend JavaScript não escapa os caracteres `<` e `>`, apenas aspas. Além disso, alguns valores (como `cert.port`) são inseridos sem escaping em certas partes do modal.

## Evidência

**Arquivo:** `internal/api/static/app.js:559-561`

```javascript
function escAttr(str) {
    return str.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}
```

**Arquivo:** `internal/api/static/app.js:251`

```javascript
'<span class="detail-value">' + cert.port + '</span>'
```

## Impacto

XSS armazenado potencial se um certificado malicioso for monitorado. O atacante precisaria controlar um servidor com certificado adulterado contendo payloads HTML nos campos (issuer, commonName, etc).

**Probabilidade:** Baixa, pois requer:
1. Usuário monitorando host controlado pelo atacante
2. Certificados com payloads específicos nos campos X.509

## Sugestão de Correção

1. Melhorar `escAttr()` para escapar todos os caracteres especiais:

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

2. Usar `esc()` para `cert.port` e outros valores não escapados:

```javascript
'<span class="detail-value">' + esc(String(cert.port)) + '</span>'
```

## Critérios de Aceite

- [ ] Atualizar `escAttr()` para escapar `<`, `>`, e `&`
- [ ] Revisar todos os usos de `innerHTML` e garantir escaping consistente
- [ ] Adicionar teste manual com certificado contendo `<script>alert(1)</script>` nos campos
- [ ] Considerar usar `textContent` ao invés de `innerHTML` onde possível
"""
        },
        {
            "number": 3,
            "title": "[Segurança] Adicionar validação de startup para API sem autenticação",
            "labels": "security, low, documentation",
            "findings": ["SEC-004", "SEC-005"],
            "body": """## Descrição

O servidor inicia sem autenticação se nenhuma API key for configurada. Não há warning no startup ou validação que previna deploys acidentais sem autenticação em ambientes de produção.

Além disso, os manifests Kubernetes de exemplo não demonstram como configurar a API key via Secret.

## Evidência

**Arquivo:** `internal/cmd/serve.go:202-205`

```go
apiToken := apiKeyFlag
if apiToken == "" {
    apiToken = cfg.APIKey
}
```

**Arquivo:** `kubernetes/Deployment.yml` - não configura API key

## Impacto

Deploy acidental sem autenticação pode expor dados de certificados (hostnames, emissores, datas de expiração) a qualquer pessoa com acesso à rede.

**Probabilidade:** Média, especialmente em:
- Deploys rápidos sem revisão de configuração
- Ambientes de desenvolvimento promovidos para produção
- Falta de documentação clara sobre autenticação

## Sugestão de Correção

1. Adicionar warning no startup quando `apiToken == ""`:

```go
if apiToken == "" {
    slog.Warn("API server starting WITHOUT authentication. " +
              "Set api_key in config or use --api-key flag. " +
              "Use --allow-insecure to suppress this warning.")
}
```

2. Adicionar flag `--allow-insecure` para suppressir o warning explicitamente.

3. Atualizar `kubernetes/Deployment.yml` com exemplo de Secret:

```yaml
# Criar Secret:
# kubectl create secret generic certificate-validate-api-key \
#   --from-literal=API_KEY=your-secret-key

env:
  - name: CV_API_KEY
    valueFrom:
      secretKeyRef:
        name: certificate-validate-api-key
        key: API_KEY
```

4. Atualizar documentação (README, site) com seção "Security Considerations".

## Critérios de Aceite

- [ ] Adicionar warning no startup quando API key não configurada
- [ ] Adicionar flag `--allow-insecure` para suppressir warning
- [ ] Atualizar `kubernetes/Deployment.yml` com exemplo de Secret
- [ ] Adicionar seção "Security" no README.md
- [ ] Atualizar site com guia de configuração de autenticação
"""
        },
    ]

    for issue in issues:
        # Escape the body to avoid HTML parsing issues
        escaped_body = escape_html(issue['body'])
        issue_text = f"""
        <b>--- ISSUE {issue['number']} ---</b><br/>
        <b>Título:</b> {escape_html(issue['title'])}<br/>
        <b>Labels:</b> {issue['labels']}<br/>
        <br/>
        <b>Descrição:</b><br/>
        <font size="8"><pre>{escaped_body}</pre></font>
        <b>--- FIM ISSUE {issue['number']} ---</b>
        """
        story.append(Paragraph(issue_text, styles['CustomBody']))
        story.append(Spacer(1, 0.5*cm))
        story.append(HRFlowable(width="100%", thickness=1, color=BORDER))
        story.append(Spacer(1, 0.5*cm))

    # Build PDF
    doc.build(story, onFirstPage=first_page, onLaterPages=header_footer)
    print(f"Relatório gerado: {output_file}")
    return output_file

if __name__ == "__main__":
    generate_report()
