package pkg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Manifest describes a package in the anyclaw app store.
type Manifest struct {
	Anyclaw     string    `yaml:"anyclaw,omitempty"`     // format version, e.g. "1.0"
	Name        string    `yaml:"name"`
	Version     string    `yaml:"version,omitempty"`
	Description string    `yaml:"description"`
	Author      string    `yaml:"author,omitempty"`
	Adapter     string    `yaml:"adapter,omitempty"`     // auto-inferred if empty: "openapi", "cli", "script", "pipeline"
	Source      string    `yaml:"source,omitempty"`      // origin marker, e.g. "local:web.yaml"
	Commands    []Command `yaml:"commands"`
}

// InferAdapter returns the effective adapter type.
// If Adapter is set explicitly, use it. Otherwise infer from command contents.
func (m *Manifest) InferAdapter() string {
	if m.Adapter != "" {
		return m.Adapter
	}
	for _, cmd := range m.Commands {
		if cmd.HTTP != nil {
			return "openapi"
		}
		if cmd.Run != "" {
			return "cli"
		}
		if cmd.Script != nil {
			return "script"
		}
		if len(cmd.Pipeline) > 0 {
			return "pipeline"
		}
	}
	return "cli"
}

// Command describes a single callable action within a package.
type Command struct {
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description"`
	Args        map[string]Arg           `yaml:"args"`
	Run         string                   `yaml:"run"`      // shell command template for cli/script adapter
	HTTP        *HTTPConfig              `yaml:"http"`      // for openapi adapter
	Script      *ScriptConfig            `yaml:"script"`    // for script adapter
	Pipeline    []map[string]any         `yaml:"pipeline"`  // for pipeline adapter
	Columns     []string                 `yaml:"columns,omitempty"` // column order for table output
}

// Arg describes a command argument.
type Arg struct {
	Type        string `yaml:"type"`        // "string", "int", "bool"
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default"`
	Description string `yaml:"description"`
	Short       string `yaml:"short"`       // single-char short flag (e.g. "a" for -a)
}

// HTTPConfig describes how to call an HTTP API for a command.
type HTTPConfig struct {
	BaseURL string `yaml:"base_url"`
	Method  string `yaml:"method"`
	Path    string `yaml:"path"`
	Auth    *Auth  `yaml:"auth"` // optional authentication
}

// Auth describes authentication for HTTP requests.
type Auth struct {
	Type     string `yaml:"type"`      // "bearer", "basic", "api_key"
	TokenEnv string `yaml:"token_env"` // credentials key (package name)
	Header   string `yaml:"header"`    // custom header name (for api_key type)
	Prefix   string `yaml:"prefix"`    // token prefix (e.g. "Key", "Bearer")
}

// ScriptConfig describes how to run a script for a command.
type ScriptConfig struct {
	Runtime string `yaml:"runtime"`          // "python", "node", "bb-site"
	File    string `yaml:"file"`
	Code    string `yaml:"code"`             // inline script content
	Domain  string `yaml:"domain,omitempty"` // for bb-site runtime: domain to navigate to
}

// LoadManifest reads a manifest.yaml file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	return LoadManifestData(data)
}

// LoadManifestData parses manifest from raw YAML data.
func LoadManifestData(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Name == "" {
		return nil, fmt.Errorf("manifest missing name")
	}
	return &m, nil
}
