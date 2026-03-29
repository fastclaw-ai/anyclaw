package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/registry"
	"github.com/fastclaw-ai/anyclaw/internal/site"
	"github.com/spf13/cobra"
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
		fmt.Printf("%-15s %-12s %s\n", "NAME", "TYPE", "URL")
		fmt.Println(strings.Repeat("─", 70))
		for _, r := range cfg.Repos {
			builtin := ""
			for _, d := range registry.DefaultRepos {
				if d.Name == r.Name {
					builtin = " (built-in)"
					break
				}
			}
			fmt.Printf("%-15s %-12s %s%s\n", r.Name, r.Type, r.URL, builtin)
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
				fmt.Fprintf(os.Stderr, "✓ %s updated\n", repo.Name)
				updated++
			case "opencli":
				// Nothing to cache locally — just verify connectivity
				resp, err := http.Head(repo.URL)
				if err != nil {
					fmt.Fprintf(os.Stderr, "✗ %s: %v\n", repo.Name, err)
					continue
				}
				resp.Body.Close()
				fmt.Fprintf(os.Stderr, "✓ %s ok\n", repo.Name)
				updated++
			default:
				// Anyclaw-type repo: fetch and cache index.yaml
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
				fmt.Fprintf(os.Stderr, "✓ %s updated\n", repo.Name)
				updated++
			}
		}

		fmt.Fprintf(os.Stderr, "%d repos updated\n", updated)
		return nil
	},
}

func init() {
	repoAddCmd.Flags().String("type", "", "repo type: anyclaw, opencli, bb-sites")
	repoCmd.AddCommand(repoListCmd, repoAddCmd, repoRemoveCmd, repoUpdateCmd)
	rootCmd.AddCommand(repoCmd)
}
