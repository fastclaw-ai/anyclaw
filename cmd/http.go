package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/fastclaw-ai/anyclaw/internal/backend/http"
	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/fastclaw-ai/anyclaw/internal/core"
	"github.com/fastclaw-ai/anyclaw/internal/frontend/httpapi"
	"github.com/spf13/cobra"
)

var httpPort int

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "Start as HTTP API server",
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
		server := httpapi.NewServer(router, httpPort)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		fmt.Fprintf(os.Stderr, "[anyclaw] HTTP server starting: %s\n", cfg.Name)
		return server.Serve(ctx)
	},
}

func init() {
	httpCmd.Flags().IntVarP(&httpPort, "port", "p", 8080, "HTTP server port")
	rootCmd.AddCommand(httpCmd)
}
