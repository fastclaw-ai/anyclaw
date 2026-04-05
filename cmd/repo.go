package cmd

import (
	"encoding/base64"
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
	Long: `Manage package repositories.

Examples:
  anyclaw repo add https://github.com/larksuite/cli --name feishu-cli
  anyclaw repo add https://github.com/user/skills/tree/main/packages --name myskills
  anyclaw repo add https://example.com/index.yaml --name myrepo
  anyclaw repo list
  anyclaw repo update
  anyclaw repo remove feishu-cli`,
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
		fmt.Printf("%-15s %-15s %-30s %s\n", "NAME", "TYPE", "URL", "CACHE")
		fmt.Println(strings.Repeat("─", 100))
		for _, r := range cfg.Repos {
			cacheInfo := "(no cache — run: anyclaw repo update)"
			if cache, err := registry.ReadCache(r.Name); err == nil {
				age := registry.FormatCacheAge(registry.CacheAgeDuration(cache))
				cacheInfo = fmt.Sprintf("(%d packages, updated %s)", len(cache.Packages), age)
			}
			fmt.Printf("%-15s %-15s %s\n", r.Name, r.Type, r.URL)
			fmt.Printf("%-15s %-15s %s\n", "", "", cacheInfo)
		}
		return nil
	},
}

var repoAddCmd = &cobra.Command{
	Use:   "add <url> [--name name] [--type type]",
	Short: "Add a repository",
	Long: `Add a repository by URL.

Examples:
  anyclaw repo add https://github.com/larksuite/cli --name feishu-cli
  anyclaw repo add https://github.com/user/skills/tree/main/packages --name myskills
  anyclaw repo add https://example.com/index.yaml --name myrepo

The repo type is auto-detected from the URL:
  github.com/owner/repo              → github  (scans skills/, packages/, root dirs)
  github.com/owner/repo/tree/...     → github-skills (flat directory listing)
  other URLs                         → anyclaw (expects index.yaml)`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var name, url string
		nameFlag, _ := cmd.Flags().GetString("name")
		repoType, _ := cmd.Flags().GetString("type")

		if len(args) == 2 && nameFlag == "" {
			// Backward compat: anyclaw repo add <name> <url>
			name, url = args[0], args[1]
		} else {
			// New style: anyclaw repo add <url> --name <name>
			url = args[0]
			name = nameFlag
		}

		// Auto-derive name from URL if not provided
		if name == "" {
			name = repoNameFromURL(url)
			if name == "" {
				return fmt.Errorf("cannot derive repo name from URL, please specify --name")
			}
		}

		// Auto-detect type
		if repoType == "" {
			repoType = repoTypeFromURL(url)
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

// repoNameFromURL derives a repo name from a URL.
func repoNameFromURL(url string) string {
	url = strings.TrimSuffix(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) >= 5 && strings.Contains(url, "github.com") {
		// github.com/owner/repo → repo name
		return parts[4]
	}
	// Last path segment
	if len(parts) > 0 {
		base := parts[len(parts)-1]
		base = strings.TrimSuffix(base, filepath.Ext(base))
		if base != "" {
			return base
		}
	}
	return ""
}

// repoTypeFromURL auto-detects repo type from URL.
func repoTypeFromURL(url string) string {
	if strings.Contains(url, "github.com/") {
		if strings.Contains(url, "/tree/") {
			return "github-skills"
		}
		return "github"
	}
	return "anyclaw"
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
			case "github", "github-skills":
				// No connectivity check — buildRepoCache uses GitHub API
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
func buildRepoCache(repo *registry.Repo) (int, error) {
	var pkgs []registry.CachePackage

	switch repo.Type {
	case "github":
		items, err := fetchGitHubRepoPackages(repo.URL)
		if err != nil {
			return 0, err
		}
		pkgs = items
	case "github-skills":
		items, err := fetchGitHubDirAll(repo.URL)
		if err != nil {
			return 0, err
		}
		pkgs = items
	default:
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

// parseGitHubRepo extracts owner and repo from a GitHub URL.
func parseGitHubRepo(repoURL string) (owner, repo string, err error) {
	repoURL = strings.TrimSuffix(repoURL, "/")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 5 || !strings.Contains(repoURL, "github.com") {
		return "", "", fmt.Errorf("invalid GitHub URL: %s", repoURL)
	}
	return parts[3], parts[4], nil
}

// fetchGitHubRepoPackages scans a GitHub repo for packages.
// It looks in well-known directories (skills/, packages/) and also scans root-level dirs.
func fetchGitHubRepoPackages(repoURL string) ([]registry.CachePackage, error) {
	owner, repo, err := parseGitHubRepo(repoURL)
	if err != nil {
		return nil, err
	}

	// Fetch root contents
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/", owner, repo)
	rootEntries, err := fetchGitHubContents(apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch repo root: %w", err)
	}

	var pkgs []registry.CachePackage
	seen := make(map[string]bool)

	// Well-known package directories to scan (subdirs inside these are packages)
	packageDirs := []string{"skills", "packages", "registry", "plugins", "tools"}

	for _, entry := range rootEntries {
		if entry.Type != "dir" {
			continue
		}
		isPackageDir := false
		for _, pd := range packageDirs {
			if strings.EqualFold(entry.Name, pd) {
				isPackageDir = true
				break
			}
		}
		if isPackageDir {
			// Scan subdirectories as individual packages
			subURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, entry.Name)
			subEntries, err := fetchGitHubContents(subURL)
			if err != nil {
				continue
			}
			for _, sub := range subEntries {
				if sub.Type != "dir" {
					continue
				}
				if seen[sub.Name] {
					continue
				}
				seen[sub.Name] = true
				desc := tryFetchSkillDescription(owner, repo, entry.Name+"/"+sub.Name)
				pkgs = append(pkgs, registry.CachePackage{
					Name:        sub.Name,
					Description: desc,
				})
			}
		}
	}

	// If no packages found in well-known dirs, scan root dirs as packages
	if len(pkgs) == 0 {
		for _, entry := range rootEntries {
			if entry.Type != "dir" {
				continue
			}
			// Skip common non-package dirs
			if isCommonNonPackageDir(entry.Name) {
				continue
			}
			if seen[entry.Name] {
				continue
			}
			seen[entry.Name] = true
			desc := tryFetchSkillDescription(owner, repo, entry.Name)
			pkgs = append(pkgs, registry.CachePackage{
				Name:        entry.Name,
				Description: desc,
			})
		}
	}

	return pkgs, nil
}

type githubEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file" or "dir"
}

func fetchGitHubContents(apiURL string) ([]githubEntry, error) {
	resp, err := githubGet(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var entries []githubEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// tryFetchSkillDescription reads the first line of description from SKILL.md YAML frontmatter.
func tryFetchSkillDescription(owner, repo, path string) string {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s/SKILL.md", owner, repo, path)
	resp, err := githubGet(apiURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var fileResp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
		return ""
	}
	if fileResp.Encoding != "base64" {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(fileResp.Content, "\n", ""))
	if err != nil {
		return ""
	}
	content := string(decoded)

	// Parse YAML frontmatter for description
	if strings.HasPrefix(content, "---") {
		end := strings.Index(content[3:], "---")
		if end > 0 {
			frontmatter := content[3 : 3+end]
			var meta struct {
				Description string `yaml:"description"`
			}
			if err := yaml.Unmarshal([]byte(frontmatter), &meta); err == nil && meta.Description != "" {
				desc := meta.Description
				if len(desc) > 120 {
					desc = desc[:117] + "..."
				}
				return desc
			}
		}
	}
	return ""
}

func isCommonNonPackageDir(name string) bool {
	skip := map[string]bool{
		".github": true, ".git": true, "node_modules": true, "vendor": true,
		"cmd": true, "internal": true, "pkg": true, "lib": true, "src": true,
		"test": true, "tests": true, "docs": true, "doc": true,
		"scripts": true, "build": true, "dist": true, "bin": true,
		"assets": true, "static": true, "public": true, "examples": true,
		"skill-template": true, ".vscode": true, ".idea": true,
	}
	return skip[strings.ToLower(name)]
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
	entries, err := fetchGitHubContents(apiURL)
	if err != nil {
		return nil, err
	}

	var pkgs []registry.CachePackage
	for _, c := range entries {
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
	repoAddCmd.Flags().String("name", "", "Custom repo name (derived from URL if omitted)")
	repoAddCmd.Flags().String("type", "", "Repo type: anyclaw, github, github-skills (auto-detected)")
	repoCmd.AddCommand(repoListCmd, repoAddCmd, repoRemoveCmd, repoUpdateCmd)
	rootCmd.AddCommand(repoCmd)
}
