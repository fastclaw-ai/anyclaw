package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type CacheEntry struct {
	Repo      string         `json:"repo"`
	UpdatedAt time.Time      `json:"updated_at"`
	Packages  []CachePackage `json:"packages"`
}

type CachePackage struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func cacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".anyclaw", "repo-cache")
	return dir, os.MkdirAll(dir, 0755)
}

func CachePath(repoName string) (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, repoName+".json"), nil
}

func ReadCache(repoName string) (*CacheEntry, error) {
	path, err := CachePath(repoName)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func WriteCache(entry *CacheEntry) error {
	path, err := CachePath(entry.Repo)
	if err != nil {
		return err
	}
	entry.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func CacheAge(repoName string) (time.Duration, error) {
	entry, err := ReadCache(repoName)
	if err != nil {
		return 0, err
	}
	return time.Since(entry.UpdatedAt), nil
}

func CacheExists(repoName string) bool {
	_, err := ReadCache(repoName)
	return err == nil
}

func CacheAgeDuration(entry *CacheEntry) time.Duration {
	return time.Since(entry.UpdatedAt)
}

func FormatCacheAge(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd ago", days)
}
