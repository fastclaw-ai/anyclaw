package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var configFile string

var rootCmd = &cobra.Command{
	Use:   "anyclaw",
	Short: "Agent protocol reverse proxy",
	Long:  "anyclaw makes any HTTP API, CLI tool, or service callable by agent platforms via ACP, MCP, or HTTP.",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to config file (yaml/toml/json)")
}
