package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"github.com/fastclaw-ai/anyclaw/internal/registry"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <package>",
	Short: "Show package details",
	Long: `Show detailed information about a package.

Examples:
  anyclaw show hackernews       Show installed or registry package info
  anyclaw show opencli/weibo    Show package from a specific repo`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Check if it's a repo/package reference
		if strings.Contains(name, "/") && !strings.HasPrefix(name, "http") {
			parts := strings.SplitN(name, "/", 2)
			return showFromRepo(parts[0], parts[1])
		}

		return showPackage(name)
	},
}

func showPackage(name string) error {
	store, err := pkg.NewStore()
	if err != nil {
		return err
	}

	// Try installed first
	if m, err := store.Get(name); err == nil {
		printManifestInfo(m, true)
		return nil
	}

	// Try registry
	idx, err := registry.FetchIndex()
	if err != nil {
		return fmt.Errorf("package %q not found locally and registry unavailable: %w", name, err)
	}

	entry, found := idx.Lookup(name)
	if !found {
		return fmt.Errorf("package %q not found (installed or registry)", name)
	}

	fmt.Printf("Name:        %s\n", entry.Name)
	fmt.Printf("Description: %s\n", entry.Description)
	fmt.Printf("Source:      %s\n", entry.Source)
	fmt.Printf("Installed:   no\n")
	fmt.Printf("Type:        %s\n", entry.Type)

	return nil
}

func showFromRepo(repoName string, pkgName string) error {
	store, err := pkg.NewStore()
	if err != nil {
		return err
	}

	// If installed, show local info
	if m, err := store.Get(pkgName); err == nil {
		printManifestInfo(m, true)
		return nil
	}

	// Look up in repo
	cfg, err := registry.LoadRepoConfig()
	if err != nil {
		return err
	}
	repo, ok := cfg.GetRepo(repoName)
	if !ok {
		return fmt.Errorf("repo %q not found", repoName)
	}

	fmt.Printf("Name:        %s\n", pkgName)
	fmt.Printf("Source:      %s/%s\n", repo.Name, pkgName)
	fmt.Printf("Installed:   no\n")
	fmt.Printf("Repo:        %s (%s)\n", repo.Name, repo.Type)
	fmt.Printf("\nInstall with: anyclaw install %s/%s\n", repoName, pkgName)

	return nil
}

func printManifestInfo(m *pkg.Manifest, installed bool) {
	fmt.Printf("Name:        %s\n", m.Name)
	if m.Description != "" {
		fmt.Printf("Description: %s\n", m.Description)
	}
	if m.Source != "" {
		fmt.Printf("Source:      %s\n", m.Source)
	}
	if installed {
		fmt.Printf("Installed:   yes\n")
	} else {
		fmt.Printf("Installed:   no\n")
	}
	fmt.Printf("Adapter:     %s\n", m.InferAdapter())
	v := m.Version
	if v == "" {
		v = "-"
	}
	fmt.Printf("Version:     %s\n", v)

	if len(m.Commands) > 0 {
		fmt.Println()
		fmt.Println("Commands:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, c := range m.Commands {
			fmt.Fprintf(w, "  %s\t%s\n", c.Name, c.Description)
		}
		w.Flush()
	}
}

func init() {
	rootCmd.AddCommand(showCmd)
}
