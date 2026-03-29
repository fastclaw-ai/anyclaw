package site

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// SiteArg describes an argument for a site adapter command.
type SiteArg struct {
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Default     string `json:"default"`
}

// SiteAdapter is a browser-based site command adapter.
type SiteAdapter struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Domain      string             `json:"domain"`
	Args        map[string]SiteArg `json:"args"`
	ReadOnly    bool               `json:"readOnly"`
	Example     string             `json:"example"`
	Code        string             `json:"-"` // the async function body
}

// Platform returns the platform part of the name (e.g. "zhihu" from "zhihu/hot")
func (a *SiteAdapter) Platform() string {
	parts := strings.SplitN(a.Name, "/", 2)
	return parts[0]
}

// Command returns the command part of the name (e.g. "hot" from "zhihu/hot")
func (a *SiteAdapter) Command() string {
	parts := strings.SplitN(a.Name, "/", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

var metaRe = regexp.MustCompile(`(?s)/\*\s*@meta\s*(\{.*?\})\s*\*/`)

// ParseAdapter reads a .js file and parses it as a site adapter.
func ParseAdapter(path string) (*SiteAdapter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read adapter file: %w", err)
	}
	return ParseAdapterData(data)
}

// ParseAdapterData parses a JS site adapter from raw bytes.
func ParseAdapterData(data []byte) (*SiteAdapter, error) {
	src := string(data)

	// Extract @meta JSON
	m := metaRe.FindStringSubmatch(src)
	if m == nil {
		return nil, fmt.Errorf("no @meta block found")
	}

	var adapter SiteAdapter
	if err := json.Unmarshal([]byte(m[1]), &adapter); err != nil {
		return nil, fmt.Errorf("parse @meta JSON: %w", err)
	}

	// Extract function code (everything after the meta comment)
	codeStart := strings.Index(src, m[0]) + len(m[0])
	code := strings.TrimSpace(src[codeStart:])
	adapter.Code = code

	if adapter.Name == "" {
		return nil, fmt.Errorf("adapter missing name in @meta")
	}

	return &adapter, nil
}
