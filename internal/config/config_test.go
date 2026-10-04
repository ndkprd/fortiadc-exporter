package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad(t *testing.T) {
	t.Setenv("ADC_KEY", "from-env")

	tests := []struct {
		name    string
		body    string
		wantErr string
		check   func(*testing.T, *Config)
	}{
		{
			name: "defaults applied",
			body: "fortiadc:\n  a:\n    url: https://x\n    api_key: k\n",
			check: func(t *testing.T, c *Config) {
				if c.ListenAddress != ":9634" || c.FortiADC["a"].VDOM != "root" {
					t.Errorf("defaults not applied: %+v", c)
				}
			},
		},
		{
			name: "api_key_env overrides api_key",
			body: "fortiadc:\n  a:\n    url: https://x\n    api_key: k\n    api_key_env: ADC_KEY\n",
			check: func(t *testing.T, c *Config) {
				if c.FortiADC["a"].APIKey != "from-env" {
					t.Errorf("api key = %q", c.FortiADC["a"].APIKey)
				}
			},
		},
		{name: "missing url", body: "fortiadc:\n  a:\n    api_key: k\n", wantErr: "url is required"},
		{name: "missing api_key", body: "fortiadc:\n  a:\n    url: https://x\n", wantErr: "api_key is required"},
		{name: "no targets", body: "listen_address: ':1'\n", wantErr: "at least one fortiadc target"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := Load(write(t, tt.body))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, c)
		})
	}
}
