package cmd

import (
	"fmt"
	"os"
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
		fmt.Println("  anyclaw search repo news       # discover packages from repos")
		fmt.Println("  anyclaw install opencli/hackernews  # install a package")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tCOMMANDS\tADAPTER\tSOURCE\n")
	for _, m := range manifests {
		source := m.Source
		// Simplify source display
		switch {
		case strings.HasPrefix(source, "github-url:"), strings.HasPrefix(source, "github:"):
			source = "github"
		case strings.HasPrefix(source, "bb-sites:"):
			source = "bb-sites"
		case strings.HasPrefix(source, "url:"):
			source = "url"
		case strings.HasPrefix(source, "cli:"):
			source = "cli"
		case strings.HasPrefix(source, "local:"):
			source = "local"
		default:
			if idx := strings.Index(source, ":"); idx >= 0 {
				source = source[:idx]
			}
		}
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", m.Name, len(m.Commands), m.InferAdapter(), source)
	}
	w.Flush()

	return nil
}

func init() {
	rootCmd.AddCommand(listCmd)
}
