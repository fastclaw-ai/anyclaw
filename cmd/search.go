package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fastclaw-ai/anyclaw/internal/registry"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <keyword>",
	Short: "Search packages in the registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyword := args[0]

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
			fmt.Fprintf(w, "%s\t%s\t[%s]\n", p.Name, p.Description, p.Type)
		}
		w.Flush()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
