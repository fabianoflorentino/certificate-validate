package config

import (
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/fetcher"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration from settings.yml.
// It includes settings for certificate checking, API server, Prometheus metrics,
// webhooks, history recording, and trusted CAs.
type Config struct {
	CheckTime  int            `yaml:"check_time"`
	APIKey     string         `yaml:"api_key"`
	AppConfigs []AppConfig    `yaml:"app_configs"`
	Hosts      []HostConfig   `yaml:"hosts"`
	Prometheus PrometheusConf `yaml:"prometheus"`
	Webhook    WebhookConf    `yaml:"webhook"`
	History    HistoryConf    `yaml:"history"`
	TrustedCAs []string       `yaml:"trusted_cas"`
}

// PrometheusConf controls Prometheus metrics exposition.
type PrometheusConf struct {
	Enabled bool   `yaml:"enabled"`
	Address string `yaml:"address"`
}

// WebhookConf controls webhook alert notifications.
type WebhookConf struct {
	URL       string `yaml:"url"`
	Threshold int    `yaml:"threshold"`
	Interval  int    `yaml:"interval"`
}

// HistoryConf controls local history recording.
type HistoryConf struct {
	Enabled    bool   `yaml:"enabled"`
	FilePath   string `yaml:"file_path"`
	MaxEntries int    `yaml:"max_entries"`
	MaxDays    int    `yaml:"max_days"`
}

// HostConfig represents a single host entry to check.
type HostConfig struct {
	Name       string   `yaml:"name"`
	URL        string   `yaml:"url"`
	Port       string   `yaml:"port"`
	Ports      []int    `yaml:"ports"`
	Timeout    int      `yaml:"timeout"`     // per-host dial timeout in seconds (0 = use default)
	TrustedCAs []string `yaml:"trusted_cas"` // per-host trusted CA certificate paths
}

// AppConfig represents the API application configuration.
type AppConfig struct {
	Name        string `yaml:"name"`
	Host        string `yaml:"host"`
	Port        string `yaml:"port"`
	Environment string `yaml:"environment"`
	Debug       bool   `yaml:"debug"`
}

// PortInt converts the string port to an integer. Returns 443 as the default
// if the port string is empty, non-numeric, or out of range.
func (h HostConfig) PortInt() int {
	p, err := strconv.Atoi(h.Port)
	if err != nil || p <= 0 {
		return 443
	}
	return p
}

// PortInts returns all ports for this host.
// Falls back to PortInt() if Ports is empty.
func (h HostConfig) PortInts() []int {
	if len(h.Ports) > 0 {
		return h.Ports
	}
	return []int{h.PortInt()}
}

// ToCheckerHosts converts HostConfig entries to checker.Host, expanding multiple ports.
// Each port in a host's Ports list creates a separate checker.Host entry.
// If Ports is empty, uses the single Port field.
func ToCheckerHosts(cfgHosts []HostConfig) []checker.Host {
	var hosts []checker.Host
	for _, h := range cfgHosts {
		timeout := time.Duration(h.Timeout) * time.Second
		for _, port := range h.PortInts() {
			hosts = append(hosts, checker.Host{
				Hostname: h.URL,
				Port:     port,
				Name:     h.Name,
				Timeout:  timeout,
			})
		}
	}
	return hosts
}

// LoadPerHostCAs reads per-host CA paths from HostConfig entries.
// Returns a map of host URL to certificate pool.
// Returns an error if any CA file cannot be read or parsed.
func LoadPerHostCAs(hosts []HostConfig) (map[string]*x509.CertPool, error) {
	m := make(map[string]*x509.CertPool)
	for _, h := range hosts {
		if len(h.TrustedCAs) == 0 {
			continue
		}
		pool, err := fetcher.LoadRootCAs(h.TrustedCAs)
		if err != nil {
			return nil, fmt.Errorf("load per-host CAs for %s: %w", h.URL, err)
		}
		m[h.URL] = pool
	}
	return m, nil
}

// Validate checks the configuration for common issues.
// Returns a list of warnings (non-fatal) and an error if the configuration is invalid.
// Warnings include missing host names, invalid ports, and empty CA paths.
// Errors include missing hosts and invalid configuration values.
func (cfg *Config) Validate() ([]string, error) {
	var warnings []string

	if len(cfg.Hosts) == 0 {
		return warnings, errors.New("no hosts configured")
	}

	for i, h := range cfg.Hosts {
		if h.URL == "" {
			return warnings, fmt.Errorf("host[%d]: url is required", i)
		}
		if h.Name == "" {
			warnings = append(warnings, fmt.Sprintf("host[%d]: name is empty, using url as name", i))
		}
		if h.Port != "" {
			p, err := strconv.Atoi(h.Port)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("host[%d]: invalid port %q, defaulting to 443", i, h.Port))
			} else if p < 1 || p > 65535 {
				warnings = append(warnings, fmt.Sprintf("host[%d]: port %d out of range (1-65535), defaulting to 443", i, p))
			}
		}
		for j, p := range h.Ports {
			if p < 1 || p > 65535 {
				warnings = append(warnings, fmt.Sprintf("host[%d].ports[%d]: port %d out of range (1-65535)", i, j, p))
			}
		}
		if h.Timeout < 0 {
			warnings = append(warnings, fmt.Sprintf("host[%d]: timeout %d is negative, using default", i, h.Timeout))
		}
		if len(h.TrustedCAs) > 0 {
			for j, ca := range h.TrustedCAs {
				if ca == "" {
					warnings = append(warnings, fmt.Sprintf("host[%d].trusted_cas[%d]: empty path", i, j))
				}
			}
		}
	}

	if cfg.Webhook.URL != "" && cfg.Webhook.Threshold <= 0 {
		warnings = append(warnings, "webhook threshold must be > 0, using default")
	}

	if cfg.Prometheus.Enabled && cfg.Prometheus.Address == "" {
		warnings = append(warnings, "prometheus enabled but no address set, using default")
	}

	return warnings, nil
}

// applyEnvOverrides overrides config fields with environment variables.
// Uses the CV_ prefix. Only overrides if the env var is set and non-empty.
func (cfg *Config) applyEnvOverrides() {
	if v := os.Getenv("CV_CHECK_TIME"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			cfg.CheckTime = i
		}
	}
	if v := os.Getenv("CV_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("CV_APP_HOST"); v != "" && len(cfg.AppConfigs) > 0 {
		cfg.AppConfigs[0].Host = v
	}
	if v := os.Getenv("CV_APP_PORT"); v != "" && len(cfg.AppConfigs) > 0 {
		cfg.AppConfigs[0].Port = v
	}
	if v := os.Getenv("CV_PROMETHEUS_ENABLED"); v != "" {
		cfg.Prometheus.Enabled = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("CV_PROMETHEUS_ADDRESS"); v != "" {
		cfg.Prometheus.Address = v
	}
	if v := os.Getenv("CV_WEBHOOK_URL"); v != "" {
		cfg.Webhook.URL = v
	}
	if v := os.Getenv("CV_WEBHOOK_THRESHOLD"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Webhook.Threshold = i
		}
	}
	if v := os.Getenv("CV_WEBHOOK_INTERVAL"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Webhook.Interval = i
		}
	}
	if v := os.Getenv("CV_HISTORY_ENABLED"); v != "" {
		cfg.History.Enabled = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("CV_HISTORY_FILE_PATH"); v != "" {
		cfg.History.FilePath = v
	}
	if v := os.Getenv("CV_HISTORY_MAX_ENTRIES"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			cfg.History.MaxEntries = i
		}
	}
	if v := os.Getenv("CV_HISTORY_MAX_DAYS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			cfg.History.MaxDays = i
		}
	}
	if v := os.Getenv("CV_TRUSTED_CAS"); v != "" {
		cfg.TrustedCAs = strings.Split(v, ",")
	}
}

// Load reads and parses a YAML configuration file.
// Applies environment variable overrides (CV_ prefix) after loading.
// Returns the configuration or an error if the file cannot be read or parsed.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.CheckTime <= 0 {
		cfg.CheckTime = 86400
	}

	cfg.applyEnvOverrides()

	return &cfg, nil
}
