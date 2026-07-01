package config

import (
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/fetcher"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration from settings.yml.
type Config struct {
	CheckTime  int            `yaml:"check_time"`
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

// PortInt converts the string port to an integer. Returns 443 on error.
func (h HostConfig) PortInt() int {
	p, err := strconv.Atoi(h.Port)
	if err != nil || p <= 0 {
		return 443
	}
	return p
}

// PortInts returns all ports for this host. Falls back to PortInt() if Ports is empty.
func (h HostConfig) PortInts() []int {
	if len(h.Ports) > 0 {
		return h.Ports
	}
	return []int{h.PortInt()}
}

// ToCheckerHosts converts HostConfig entries to checker.Host, expanding multiple ports.
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
// Returns a list of warnings and an error if the configuration is invalid.
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

// Load reads and parses a YAML configuration file.
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

	return &cfg, nil
}
