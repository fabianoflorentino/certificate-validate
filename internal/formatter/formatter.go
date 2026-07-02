package formatter

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
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
	header := fmt.Sprintf("%-22s %-5s %-5s %-9s %-10s %-48s %s\n",
		"Host", "Port", "Days", "Status", "Revoc", "Issuer", "TLS Version")
	if _, err := buf.WriteString(header); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}

	sep := fmt.Sprintf("%s %s %s %s %s %s %s\n",
		strings.Repeat("-", colWidth),
		strings.Repeat("-", 5),
		strings.Repeat("-", 5),
		strings.Repeat("-", 9),
		strings.Repeat("-", 10),
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
		line := fmt.Sprintf("%-22s %-5d %-5d %-9s %-10s %-48s %s\n",
			c.Hostname, c.Port, c.DaysLeft, status, c.RevocationStatus, issuer, c.TLSVersion)
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

// FormatJSON formats multiple certificates as a JSON array.
func FormatJSON(certs []*certificate.Certificate) ([]byte, error) {
	data, err := json.MarshalIndent(certs, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("format certificates: %w", err)
	}
	return data, nil
}

// FormatCSV formats certificates as CSV with a header row.
func FormatCSV(certs []*certificate.Certificate) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	header := []string{"hostname", "port", "commonName", "issuer", "notBefore", "notAfter", "daysLeft", "revocationStatus", "tlsVersion", "cipherSuite"}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	for _, c := range certs {
		if c == nil {
			continue
		}
		rec := []string{
			c.Hostname,
			strconv.Itoa(c.Port),
			c.CommonName,
			c.Issuer,
			c.NotBefore,
			c.NotAfter,
			strconv.Itoa(c.DaysLeft),
			string(c.RevocationStatus),
			c.TLSVersion,
			c.CipherSuite,
		}
		if err := w.Write(rec); err != nil {
			return nil, fmt.Errorf("write csv record: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv flush: %w", err)
	}
	return buf.Bytes(), nil
}
