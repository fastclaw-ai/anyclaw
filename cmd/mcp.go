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
	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp [url]",
	Short: "Start as MCP server exposing installed packages as tools",
	Long: `Start as MCP server (stdin/stdout).

Without --config: exposes all installed packages as MCP tools.
With --config: legacy mode, exposes a single OpenAPI spec as MCP tools.
With --package: exposes only the specified package.

Examples:
  anyclaw mcp                        # serve all installed packages
  anyclaw mcp gh                     # serve only gh package
  anyclaw mcp --package gh           # same as above
  anyclaw mcp -c translator.yaml     # legacy: serve single OpenAPI spec`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		// Legacy mode: --config
		if configFile != "" {
			return mcpLegacy(ctx, args)
		}

		// Package filter: positional arg or --package flag
		pkgFilter, _ := cmd.Flags().GetString("package")
		if pkgFilter == "" && len(args) > 0 {
			pkgFilter = args[0]
		}
		return mcpPackages(ctx, pkgFilter)
	},
}

func mcpLegacy(ctx context.Context, args []string) error {
	var cfg *config.Config
	var err error

	if configFile != "" {
		cfg, err = config.Load(configFile)
		if err != nil {
			return err
		}
	} else {
		cfg = config.FromURL(args[0])
	}

	backend := http.NewClient(&cfg.Backend)
	router := core.NewRouter(cfg, backend)
	server := mcpfrontend.NewServer(router)

	fmt.Fprintf(os.Stderr, "[anyclaw] MCP server started: %s\n", cfg.Name)
	return server.Serve(ctx)
}

func mcpPackages(ctx context.Context, pkgFilter string) error {
	store, err := pkg.NewStore()
	if err != nil {
		return err
	}

	if pkgFilter != "" {
		// Serve a single package
		m, err := store.Get(pkgFilter)
		if err != nil {
			return fmt.Errorf("package %q not installed", pkgFilter)
		}
		server, err := mcpfrontend.NewServerFromManifests(store, []*pkg.Manifest{m})
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "[anyclaw] MCP server started: %s (%d tools)\n", m.Name, len(m.Commands))
		return server.Serve(ctx)
	}

	manifests, err := store.List()
	if err != nil {
		return err
	}

	if len(manifests) == 0 {
		return fmt.Errorf("no packages installed. Run 'anyclaw install <name>' first")
	}

	server, err := mcpfrontend.NewServerFromManifests(store, manifests)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "[anyclaw] MCP server started with %d packages\n", len(manifests))
	return server.Serve(ctx)
}

func init() {
	mcpCmd.Flags().StringP("package", "p", "", "Only expose a specific package")
	rootCmd.AddCommand(mcpCmd)
}
