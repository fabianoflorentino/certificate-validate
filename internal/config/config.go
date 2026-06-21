package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/fabianoflorentino/certificate-validate/internal/checker"
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
	Name  string `yaml:"name"`
	URL   string `yaml:"url"`
	Port  string `yaml:"port"`
	Ports []int  `yaml:"ports"`
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
		for _, port := range h.PortInts() {
			hosts = append(hosts, checker.Host{
				Hostname: h.URL,
				Port:     port,
				Name:     h.Name,
			})
		}
	}
	return hosts
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
