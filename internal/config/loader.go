package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads an OpenAPI spec file (YAML) and returns the parsed Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}

	var spec openAPISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if spec.OpenAPI == "" {
		return nil, fmt.Errorf("invalid config %s: missing 'openapi' field (only OpenAPI 3.x format is supported)", path)
	}

	return fromOpenAPI(&spec), nil
}

// FromURL creates a minimal config for a single HTTP backend URL.
func FromURL(rawURL string) *Config {
	name := "api"
	if strings.Contains(rawURL, "://") {
		parts := strings.SplitN(rawURL, "://", 2)
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
		Description: fmt.Sprintf("Proxy to %s", rawURL),
		Backend: Backend{
			Type:    "http",
			BaseURL: rawURL,
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
