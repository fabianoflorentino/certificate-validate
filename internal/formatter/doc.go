// Package formatter provides output formatting for certificate information.
//
// The Formatter interface defines the contract for formatting certificate data.
// The package includes implementations for JSON, table, and CSV output formats.
//
// # Formats
//
//   - JSON: indented JSON output suitable for programmatic consumption
//   - Table: aligned text table for CLI display
//   - CSV: comma-separated values for spreadsheet import
//
// # Usage
//
// Format a certificate as JSON:
//
//	f := formatter.New()
//	data, err := f.Format(cert)
//
// Format multiple certificates as a table:
//
//	data, err := formatter.FormatTable(certs)
//
// Export certificates as CSV:
//
//	data, err := formatter.FormatCSV(certs)
package formatter
