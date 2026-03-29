package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Repo describes a package repository.
type Repo struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Type string `yaml:"type"` // "anyclaw", "opencli", "bb-sites"
}

// RepoConfig is the repos.yaml file.
type RepoConfig struct {
	Repos []Repo `yaml:"repos"`
}

// DefaultRepos are built-in repos (always available, no need to add).
var DefaultRepos = []Repo{
	{Name: "opencli", URL: "https://github.com/jackwener/opencli/tree/main/src/clis", Type: "opencli"},
	{Name: "bb-sites", URL: "https://github.com/nicepkg/bb-sites", Type: "bb-sites"},
}

func repoConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".anyclaw", "repos.yaml"), nil
}

// LoadRepoConfig loads repos.yaml, returning default config if not found.
func LoadRepoConfig() (*RepoConfig, error) {
	path, err := repoConfigPath()
	if err != nil {
		return &RepoConfig{Repos: append([]Repo{}, DefaultRepos...)}, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &RepoConfig{Repos: append([]Repo{}, DefaultRepos...)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read repos: %w", err)
	}
	var cfg RepoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse repos: %w", err)
	}
	// Always include default repos (deduplicated by name)
	existing := make(map[string]bool)
	for _, r := range cfg.Repos {
		existing[r.Name] = true
	}
	for _, r := range DefaultRepos {
		if !existing[r.Name] {
			cfg.Repos = append(cfg.Repos, r)
		}
	}
	return &cfg, nil
}

// SaveRepoConfig writes repos.yaml (only user-added repos, not defaults).
func SaveRepoConfig(cfg *RepoConfig) error {
	path, err := repoConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// Filter out default repos — they're always injected on load
	isDefault := make(map[string]bool)
	for _, d := range DefaultRepos {
		isDefault[d.Name] = true
	}
	save := &RepoConfig{}
	for _, r := range cfg.Repos {
		if !isDefault[r.Name] {
			save.Repos = append(save.Repos, r)
		}
	}
	data, err := yaml.Marshal(save)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetRepo finds a repo by name.
func (cfg *RepoConfig) GetRepo(name string) (*Repo, bool) {
	for i := range cfg.Repos {
		if cfg.Repos[i].Name == name {
			return &cfg.Repos[i], true
		}
	}
	return nil, false
}

// AddRepo adds or updates a repo.
func (cfg *RepoConfig) AddRepo(repo Repo) {
	for i, r := range cfg.Repos {
		if r.Name == repo.Name {
			cfg.Repos[i] = repo
			return
		}
	}
	cfg.Repos = append(cfg.Repos, repo)
}

// RemoveRepo removes a repo by name.
func (cfg *RepoConfig) RemoveRepo(name string) bool {
	for i, r := range cfg.Repos {
		if r.Name == name {
			cfg.Repos = append(cfg.Repos[:i], cfg.Repos[i+1:]...)
			return true
		}
	}
	return false
}
