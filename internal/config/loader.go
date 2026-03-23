package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Load reads a config file (YAML/TOML/JSON) and returns the parsed Config.
func Load(path string) (*Config, error) {
	v := viper.New()

	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext == "yml" {
		ext = "yaml"
	}
	v.SetConfigType(ext)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	if err := v.ReadConfig(f); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if cfg.Backend.Type == "" {
		cfg.Backend.Type = "http"
	}

	return &cfg, nil
}

// FromURL creates a minimal config for a single HTTP backend URL.
// Used when the user runs: anyclaw --mcp https://example.com
func FromURL(url string) *Config {
	name := "api"
	// Try to derive name from hostname
	if strings.Contains(url, "://") {
		parts := strings.SplitN(url, "://", 2)
		if len(parts) == 2 {
			host := strings.SplitN(parts[1], "/", 2)[0]
			host = strings.SplitN(host, ":", 2)[0]
			host = strings.SplitN(host, ".", 2)[0]
			if host != "" {
				name = host
			}
		}
	}

	return &Config{
		Name:        name,
		Description: fmt.Sprintf("Proxy to %s", url),
		Backend: Backend{
			Type:    "http",
			BaseURL: url,
		},
		Skills: []Skill{
			{
				Name:        "request",
				Description: "Send a request to the API",
				Input: map[string]Field{
					"method": {Type: "string", Description: "HTTP method", Default: "GET"},
					"path":   {Type: "string", Description: "Request path", Required: true},
					"body":   {Type: "string", Description: "Request body (JSON)"},
				},
				Backend: SkillBackend{
					Method: "GET",
					Path:   "/",
				},
			},
		},
	}
}
