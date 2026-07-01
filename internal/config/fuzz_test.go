package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func FuzzConfigParse(f *testing.F) {
	seeds := []string{
		"hosts:\n  - name: test\n    url: example.com\n    port: '443'\n",
		"hosts:\n  - name: ''\n    url: ''\n    port: 'abc'\n",
		"check_time: 0\nhosts: []\n",
		"prometheus:\n  enabled: true\n  address: ':9090'\nhosts:\n  - name: a\n    url: a.com\n    port: '443'\n",
		"history:\n  enabled: true\n  file_path: /tmp/x\n  max_entries: 0\n  max_days: 0\nhosts:\n  - name: a\n    url: a.com\n    port: '80'\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return
		}
		_, _ = cfg.Validate()
	})
}
