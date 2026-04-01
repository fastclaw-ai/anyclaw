package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/registry"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage package repositories",
	Long: `Manage package repositories (like helm repo).

Examples:
  anyclaw repo list
  anyclaw repo add myrepo https://raw.githubusercontent.com/user/repo/main/index.yaml
  anyclaw repo add myskills https://github.com/user/skills/tree/main/packages --type github-skills
  anyclaw repo remove myrepo
  anyclaw install myrepo/pkgname`,
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured repositories",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := registry.LoadRepoConfig()
		if err != nil {
			return err
		}
		if len(cfg.Repos) == 0 {
			fmt.Println("No repositories configured.")
			return nil
		}
		fmt.Printf("%-15s %-12s %-30s %s\n", "NAME", "TYPE", "URL", "CACHE")
		fmt.Println(strings.Repeat("─", 90))
		for _, r := range cfg.Repos {
			cacheInfo := "(no cache — run: anyclaw repo update)"
			if cache, err := registry.ReadCache(r.Name); err == nil {
				age := registry.FormatCacheAge(registry.CacheAgeDuration(cache))
				cacheInfo = fmt.Sprintf("(%d packages, updated %s)", len(cache.Packages), age)
			}
			fmt.Printf("%-15s %-12s %s\n", r.Name, r.Type, r.URL)
			fmt.Printf("%-15s %-12s %s\n", "", "", cacheInfo)
		}
		return nil
	},
}

var repoAddCmd = &cobra.Command{
	Use:   "add <name> <url> [--type anyclaw|github-skills]",
	Short: "Add a repository",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, url := args[0], args[1]
		repoType, _ := cmd.Flags().GetString("type")
		if repoType == "" {
			if strings.Contains(url, "github.com/") && strings.Contains(url, "/tree/") {
				repoType = "github-skills"
			} else {
				repoType = "anyclaw"
			}
		}

		cfg, err := registry.LoadRepoConfig()
		if err != nil {
			return err
		}
		cfg.AddRepo(registry.Repo{Name: name, URL: url, Type: repoType})
		if err := registry.SaveRepoConfig(cfg); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStderr(), "Added repo %q (%s)\n", name, repoType)
		return nil
	},
}

var repoRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg, err := registry.LoadRepoConfig()
		if err != nil {
			return err
		}
		if !cfg.RemoveRepo(name) {
			return fmt.Errorf("repo %q not found", name)
		}
		if err := registry.SaveRepoConfig(cfg); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStderr(), "Removed repo %q\n", name)
		return nil
	},
}

var repoUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update/refresh all configured repositories",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := registry.LoadRepoConfig()
		if err != nil {
			return err
		}
		if len(cfg.Repos) == 0 {
			fmt.Println("No repositories configured.")
			return nil
		}

		fmt.Println("Updating repos...")
		updated := 0
		for _, repo := range cfg.Repos {
			switch repo.Type {
			case "github-skills":
				// No connectivity check — buildRepoCache uses GitHub Contents API
				updated++
			default:
				// Anyclaw-type repo: verify index.yaml
				indexURL := strings.TrimSuffix(repo.URL, "/")
				if !strings.HasSuffix(indexURL, ".yaml") && !strings.HasSuffix(indexURL, ".yml") {
					indexURL += "/index.yaml"
				}
				resp, err := http.Get(indexURL)
				if err != nil {
					fmt.Fprintf(os.Stderr, "✗ %s: %v\n", repo.Name, err)
					continue
				}
				resp.Body.Close()
				if resp.StatusCode >= 400 {
					fmt.Fprintf(os.Stderr, "✗ %s: HTTP %d\n", repo.Name, resp.StatusCode)
					continue
				}
				updated++
			}

			// Build cache for this repo
			if n, err := buildRepoCache(&repo); err != nil {
				fmt.Fprintf(os.Stderr, "✗ %s: cache failed: %v\n", repo.Name, err)
			} else {
				fmt.Fprintf(os.Stderr, "✓ %s: %d packages cached\n", repo.Name, n)
			}
		}

		fmt.Fprintf(os.Stderr, "\n%d repos updated. Search is now fast.\n", updated)
		return nil
	},
}

// buildRepoCache fetches the full package list for a repo and writes it to local cache.
// Returns the number of packages cached.
func buildRepoCache(repo *registry.Repo) (int, error) {
	var pkgs []registry.CachePackage

	switch repo.Type {
	case "github-skills":
		items, err := fetchGitHubDirAll(repo.URL)
		if err != nil {
			return 0, err
		}
		pkgs = items
	default:
		// anyclaw-type: fetch index.yaml
		items, err := fetchAnyclawIndex(repo)
		if err != nil {
			return 0, err
		}
		pkgs = items
	}

	entry := &registry.CacheEntry{
		Repo:     repo.Name,
		Packages: pkgs,
	}
	if err := registry.WriteCache(entry); err != nil {
		return 0, err
	}
	return len(pkgs), nil
}

// fetchGitHubDirAll fetches all entries from a GitHub directory listing.
func fetchGitHubDirAll(repoURL string) ([]registry.CachePackage, error) {
	repoURL = strings.TrimSuffix(repoURL, "/")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid GitHub URL: %s", repoURL)
	}
	owner := parts[3]
	repo := parts[4]
	subDir := ""
	if len(parts) > 6 && parts[5] == "tree" {
		subDir = strings.Join(parts[7:], "/")
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, subDir)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var contents []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, err
	}

	var pkgs []registry.CachePackage
	for _, c := range contents {
		name := strings.TrimSuffix(c.Name, filepath.Ext(c.Name))
		pkgs = append(pkgs, registry.CachePackage{Name: name})
	}
	return pkgs, nil
}

// fetchAnyclawIndex fetches packages from an anyclaw-type repo's index.yaml.
func fetchAnyclawIndex(repo *registry.Repo) ([]registry.CachePackage, error) {
	indexURL := strings.TrimSuffix(repo.URL, "/")
	if !strings.HasSuffix(indexURL, ".yaml") && !strings.HasSuffix(indexURL, ".yml") {
		indexURL += "/index.yaml"
	}

	resp, err := http.Get(indexURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var idx registry.Index
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}

	var pkgs []registry.CachePackage
	for _, p := range idx.Packages {
		pkgs = append(pkgs, registry.CachePackage{
			Name:        p.Name,
			Description: p.Description,
		})
	}
	return pkgs, nil
}

func init() {
	repoAddCmd.Flags().String("type", "", "repo type: anyclaw, github-skills")
	repoCmd.AddCommand(repoListCmd, repoAddCmd, repoRemoveCmd, repoUpdateCmd)
	rootCmd.AddCommand(repoCmd)
}
