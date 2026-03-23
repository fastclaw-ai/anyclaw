package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/fastclaw-ai/anyclaw/internal/backend/http"
	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/fastclaw-ai/anyclaw/internal/core"
	mcpfrontend "github.com/fastclaw-ai/anyclaw/internal/frontend/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp [url]",
	Short: "Start as MCP server (stdin/stdout)",
	RunE: func(cmd *cobra.Command, args []string) error {
		var cfg *config.Config
		var err error

		if configFile != "" {
			cfg, err = config.Load(configFile)
			if err != nil {
				return err
			}
		} else if len(args) > 0 {
			cfg = config.FromURL(args[0])
		} else {
			return fmt.Errorf("--config or URL argument is required")
		}

		backend := http.NewClient(&cfg.Backend)
		router := core.NewRouter(cfg, backend)
		server := mcpfrontend.NewServer(router)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		fmt.Fprintf(os.Stderr, "[anyclaw] MCP server started: %s\n", cfg.Name)
		return server.Serve(ctx)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
