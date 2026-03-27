package pkg

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Credentials manages API keys stored in ~/.anyclaw/credentials.yaml.
type Credentials struct {
	path string
	data map[string]string
}

// LoadCredentials reads the credentials file.
func LoadCredentials() (*Credentials, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(home, ".anyclaw", "credentials.yaml")
	c := &Credentials{path: path, data: make(map[string]string)}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(raw, &c.data); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	return c, nil
}

// Get returns the API key for a package. Falls back to env var.
func (c *Credentials) Get(packageName string) string {
	if v, ok := c.data[packageName]; ok {
		return v
	}
	return ""
}

// Set stores an API key for a package and saves to disk.
func (c *Credentials) Set(packageName, value string) error {
	c.data[packageName] = value
	return c.save()
}

// Remove deletes an API key for a package and saves to disk.
func (c *Credentials) Remove(packageName string) error {
	delete(c.data, packageName)
	return c.save()
}

func (c *Credentials) save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		return err
	}

	raw, err := yaml.Marshal(c.data)
	if err != nil {
		return err
	}

	return os.WriteFile(c.path, raw, 0600) // 0600: only owner can read
}
