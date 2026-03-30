package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/registry"
	"github.com/fastclaw-ai/anyclaw/internal/site"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage package repositories",
	Long: `Manage package repositories (like helm repo).

Repositories are sources of packages. anyclaw ships with two built-in repos:
  opencli   - https://github.com/jackwener/opencli (YAML + TS packages)
  bb-sites  - https://github.com/nicepkg/bb-sites (JS browser adapters)

Examples:
  anyclaw repo list
  anyclaw repo add myrepo https://raw.githubusercontent.com/user/repo/main/index.yaml
  anyclaw repo remove myrepo
  anyclaw install opencli/weibo
  anyclaw install bb-sites/zhihu`,
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
			builtin := ""
			for _, d := range registry.DefaultRepos {
				if d.Name == r.Name {
					builtin = " (built-in)"
					break
				}
			}
			fmt.Printf("%-15s %-12s %s%s\n", r.Name, r.Type, r.URL, builtin)
			fmt.Printf("%-15s %-12s %s\n", "", "", cacheInfo)
		}
		return nil
	},
}

var repoAddCmd = &cobra.Command{
	Use:   "add <name> <url> [--type anyclaw|opencli|bb-sites]",
	Short: "Add a repository",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, url := args[0], args[1]
		repoType, _ := cmd.Flags().GetString("type")
		if repoType == "" {
			// Auto-detect type from URL
			if strings.Contains(url, "opencli") {
				repoType = "opencli"
			} else if strings.Contains(url, "bb-sites") || strings.Contains(url, "bb-browser") {
				repoType = "bb-sites"
			} else if strings.Contains(url, "github.com/") && strings.Contains(url, "/tree/") {
				// GitHub tree URL pointing to a skills/packages directory
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
		// Don't allow removing built-in repos
		for _, d := range registry.DefaultRepos {
			if d.Name == name {
				return fmt.Errorf("cannot remove built-in repo %q", name)
			}
		}
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
			case "bb-sites":
				home, _ := os.UserHomeDir()
				bbDir := filepath.Join(home, ".anyclaw", "bb-sites")
				if err := site.UpdateFromGitHub(context.Background(), bbDir); err != nil {
					fmt.Fprintf(os.Stderr, "✗ %s: %v\n", repo.Name, err)
					continue
				}
				updated++
			case "opencli", "github-skills":
				// No connectivity check — buildRepoCache uses GitHub Contents API
				updated++
			case "clawhub":
				// Connectivity check only — cache build below
				resp, err := http.Head("https://clawhub.ai")
				if err != nil {
					fmt.Fprintf(os.Stderr, "✗ %s: %v\n", repo.Name, err)
					continue
				}
				resp.Body.Close()
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
	case "opencli", "github-skills":
		items, err := fetchGitHubDirAll(repo.URL)
		if err != nil {
			return 0, err
		}
		pkgs = items

	case "bb-sites":
		items, err := fetchBBSitesAll()
		if err != nil {
			return 0, err
		}
		pkgs = items

	case "clawhub":
		items, err := fetchClawhubAll()
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

// fetchBBSitesAll lists packages from the local bb-sites clone or GitHub.
func fetchBBSitesAll() ([]registry.CachePackage, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	bbDir := filepath.Join(home, ".anyclaw", "bb-sites")

	// Try local clone first - bb-sites structure: platform/command.js
	_, statErr := os.Stat(bbDir)
	if statErr == nil {
		var pkgs []registry.CachePackage
		entries, err := os.ReadDir(bbDir)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "node_modules" {
					continue
				}
				platform := e.Name()
				// Skip non-platform dirs
				if platform == "CONTRIBUTING.md" || platform == "README.md" || platform == "SKILL.md" {
					continue
				}
				subEntries, err := os.ReadDir(filepath.Join(bbDir, platform))
				if err != nil {
					continue
				}
				for _, se := range subEntries {
					if !se.IsDir() && strings.HasSuffix(se.Name(), ".js") {
						cmd := strings.TrimSuffix(se.Name(), ".js")
						pkgs = append(pkgs, registry.CachePackage{
							Name: platform + "/" + cmd,
						})
					}
				}
			}
		}
		return pkgs, nil
	}

	// Fallback to GitHub API
	resp, err := http.Get("https://api.github.com/repos/epiral/bb-sites/contents")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var contents []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, err
	}

	var pkgs []registry.CachePackage
	for _, c := range contents {
		if strings.HasSuffix(c.Name, ".js") {
			pkgs = append(pkgs, registry.CachePackage{
				Name: strings.TrimSuffix(c.Name, ".js"),
			})
		}
	}
	return pkgs, nil
}

// fetchClawhubAll builds a clawhub cache by searching for broad category terms.
// The clawhub list API requires auth, so we use vector search with common terms.
func fetchClawhubAll() ([]registry.CachePackage, error) {
	// Broad search terms to populate the cache
	terms := []string{"web", "data", "search", "news", "social", "code", "file", "ai", "api", "browser", "finance", "tool", "domain", "email", "github", "image", "video", "music", "chart", "translate", "weather", "stock", "crypto", "pdf", "text", "scrape", "fetch", "read", "write", "database", "cloud", "deploy", "monitor", "alert", "notion", "slack", "discord", "twitter", "git"}
	seen := make(map[string]bool)
	var pkgs []registry.CachePackage

	for _, term := range terms {
		apiURL := fmt.Sprintf("https://clawhub.ai/api/v1/search?q=%s&limit=50", term)
		resp, err := http.Get(apiURL)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode >= 400 {
			continue
		}

		var result struct {
			Results []struct {
				Slug        string `json:"slug"`
				DisplayName string `json:"displayName"`
				Summary     string `json:"summary"`
			} `json:"results"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		for _, item := range result.Results {
			if seen[item.Slug] {
				continue
			}
			seen[item.Slug] = true
			desc := item.Summary
			if len(desc) > 100 {
				desc = desc[:97] + "..."
			}
			if desc == "" {
				desc = item.DisplayName
			}
			pkgs = append(pkgs, registry.CachePackage{
				Name:        item.Slug,
				Description: desc,
			})
		}
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
	repoAddCmd.Flags().String("type", "", "repo type: anyclaw, opencli, bb-sites, github-skills")
	repoCmd.AddCommand(repoListCmd, repoAddCmd, repoRemoveCmd, repoUpdateCmd)
	rootCmd.AddCommand(repoCmd)
}
