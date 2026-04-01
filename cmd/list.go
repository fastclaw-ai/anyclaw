package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed packages",
	Long: `List installed packages and their commands.

To browse available packages, use: anyclaw search repo <keyword>

Examples:
  anyclaw list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return listLocal()
	},
}

func listLocal() error {
	store, err := pkg.NewStore()
	if err != nil {
		return err
	}

	manifests, err := store.List()
	if err != nil {
		return err
	}

	if len(manifests) == 0 {
		fmt.Println("No packages installed.")
		fmt.Println()
		fmt.Println("Get started:")
		fmt.Println("  anyclaw search news             # discover packages")
		fmt.Println("  anyclaw install hackernews       # install a package")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tCOMMANDS\tSKILL\tADAPTER\tSOURCE\n")
	for _, m := range manifests {
		source := m.Source
		// Simplify source display
		if idx := strings.Index(source, ":"); idx >= 0 {
			source = source[:idx]
		}

		// Check for SKILL.md
		skillIndicator := "-"
		skillPath := filepath.Join(store.PackageDir(m.Name), "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			skillIndicator = "✓"
		}

		adapterStr := m.InferAdapter()
		if len(m.Commands) == 0 && skillIndicator == "✓" {
			adapterStr = "skill"
		} else if len(m.Commands) > 0 && skillIndicator == "✓" {
			adapterStr = m.InferAdapter()
		}

		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", m.Name, len(m.Commands), skillIndicator, adapterStr, source)
	}
	w.Flush()

	return nil
}

func init() {
	rootCmd.AddCommand(listCmd)
}
