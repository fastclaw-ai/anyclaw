package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"github.com/fastclaw-ai/anyclaw/internal/registry"
	"github.com/spf13/cobra"
)

func init() {
	upgradePkgCmd.Flags().Bool("all", false, "Upgrade all installed packages")
	rootCmd.AddCommand(upgradePkgCmd)
}

var upgradePkgCmd = &cobra.Command{
	Use:   "upgrade [package]",
	Short: "Upgrade installed packages to latest version",
	Long: `Upgrade installed packages by re-installing from their original source.

Examples:
  anyclaw upgrade              Upgrade all installed packages
  anyclaw upgrade hackernews   Upgrade a specific package
  anyclaw upgrade --all        Upgrade all (explicit)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := pkg.NewStore()
		if err != nil {
			return err
		}

		if len(args) > 0 {
			return upgradePackage(store, args[0])
		}

		// Upgrade all
		return upgradeAll(store)
	},
}

func upgradeAll(store *pkg.Store) error {
	manifests, err := store.List()
	if err != nil {
		return err
	}
	if len(manifests) == 0 {
		fmt.Println("No packages installed.")
		return nil
	}

	success, fail := 0, 0
	for _, m := range manifests {
		if err := upgradePackage(store, m.Name); err != nil {
			fail++
		} else {
			success++
		}
	}

	fmt.Fprintf(os.Stderr, "\n%d upgraded, %d failed\n", success, fail)
	return nil
}

func upgradePackage(store *pkg.Store, name string) error {
	m, err := store.Get(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ %s: not installed\n", name)
		return err
	}

	fmt.Fprintf(os.Stderr, "Upgrading %s...\n", name)

	source := m.Source
	if source == "" {
		fmt.Fprintf(os.Stderr, "✗ %s: no source recorded\n", name)
		return fmt.Errorf("no source for %s", name)
	}

	if err := reinstallFromSource(source, name); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %s: %v\n", name, err)
		return err
	}

	fmt.Fprintf(os.Stderr, "✓ %s upgraded\n", name)
	return nil
}

func reinstallFromSource(source string, name string) error {
	// Source format examples:
	//   "github:owner/repo"
	//   "local:file.yaml"
	//   "url:https://..."
	//   "cli:/usr/bin/tool"
	//   "registry:https://..."
	parts := strings.SplitN(source, ":", 2)
	if len(parts) != 2 {
		// Try as registry name
		return installFromRegistry(name, "")
	}

	scheme, ref := parts[0], parts[1]

	switch scheme {
	case "github", "github-url":
		url := ref
		if !strings.HasPrefix(ref, "http") {
			url = "https://github.com/" + ref
		}
		return installFromGitHub(url, "")
	case "registry":
		if strings.HasPrefix(ref, "http") {
			return installManifestFromURL(ref, "")
		}
		return installFromRegistry(name, "")
	case "url":
		return installFromURL(ref, "")
	case "cli":
		return installFromCLI(ref, name, "")
	case "local":
		// Can't upgrade local files — skip
		return fmt.Errorf("local source, skipping")
	default:
		// Try registry as fallback
		cfg, err := registry.LoadRepoConfig()
		if err == nil {
			if repo, ok := cfg.GetRepo(scheme); ok {
				return installFromRepo(repo, ref, "")
			}
		}
		return installFromRegistry(name, "")
	}
}
