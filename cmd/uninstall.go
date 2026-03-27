package cmd

import (
	"fmt"
	"os"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Uninstall a package",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		store, err := pkg.NewStore()
		if err != nil {
			return err
		}

		if !store.Has(name) {
			return fmt.Errorf("package %q not installed", name)
		}

		if err := store.Remove(name); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Uninstalled %s\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
