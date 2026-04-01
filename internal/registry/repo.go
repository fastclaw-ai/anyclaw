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
	Type string `yaml:"type"` // "anyclaw", "github-skills"
}

// RepoConfig is the repos.yaml file.
type RepoConfig struct {
	Repos []Repo `yaml:"repos"`
}

func repoConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".anyclaw", "repos.yaml"), nil
}

// LoadRepoConfig loads repos.yaml, returning empty config if not found.
func LoadRepoConfig() (*RepoConfig, error) {
	path, err := repoConfigPath()
	if err != nil {
		return &RepoConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &RepoConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read repos: %w", err)
	}
	var cfg RepoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse repos: %w", err)
	}
	return &cfg, nil
}

// SaveRepoConfig writes repos.yaml.
func SaveRepoConfig(cfg *RepoConfig) error {
	path, err := repoConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
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
