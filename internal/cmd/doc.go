// Package cmd provides the command-line interface for certificate-validate.
//
// The package uses cobra for command parsing and defines the following commands:
//
//   - check: Check certificates from configured hosts
//   - serve: Start the HTTP API server
//   - export: Export certificate data to JSON or CSV
//   - k8s monitor: Monitor Kubernetes TLS certificates
//   - version: Print build information
//   - completion: Generate shell completion scripts
//
// # Architecture
//
// Each command is defined as a cobra.Command with its own flags and RunE function.
// Commands share common configuration via persistent flags (e.g., --config, --log).
//
// # Usage
//
// The package is typically invoked via the main package:
//
//	package main
//
//	import "github.com/fabianoflorentino/certificate-validate/internal/cmd"
//
//	func main() {
//		cmd.Execute()
//	}
package cmd
