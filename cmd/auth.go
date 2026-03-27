package cmd

import (
	"fmt"
	"os"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth <package> <api-key>",
	Short: "Set API key for a package",
	Long: `Set or remove API key for a package. Stored in ~/.anyclaw/credentials.yaml.

Examples:
  anyclaw auth fal-banana "Key your-api-key"
  anyclaw auth fal-banana --remove`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		remove, _ := cmd.Flags().GetBool("remove")

		creds, err := pkg.LoadCredentials()
		if err != nil {
			return err
		}

		if remove {
			if err := creds.Remove(name); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Removed credentials for %s\n", name)
			return nil
		}

		if len(args) < 2 {
			// Show current value (masked)
			if v := creds.Get(name); v != "" {
				masked := v[:3] + "***"
				fmt.Printf("%s: %s\n", name, masked)
			} else {
				fmt.Printf("%s: (not set)\n", name)
			}
			return nil
		}

		if err := creds.Set(name, args[1]); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Saved credentials for %s\n", name)
		return nil
	},
}

func init() {
	authCmd.Flags().Bool("remove", false, "Remove credentials")
	rootCmd.AddCommand(authCmd)
}
