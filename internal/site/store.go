package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store manages site adapters from community and private directories.
type Store struct {
	bbSitesDir string // ~/.anyclaw/bb-sites
	privateDir string // ~/.anyclaw/sites
}

// NewStore creates a Store at default locations.
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	s := &Store{
		bbSitesDir: filepath.Join(home, ".anyclaw", "bb-sites"),
		privateDir: filepath.Join(home, ".anyclaw", "sites"),
	}
	// Ensure private dir exists
	if err := os.MkdirAll(s.privateDir, 0755); err != nil {
		return nil, fmt.Errorf("create private site dir: %w", err)
	}
	return s, nil
}

// BBSitesDir returns the community adapters directory.
func (s *Store) BBSitesDir() string { return s.bbSitesDir }

// PrivateDir returns the private adapters directory.
func (s *Store) PrivateDir() string { return s.privateDir }

// Get finds an adapter by "platform/command" name. Private adapters take priority.
func (s *Store) Get(name string) (*SiteAdapter, error) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid adapter name %q: expected platform/command", name)
	}
	platform, command := parts[0], parts[1]

	// Check private first
	for _, dir := range []string{s.privateDir, s.bbSitesDir} {
		path := filepath.Join(dir, platform, command+".js")
		if _, err := os.Stat(path); err == nil {
			return ParseAdapter(path)
		}
	}

	return nil, fmt.Errorf("adapter %q not found. Run: anyclaw site update", name)
}

// List returns all available adapters (private overrides community by name).
func (s *Store) List() ([]*SiteAdapter, error) {
	seen := make(map[string]*SiteAdapter)

	// Load community first, then private (private overrides)
	for _, dir := range []string{s.bbSitesDir, s.privateDir} {
		adapters, err := scanDir(dir)
		if err != nil {
			continue // skip missing dirs
		}
		for _, a := range adapters {
			seen[a.Name] = a
		}
	}

	result := make([]*SiteAdapter, 0, len(seen))
	for _, a := range seen {
		result = append(result, a)
	}
	return result, nil
}

// ListByPlatform returns adapters for a specific platform.
func (s *Store) ListByPlatform(platform string) ([]*SiteAdapter, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	var result []*SiteAdapter
	for _, a := range all {
		if a.Platform() == platform {
			result = append(result, a)
		}
	}
	return result, nil
}

// scanDir scans a directory recursively for .js adapter files.
func scanDir(root string) ([]*SiteAdapter, error) {
	var adapters []*SiteAdapter
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".js" {
			return nil
		}
		a, err := ParseAdapter(path)
		if err != nil {
			return nil // skip malformed adapters
		}
		adapters = append(adapters, a)
		return nil
	})
	return adapters, err
}
