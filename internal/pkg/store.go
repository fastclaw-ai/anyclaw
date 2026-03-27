package pkg

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Store manages locally installed packages in ~/.anyclaw/packages/.
type Store struct {
	root string // ~/.anyclaw/packages
}

// NewStore creates a store at the default location.
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	root := filepath.Join(home, ".anyclaw", "packages")
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &Store{root: root}, nil
}

// Root returns the store root directory.
func (s *Store) Root() string {
	return s.root
}

// PackageDir returns the directory for a package.
func (s *Store) PackageDir(name string) string {
	return filepath.Join(s.root, name)
}

// ManifestPath returns the manifest.yaml path for a package.
func (s *Store) ManifestPath(name string) string {
	return filepath.Join(s.root, name, "manifest.yaml")
}

// Has checks if a package is installed.
func (s *Store) Has(name string) bool {
	_, err := os.Stat(s.ManifestPath(name))
	return err == nil
}

// Get loads a package manifest.
func (s *Store) Get(name string) (*Manifest, error) {
	return LoadManifest(s.ManifestPath(name))
}

// List returns all installed package manifests.
func (s *Store) List() ([]*Manifest, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read store: %w", err)
	}

	var manifests []*Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m, err := s.Get(entry.Name())
		if err != nil {
			continue // skip broken packages
		}
		manifests = append(manifests, m)
	}
	return manifests, nil
}

// Install writes a manifest and optional files to the store.
func (s *Store) Install(m *Manifest, files map[string][]byte) error {
	dir := s.PackageDir(m.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create package dir: %w", err)
	}

	// Write extra files first
	for name, data := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create dir for %s: %w", name, err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	// Write manifest
	data, err := marshalManifest(m)
	if err != nil {
		return err
	}
	return os.WriteFile(s.ManifestPath(m.Name), data, 0644)
}

// Remove deletes a package from the store.
func (s *Store) Remove(name string) error {
	dir := s.PackageDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("package %q not installed", name)
	}
	return os.RemoveAll(dir)
}

func marshalManifest(m *Manifest) ([]byte, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	return data, nil
}
