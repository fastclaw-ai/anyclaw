package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/adapter"
	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <package/command> [--arg value ...]",
	Short: "Run a command from an installed package",
	Long: `Run a command from an installed package.

Examples:
  anyclaw run translator/translate --q hello
  anyclaw run bilibili/hot --limit 5
  anyclaw run bilibili/search --keyword "AI"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := pkg.NewStore()
		if err != nil {
			return err
		}

		pkgName, target, err := resolveCommand(store, args[0])
		if err != nil {
			return err
		}

		// Check for --help
		for _, a := range args[1:] {
			if a == "--help" || a == "-h" {
				printCommandHelp(pkgName, target)
				return nil
			}
		}

		// Parse remaining flags as params
		params := parseRunArgs(cmd, args[1:], target)

		// Create adapter and execute
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

		fmt.Println(result.Content)
		return nil
	},
}

// parseRunArgs extracts --key value pairs from remaining args.
func parseRunArgs(cmd *cobra.Command, args []string, target *pkg.Command) map[string]any {
	params := make(map[string]any)

	// Fill defaults
	for name, arg := range target.Args {
		if arg.Default != "" {
			params[name] = arg.Default
		}
	}

	// Parse --key value pairs and collect positional args
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if len(arg) > 2 && arg[:2] == "--" {
			key := arg[2:]
			if i+1 < len(args) && (len(args[i+1]) < 2 || args[i+1][:2] != "--") {
				params[key] = args[i+1]
				i++
			} else {
				params[key] = "true"
			}
		} else {
			positional = append(positional, arg)
		}
	}

	// For CLI wrapper commands, append positional args to the "args" param
	if len(positional) > 0 {
		existing, _ := params["args"].(string)
		if existing != "" {
			params["args"] = existing + " " + strings.Join(positional, " ")
		} else {
			params["args"] = strings.Join(positional, " ")
		}
	}

	return params
}

// resolveCommand finds the package and command from input like "translate", "translator/translate", or "translator".
func resolveCommand(store *pkg.Store, input string) (string, *pkg.Command, error) {
	parts := strings.SplitN(input, "/", 2)

	// Case 1: package/command — exact match
	if len(parts) == 2 {
		m, err := store.Get(parts[0])
		if err != nil {
			return "", nil, fmt.Errorf("package %q not installed", parts[0])
		}
		for i := range m.Commands {
			if m.Commands[i].Name == parts[1] {
				return m.Name, &m.Commands[i], nil
			}
		}
		names := make([]string, len(m.Commands))
		for i, c := range m.Commands {
			names[i] = c.Name
		}
		return "", nil, fmt.Errorf("command %q not found in %q, available: %v", parts[1], parts[0], names)
	}

	name := parts[0]

	// Case 2: exact package name
	if m, err := store.Get(name); err == nil {
		if len(m.Commands) == 1 {
			return m.Name, &m.Commands[0], nil
		}
		// For CLI adapter packages, passthrough to the original CLI
		if m.InferAdapter() == "cli" {
			// Extract the base CLI name from source (e.g. "cli:/usr/bin/docker" -> "docker")
			binName := name
			if strings.HasPrefix(m.Source, "cli:") {
				binName = filepath.Base(strings.TrimPrefix(m.Source, "cli:"))
			}
			passthrough := &pkg.Command{
				Name:        name,
				Description: m.Description,
				Run:         binName + " {{args}}",
				Args: map[string]pkg.Arg{
					"args": {Type: "string", Description: "Arguments"},
				},
			}
			return m.Name, passthrough, nil
		}
		// Multiple commands, list them
		var cmds []string
		for _, c := range m.Commands {
			desc := ""
			if c.Description != "" {
				desc = "  " + c.Description
			}
			cmds = append(cmds, fmt.Sprintf("  %s/%s%s", m.Name, c.Name, desc))
		}
		return "", nil, fmt.Errorf("package %q has multiple commands:\n%s", name, strings.Join(cmds, "\n"))
	}

	// Case 3: search all packages for a matching command name
	manifests, err := store.List()
	if err != nil {
		return "", nil, err
	}

	type match struct {
		pkgName string
		cmd     *pkg.Command
	}
	var matches []match

	for _, m := range manifests {
		for i := range m.Commands {
			if m.Commands[i].Name == name {
				matches = append(matches, match{m.Name, &m.Commands[i]})
			}
		}
	}

	if len(matches) == 1 {
		return matches[0].pkgName, matches[0].cmd, nil
	}
	if len(matches) > 1 {
		var options []string
		for _, m := range matches {
			options = append(options, m.pkgName+"/"+name)
		}
		return "", nil, fmt.Errorf("command %q found in multiple packages, specify one:\n  %s", name, strings.Join(options, "\n  "))
	}

	return "", nil, fmt.Errorf("command %q not found in any installed package", name)
}

func printCommandHelp(pkgName string, cmd *pkg.Command) {
	if cmd.Description != "" {
		fmt.Println(cmd.Description)
		fmt.Println()
	}

	fmt.Printf("Usage:\n  anyclaw run %s/%s [flags]\n", pkgName, cmd.Name)

	if len(cmd.Args) > 0 {
		fmt.Println("\nFlags:")
		for name, arg := range cmd.Args {
			req := ""
			if arg.Required {
				req = " (required)"
			}
			def := ""
			if arg.Default != "" {
				def = fmt.Sprintf(" (default: %s)", arg.Default)
			}
			desc := ""
			if arg.Description != "" {
				desc = "  " + arg.Description
			}
			fmt.Printf("  --%s%s%s%s\n", name, req, def, desc)
		}
	}
}

func init() {
	runCmd.DisableFlagParsing = true
	rootCmd.AddCommand(runCmd)
}
