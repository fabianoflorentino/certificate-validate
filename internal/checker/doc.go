// Package checker provides certificate checking orchestration with concurrent
// fetching and formatting capabilities.
//
// The Checker type coordinates between a Fetcher (retrieves certificates from
// hosts) and a Formatter (formats output). It supports parallel certificate
// checking across multiple hosts with configurable concurrency limits.
//
// # Interfaces
//
// The package defines key interfaces for dependency injection:
//
//   - Fetcher: retrieves certificate information from a host
//   - Formatter: formats certificate data for output
//   - CertChecker: interface for checking certificate expiration
//
// Consumers depend on the CertChecker interface, not the concrete Checker type,
// enabling easy testing and alternative implementations.
//
// # Usage
//
// Create a checker and check multiple hosts:
//
//	chk := checker.New(fetcher, formatter)
//	certs, errs := chk.CheckAll(ctx, hosts, 10) // max 10 parallel
package checker
