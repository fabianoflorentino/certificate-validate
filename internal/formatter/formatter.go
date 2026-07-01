package formatter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

// Formatter is the interface for formatting certificate information.
type Formatter interface {
	Format(cert *certificate.Certificate) ([]byte, error)
}

// JSONFormatter formats certificates as indented JSON.
type JSONFormatter struct {
	indent string
}

// New creates a new JSONFormatter.
func New() *JSONFormatter {
	return &JSONFormatter{indent: "  "}
}

func (f *JSONFormatter) Format(cert *certificate.Certificate) ([]byte, error) {
	data, err := json.MarshalIndent(cert, "", f.indent)
	if err != nil {
		return nil, fmt.Errorf("format certificate: %w", err)
	}
	return data, nil
}

// FormatTable formats certificates as an aligned table for CLI output.
func FormatTable(certs []*certificate.Certificate) ([]byte, error) {
	var buf bytes.Buffer

	const colWidth = 22
	header := fmt.Sprintf("%-22s %-5s %-5s %-9s %-48s %s\n",
		"Host", "Port", "Days", "Status", "Issuer", "TLS Version")
	if _, err := buf.WriteString(header); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}

	sep := fmt.Sprintf("%s %s %s %s %s %s\n",
		strings.Repeat("-", colWidth),
		strings.Repeat("-", 5),
		strings.Repeat("-", 5),
		strings.Repeat("-", 9),
		strings.Repeat("-", 48),
		strings.Repeat("-", 13))
	if _, err := buf.WriteString(sep); err != nil {
		return nil, fmt.Errorf("write separator: %w", err)
	}

	for _, c := range certs {
		if c == nil {
			continue
		}
		status := statusLabel(c.DaysLeft)
		issuer := c.Issuer
		if len(issuer) > 48 {
			issuer = issuer[:45] + "..."
		}
		line := fmt.Sprintf("%-22s %-5d %-5d %-9s %-48s %s\n",
			c.Hostname, c.Port, c.DaysLeft, status, issuer, c.TLSVersion)
		if _, err := buf.WriteString(line); err != nil {
			return nil, fmt.Errorf("write line: %w", err)
		}
	}

	return buf.Bytes(), nil
}

func statusLabel(days int) string {
	switch {
	case days <= 7:
		return "critical"
	case days <= 30:
		return "warning"
	default:
		return "good"
	}
}
