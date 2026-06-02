package formatter

import (
	"encoding/json"
	"fmt"

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
