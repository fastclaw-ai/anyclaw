package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/fastclaw-ai/anyclaw/internal/registry"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var searchCmd = &cobra.Command{
	Use:   "search <keyword>",
	Short: "Search packages across all repos",
	Long: `Search packages across all configured repos and the anyclaw hub.

  anyclaw search <keyword>             Search hub first, then all repos
  anyclaw search <keyword> --repo hub  Search anyclaw hub only
  anyclaw search <keyword> --repo opencli  Search a specific repo
  anyclaw search repo <keyword>        Search configured repos only
  anyclaw search hub <keyword>         Search the anyclaw hub only

Flags:
  --repo    Filter by repo name (hub, opencli, bb-sites, clawhub, ...)
  --page    Page number (default 1)
  --limit   Results per page (default 20)
  --json    Output as JSON`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoFilter, _ := cmd.Flags().GetString("repo")
		page, _ := cmd.Flags().GetInt("page")
		limit, _ := cmd.Flags().GetInt("limit")
		jsonOut, _ := cmd.Flags().GetBool("json")

		// Handle subcommands for backward compat
		if len(args) >= 2 && repoFilter == "" {
			switch args[0] {
			case "repo":
				repoFilter = "*repos*"
				args = args[1:]
			case "hub":
				repoFilter = "hub"
				args = args[1:]
			}
		}

		return searchAllWithOptions(args[0], repoFilter, page, limit, jsonOut)
	},
}

type searchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Repo        string `json:"repo"`
}

// searchAllWithOptions is the unified search with filtering, pagination, and JSON output.
func searchAllWithOptions(keyword, repoFilter string, page, limit int, jsonOut bool) error {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	kw := strings.ToLower(keyword)
	var results []searchResult

	hubOnly := repoFilter == "hub"
	reposOnly := repoFilter == "*repos*"
	specificRepo := repoFilter != "" && repoFilter != "hub" && repoFilter != "*repos*"

	// 1. anyclaw hub first (highest priority)
	if !reposOnly && !specificRepo {
		idx, err := registry.FetchIndex()
		if err == nil {
			for _, p := range idx.Search(keyword) {
				results = append(results, searchResult{
					Name:        p.Name,
					Description: p.Description,
					Repo:        "hub",
				})
			}
		}
	} else if repoFilter == "hub" {
		idx, err := registry.FetchIndex()
		if err == nil {
			for _, p := range idx.Search(keyword) {
				results = append(results, searchResult{
					Name:        p.Name,
					Description: p.Description,
					Repo:        "hub",
				})
			}
		}
	}

	// 2. Configured repos
	if !hubOnly {
		cfg, _ := registry.LoadRepoConfig()
		if cfg != nil {
			for _, repo := range cfg.Repos {
				// Filter by --repo if specified
				if specificRepo && !strings.EqualFold(repo.Name, repoFilter) {
					continue
				}
				matches, err := searchSingleRepo(&repo, kw)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", repo.Name, err)
					continue
				}
				for _, m := range matches {
					results = append(results, searchResult{
						Name:        repo.Name + "/" + m.name,
						Description: m.description,
						Repo:        repo.Name,
					})
				}
			}
		}
	}

	if len(results) == 0 {
		if jsonOut {
			fmt.Println("[]")
		} else {
			fmt.Printf("No packages found matching %q.\n", keyword)
		}
		return nil
	}

	// Pagination
	total := len(results)
	start := (page - 1) * limit
	if start >= total {
		if jsonOut {
			fmt.Println("[]")
		} else {
			fmt.Printf("No more results (total: %d)\n", total)
		}
		return nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	pageResults := results[start:end]
	totalPages := (total + limit - 1) / limit

	// Output
	if jsonOut {
		out, _ := json.MarshalIndent(pageResults, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tREPO\n")
	for _, r := range pageResults {
		desc := r.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.Name, desc, r.Repo)
	}
	w.Flush()

	fmt.Fprintf(os.Stderr, "\nPage %d/%d (%d results). ", page, totalPages, total)
	if page < totalPages {
		fmt.Fprintf(os.Stderr, "Next: anyclaw search %q --page %d\n", keyword, page+1)
	} else {
		fmt.Fprintln(os.Stderr)
	}
	return nil
}

func searchHub(keyword string) error {
	fmt.Fprintf(os.Stderr, "Searching registry for %q...\n", keyword)

	idx, err := registry.FetchIndex()
	if err != nil {
		return err
	}

	results := idx.Search(keyword)
	if len(results) == 0 {
		fmt.Printf("No packages found matching %q.\n", keyword)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tTYPE\n")
	for _, p := range results {
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.Description, p.Type)
	}
	w.Flush()

	return nil
}

func searchRepos(keyword string) error {
	cfg, err := registry.LoadRepoConfig()
	if err != nil {
		return err
	}

	if len(cfg.Repos) == 0 {
		fmt.Println("No repositories configured.")
		return nil
	}

	kw := strings.ToLower(keyword)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\n")
	found := 0

	for _, repo := range cfg.Repos {
		matches, err := searchSingleRepo(&repo, kw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", repo.Name, err)
			continue
		}
		for _, m := range matches {
			fmt.Fprintf(w, "%s/%s\t%s\n", repo.Name, m.name, m.description)
			found++
		}
	}

	if found == 0 {
		fmt.Printf("No packages found matching %q in configured repos.\n", keyword)
		return nil
	}

	w.Flush()
	return nil
}

type repoMatch struct {
	name        string
	description string
}

func searchSingleRepo(repo *registry.Repo, keyword string) ([]repoMatch, error) {
	switch repo.Type {
	case "opencli":
		return searchGitHubDir(repo.URL, keyword)
	case "bb-sites":
		return searchGitHubDir(repo.URL, keyword)
	case "clawhub":
		return searchClawhub(keyword)
	default:
		return searchRepoIndex(repo, keyword)
	}
}

// searchClawhub searches clawhub for skills matching the keyword.
func searchClawhub(keyword string) ([]repoMatch, error) {
	npxPath, err := exec.LookPath("npx")
	if err != nil {
		return nil, fmt.Errorf("clawhub search requires Node.js (npx not found)")
	}

	cmd := exec.Command(npxPath, "clawhub@latest", "search", keyword, "--no-input")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("clawhub search failed: %w", err)
	}

	var matches []repoMatch
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Parse output: each line is "<slug>  <name>  (<score>)" or similar
		// Split on multiple spaces
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			slug := parts[0]
			desc := ""
			if len(parts) >= 2 {
				desc = strings.Join(parts[1:], " ")
			}
			matches = append(matches, repoMatch{name: slug, description: desc})
		}
	}
	return matches, nil
}

// searchGitHubDir lists directories from a GitHub tree URL and matches names.
func searchGitHubDir(repoURL string, keyword string) ([]repoMatch, error) {
	repoURL = strings.TrimSuffix(repoURL, "/")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid GitHub URL: %s", repoURL)
	}
	owner := parts[3]
	repo := parts[4]

	// Extract subdir if present: /tree/branch/path
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

	var matches []repoMatch
	for _, c := range contents {
		name := strings.TrimSuffix(c.Name, filepath.Ext(c.Name))
		if strings.Contains(strings.ToLower(name), keyword) {
			matches = append(matches, repoMatch{name: name, description: ""})
		}
	}
	return matches, nil
}

// searchRepoIndex fetches an anyclaw-type repo's index.yaml and searches it.
func searchRepoIndex(repo *registry.Repo, keyword string) ([]repoMatch, error) {
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
	if err := yaml.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, err
	}

	results := idx.Search(keyword)
	var matches []repoMatch
	for _, p := range results {
		matches = append(matches, repoMatch{name: p.Name, description: p.Description})
	}
	return matches, nil
}

func init() {
	searchCmd.Flags().String("repo", "", "Filter by repo name (hub, opencli, bb-sites, clawhub, ...)")
	searchCmd.Flags().Int("page", 1, "Page number")
	searchCmd.Flags().Int("limit", 20, "Results per page")
	searchCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(searchCmd)
}
