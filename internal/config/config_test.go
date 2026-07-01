package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPortInt(t *testing.T) {
	tests := []struct {
		name string
		port string
		want int
	}{
		{"valid port", "443", 443},
		{"string port", "8080", 8080},
		{"empty port defaults to 443", "", 443},
		{"invalid port defaults to 443", "abc", 443},
		{"zero port defaults to 443", "0", 443},
		{"negative port defaults to 443", "-1", 443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := HostConfig{Port: tt.port}
			if got := h.PortInt(); got != tt.want {
				t.Errorf("PortInt() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestPortInts(t *testing.T) {
	tests := []struct {
		name  string
		host  HostConfig
		want  []int
	}{
		{
			name:  "specific ports",
			host:  HostConfig{Ports: []int{443, 8443}},
			want:  []int{443, 8443},
		},
		{
			name:  "empty ports falls back to PortInt",
			host:  HostConfig{Port: "8080"},
			want:  []int{8080},
		},
		{
			name:  "empty ports and port both empty",
			host:  HostConfig{},
			want:  []int{443},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.host.PortInts()
			if len(got) != len(tt.want) {
				t.Fatalf("PortInts() = %v (len %d); want %v (len %d)", got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("PortInts()[%d] = %d; want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestValidate_EmptyHosts(t *testing.T) {
	cfg := &Config{}
	_, err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for empty hosts")
	}
}

func TestValidate_HostWithoutURL(t *testing.T) {
	cfg := &Config{
		Hosts: []HostConfig{{Name: "test", Port: "443"}},
	}
	_, err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for host without url")
	}
}

func TestValidate_HostWithoutName(t *testing.T) {
	cfg := &Config{
		Hosts: []HostConfig{{URL: "example.com", Port: "443"}},
	}
	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for missing host name")
	}
}

func TestValidate_InvalidPortString(t *testing.T) {
	cfg := &Config{
		Hosts: []HostConfig{{Name: "test", URL: "example.com", Port: "abc"}},
	}
	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	found := false
	for _, w := range warnings {
		if contains(w, "invalid port") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for invalid port string")
	}
}

func TestValidate_PortOutOfRange(t *testing.T) {
	cfg := &Config{
		Hosts: []HostConfig{{Name: "test", URL: "example.com", Port: "99999"}},
	}
	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	found := false
	for _, w := range warnings {
		if contains(w, "out of range") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for port out of range")
	}
}

func TestValidate_PortsFieldOutOfRange(t *testing.T) {
	cfg := &Config{
		Hosts: []HostConfig{{Name: "test", URL: "example.com", Ports: []int{443, 0, 65536}}},
	}
	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	count := 0
	for _, w := range warnings {
		if contains(w, "out of range") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 out-of-range warnings, got %d", count)
	}
}

func TestValidate_WebhookThreshold(t *testing.T) {
	cfg := &Config{
		Hosts: []HostConfig{{Name: "test", URL: "example.com", Port: "443"}},
		Webhook: WebhookConf{URL: "https://hooks.example.com", Threshold: 0},
	}
	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	found := false
	for _, w := range warnings {
		if contains(w, "webhook threshold") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for zero webhook threshold")
	}
}

func TestValidate_PrometheusNoAddress(t *testing.T) {
	cfg := &Config{
		Hosts:      []HostConfig{{Name: "test", URL: "example.com", Port: "443"}},
		Prometheus: PrometheusConf{Enabled: true},
	}
	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	found := false
	for _, w := range warnings {
		if contains(w, "prometheus") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for prometheus without address")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Hosts: []HostConfig{
			{Name: "test", URL: "example.com", Port: "443"},
		},
	}
	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(warnings), warnings)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsString(s, substr)
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestToCheckerHosts(t *testing.T) {
	tests := []struct {
		name  string
		hosts []HostConfig
		want  int
	}{
		{"single host single port", []HostConfig{{Name: "test", URL: "example.com", Port: "443"}}, 1},
		{"single host multiple ports", []HostConfig{{Name: "test", URL: "example.com", Ports: []int{443, 8443}}}, 2},
		{"multiple hosts", []HostConfig{
			{Name: "a", URL: "a.com", Port: "443"},
			{Name: "b", URL: "b.com", Port: "80"},
		}, 2},
		{"empty config", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToCheckerHosts(tt.hosts)
			if len(got) != tt.want {
				t.Errorf("ToCheckerHosts() returned %d hosts; want %d", len(got), tt.want)
			}
		})
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yml")
	if err == nil {
		t.Fatal("Load expected error for nonexistent file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.yml")
	if err := writeFile(path, "{invalid: [yaml"); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load expected error for invalid YAML, got nil")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "valid.yml")
	content := `check_time: 3600
app_configs:
  - name: test-app
    host: 0.0.0.0
    port: "5000"
hosts:
  - name: github
    url: github.com
    port: "443"
prometheus:
  enabled: true
  address: ":2112"
history:
  enabled: true
  file_path: /tmp/history.log
  max_entries: 1000
  max_days: 30
`
	if err := writeFile(path, content); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.CheckTime != 3600 {
		t.Errorf("CheckTime = %d; want 3600", cfg.CheckTime)
	}
	if len(cfg.Hosts) != 1 {
		t.Errorf("len(Hosts) = %d; want 1", len(cfg.Hosts))
	}
	if cfg.Hosts[0].URL != "github.com" {
		t.Errorf("Hosts[0].URL = %q; want %q", cfg.Hosts[0].URL, "github.com")
	}
	if !cfg.Prometheus.Enabled {
		t.Error("Prometheus.Enabled = false; want true")
	}
	if !cfg.History.Enabled {
		t.Error("History.Enabled = false; want true")
	}
}

func TestLoad_DefaultsCheckTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-checktime.yml")
	content := `hosts:
  - name: test
    url: test.com
    port: "443"
`
	if err := writeFile(path, content); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.CheckTime != 86400 {
		t.Errorf("CheckTime = %d; want default 86400", cfg.CheckTime)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
