package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/fastclaw-ai/anyclaw/internal/backend/http"
	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/fastclaw-ai/anyclaw/internal/core"
	"github.com/fastclaw-ai/anyclaw/internal/frontend/acp"
	"github.com/spf13/cobra"
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Start as ACP agent (stdin/stdout JSON-RPC)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		backend := http.NewClient(&cfg.Backend)
		router := core.NewRouter(cfg, backend)
		server := acp.NewServer(router)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		fmt.Fprintf(os.Stderr, "[anyclaw] ACP agent started: %s\n", cfg.Name)
		return server.Serve(ctx)
	},
}

func loadConfig() (*config.Config, error) {
	if configFile != "" {
		return config.Load(configFile)
	}
	return nil, fmt.Errorf("--config is required")
}

func init() {
	rootCmd.AddCommand(acpCmd)
}
