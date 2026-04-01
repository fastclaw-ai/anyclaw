package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store manages site adapters from the private directory.
type Store struct {
	privateDir string // ~/.anyclaw/sites
}

// NewStore creates a Store at default locations.
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	s := &Store{
		privateDir: filepath.Join(home, ".anyclaw", "sites"),
	}
	// Ensure private dir exists
	if err := os.MkdirAll(s.privateDir, 0755); err != nil {
		return nil, fmt.Errorf("create private site dir: %w", err)
	}
	return s, nil
}

// PrivateDir returns the private adapters directory.
func (s *Store) PrivateDir() string { return s.privateDir }

// Get finds an adapter by "platform/command" name.
func (s *Store) Get(name string) (*SiteAdapter, error) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid adapter name %q: expected platform/command", name)
	}
	platform, command := parts[0], parts[1]

	path := filepath.Join(s.privateDir, platform, command+".js")
	if _, err := os.Stat(path); err == nil {
		return ParseAdapter(path)
	}

	return nil, fmt.Errorf("adapter %q not found", name)
}

// List returns all available adapters.
func (s *Store) List() ([]*SiteAdapter, error) {
	adapters, err := scanDir(s.privateDir)
	if err != nil {
		return nil, err
	}
	return adapters, nil
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
