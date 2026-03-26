package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/fastclaw-ai/anyclaw/internal/backend/http"
	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/fastclaw-ai/anyclaw/internal/core"
	"github.com/fastclaw-ai/anyclaw/internal/frontend/openai"
	"github.com/spf13/cobra"
)

var openaiPort int

var openaiCmd = &cobra.Command{
	Use:   "openai",
	Short: "Start as OpenAI-compatible API server (/v1/chat/completions)",
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
		server := openai.NewServer(router, openaiPort)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		fmt.Fprintf(os.Stderr, "[anyclaw] OpenAI-compatible server starting: %s\n", cfg.Name)
		return server.Serve(ctx)
	},
}

func init() {
	openaiCmd.Flags().IntVarP(&openaiPort, "port", "p", 8080, "HTTP server port")
	rootCmd.AddCommand(openaiCmd)
}
