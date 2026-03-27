package registry

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultRegistryURL = "https://raw.githubusercontent.com/fastclaw-ai/anyclaw/main/registry/index.yaml"

// Package describes a package available in the registry.
type Package struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Type        string   `yaml:"type"`   // "openapi", "pipeline", "cli"
	Source      string   `yaml:"source"` // URL to install from
	Tags        []string `yaml:"tags"`
}

// Index is the top-level registry index.
type Index struct {
	Packages []Package `yaml:"packages"`
}

// FetchIndex fetches and parses the registry index.
func FetchIndex() (*Index, error) {
	source := DefaultRegistryURL
	if env := os.Getenv("ANYCLAW_REGISTRY_URL"); env != "" {
		source = env
	}

	var data []byte
	var err error

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		resp, httpErr := http.Get(source)
		if httpErr != nil {
			return nil, fmt.Errorf("fetch registry: %w", httpErr)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("fetch registry: HTTP %d", resp.StatusCode)
		}

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read registry: %w", err)
		}
	} else {
		// Local file path
		data, err = os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("read registry: %w", err)
		}
	}

	var idx Index
	if err := yaml.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse registry index: %w", err)
	}

	return &idx, nil
}

// Lookup finds a package by exact name.
func (idx *Index) Lookup(name string) (*Package, bool) {
	for i := range idx.Packages {
		if strings.EqualFold(idx.Packages[i].Name, name) {
			return &idx.Packages[i], true
		}
	}
	return nil, false
}

// Search finds packages matching a keyword (case-insensitive substring match
// against name, description, and tags).
func (idx *Index) Search(keyword string) []Package {
	kw := strings.ToLower(keyword)
	var results []Package
	for _, p := range idx.Packages {
		if strings.Contains(strings.ToLower(p.Name), kw) ||
			strings.Contains(strings.ToLower(p.Description), kw) ||
			matchTags(p.Tags, kw) {
			results = append(results, p)
		}
	}
	return results
}

func matchTags(tags []string, keyword string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), keyword) {
			return true
		}
	}
	return false
}
