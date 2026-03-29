package cmd

import (
	"fmt"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/registry"
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

func init() {
	repoAddCmd.Flags().String("type", "", "repo type: anyclaw, opencli, bb-sites")
	repoCmd.AddCommand(repoListCmd, repoAddCmd, repoRemoveCmd)
	rootCmd.AddCommand(repoCmd)
}
