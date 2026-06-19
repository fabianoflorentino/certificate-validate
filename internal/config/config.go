package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration from settings.yml.
type Config struct {
	CheckTime  int            `yaml:"check_time"`
	AppConfigs []AppConfig    `yaml:"app_configs"`
	Hosts      []HostConfig   `yaml:"hosts"`
	Prometheus PrometheusConf `yaml:"prometheus"`
	Webhook    WebhookConf    `yaml:"webhook"`
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

// HostConfig represents a single host entry to check.
type HostConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Port string `yaml:"port"` // string to support '443' or 443 in YAML
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
