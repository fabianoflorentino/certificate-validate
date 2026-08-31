// Package config provides application configuration loading and management.
//
// The package handles loading configuration from YAML files (settings.yml),
// environment variables (CV_ prefix), and command-line flags. It defines the
// configuration structures for hosts, API settings, Prometheus metrics, webhooks,
// and history recording.
//
// # Configuration Sources
//
// Configuration is loaded in the following priority order (highest to lowest):
//
//  1. Command-line flags
//  2. Environment variables (CV_ prefix)
//  3. Configuration file (settings.yml)
//  4. Default values
//
// # Host Configuration
//
// Hosts can be configured with multiple ports, per-host timeouts, and per-host
// trusted CA certificates for internal/mutual TLS scenarios.
//
// # Usage
//
// Load configuration from a file:
//
//	cfg, err := config.Load("settings.yml")
//
// Convert host configs to checker hosts:
//
//	hosts := config.ToCheckerHosts(cfg.Hosts)
package config
