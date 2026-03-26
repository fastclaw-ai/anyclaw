package cmd

import (
	"fmt"
	"os"

	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/spf13/cobra"
)

var configFile string

// loadConfig reads the config file specified by --config flag.
func loadConfig() (*config.Config, error) {
	if configFile == "" {
		return nil, fmt.Errorf("--config is required")
	}
	return config.Load(configFile)
}

var rootCmd = &cobra.Command{
	Use:   "anyclaw",
	Short: "Agent protocol reverse proxy",
	Long:  "anyclaw makes any HTTP API callable by agent platforms via MCP or Skills.",
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
