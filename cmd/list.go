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

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed packages and commands",
	Long: `List installed packages. Use --all to show all available packages from the registry.

Examples:
  anyclaw list              # show installed packages
  anyclaw list --all        # show all available packages from registry
  anyclaw list --all --page 2  # page 2 of registry packages`,
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all {
			page, _ := cmd.Flags().GetInt("page")
			pageSize, _ := cmd.Flags().GetInt("size")
			return listRemote(page, pageSize)
		}
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
		fmt.Println("  anyclaw search news        # discover packages")
		fmt.Println("  anyclaw install hackernews  # install a package")
		fmt.Println("  anyclaw list --all          # browse all available packages")
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

func listRemote(page, pageSize int) error {
	idx, err := registry.FetchIndex()
	if err != nil {
		return err
	}

	total := len(idx.Packages)
	if total == 0 {
		fmt.Println("Registry is empty.")
		return nil
	}

	// Pagination
	start := (page - 1) * pageSize
	if start >= total {
		fmt.Printf("No more packages. (total: %d, showing %d per page)\n", total, pageSize)
		return nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	totalPages := (total + pageSize - 1) / pageSize

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tTYPE\n")
	for _, p := range idx.Packages[start:end] {
		fmt.Fprintf(w, "%s\t%s\t[%s]\n", p.Name, p.Description, p.Type)
	}
	w.Flush()

	fmt.Fprintf(os.Stderr, "\nPage %d/%d (%d packages). ", page, totalPages, total)
	if page < totalPages {
		fmt.Fprintf(os.Stderr, "Next: anyclaw list --all --page %d\n", page+1)
	} else {
		fmt.Fprintln(os.Stderr, "")
	}

	return nil
}

func init() {
	listCmd.Flags().BoolP("all", "a", false, "Show all available packages from registry")
	listCmd.Flags().Int("page", 1, "Page number")
	listCmd.Flags().Int("size", 20, "Packages per page")
	rootCmd.AddCommand(listCmd)
}
