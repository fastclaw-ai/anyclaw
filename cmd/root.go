package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/adapter"
	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/fastclaw-ai/anyclaw/internal/pkg"
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
	Short: "The universal tool adapter for AI agents",
	Long:  "anyclaw — The universal tool adapter for AI agents. Turn any API, website, or application into agent-ready tools via MCP, Skills, CLI and more.",
	// Handle unknown commands as package commands:
	// anyclaw juejin hot → anyclaw run juejin/hot
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return tryRunPackageCommand(args)
	},
}

// Execute runs the root command.
func Execute() {
	// Check if first arg looks like a package command (not a built-in)
	if len(os.Args) > 1 {
		first := os.Args[1]
		if !strings.HasPrefix(first, "-") && !isBuiltinCommand(first) {
			err := tryRunPackageCommand(os.Args[1:])
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			return
		}
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func tryRunPackageCommand(args []string) error {
	store, err := pkg.NewStore()
	if err != nil {
		return err
	}

	// Try: "juejin hot" → package=juejin, command=hot
	// Try: "juejin/hot" → package=juejin, command=hot
	// Try: "translate" → auto-find
	input := args[0]
	remaining := args[1:]

	// If first arg contains "/", treat as package/command
	if strings.Contains(input, "/") {
		pkgName, target, err := resolveCommand(store, input)
		if err != nil {
			return err
		}
		return executeResolved(store, pkgName, target, remaining)
	}

	// Try as package name + second arg as command
	if len(remaining) > 0 {
		// Check if first remaining arg is a flag
		if !strings.HasPrefix(remaining[0], "-") {
			combined := input + "/" + remaining[0]
			pkgName, target, err := resolveCommand(store, combined)
			if err == nil {
				return executeResolved(store, pkgName, target, remaining[1:])
			}
		}
	}

	// Try as package name (single command) or command name (auto-find)
	pkgName, target, err := resolveCommand(store, input)
	if err != nil {
		return err
	}
	return executeResolved(store, pkgName, target, remaining)
}

func executeResolved(store *pkg.Store, pkgName string, target *pkg.Command, args []string) error {
	// Check for --help
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printCommandHelp(pkgName, target)
			return nil
		}
	}

	params := parseRunArgs(nil, args, target)

	manifest, _ := store.Get(pkgName)
	a, err := adapter.New(manifest.InferAdapter())
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	result, err := a.Execute(ctx, target, params, store.PackageDir(pkgName))
	if err != nil {
		return err
	}

	jsonOutput := hasFlag(args, "--json")
	if !jsonOutput && len(target.Columns) > 0 {
		printTable(result.Content, target.Columns)
	} else {
		fmt.Println(result.Content)
	}
	return nil
}

var builtinCommands = map[string]bool{
	"install": true, "uninstall": true, "list": true, "run": true,
	"search": true, "mcp": true, "skills": true, "skill": true, "daemon": true, "version": true,
	"update": true, "upgrade": true, "help": true, "completion": true,
	"auth": true, "site": true, "browser": true, "repo": true, "show": true,
}

func isBuiltinCommand(name string) bool {
	return builtinCommands[name]
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to OpenAPI spec file")
}
