package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/fastclaw-ai/anyclaw/internal/adapter"
	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <package> <command> [flags]",
	Short: "Run a command from an installed package",
	Long: `Run a command from an installed package.

Examples:
  anyclaw run hackernews top --limit 5
  anyclaw run translator translate --q hello
  anyclaw run bilibili hot --limit 5
  anyclaw run bilibili search --keyword "AI"

  # Slash format also supported (backward compat)
  anyclaw run hackernews/top --limit 5

  # Package with single command (command name optional)
  anyclaw run translator --q hello`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle: anyclaw run --help / anyclaw run -h / anyclaw run (no args)
		if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
			return cmd.Help()
		}

		store, err := pkg.NewStore()
		if err != nil {
			return err
		}

		var pkgName string
		var target *pkg.Command
		var remaining []string

		first := args[0]

		if strings.Contains(first, "/") {
			// Slash format: anyclaw run pkg/cmd [flags]
			pkgName, target, err = resolveCommand(store, first)
			if err != nil {
				if strings.HasPrefix(err.Error(), "skill-only:") {
					return printSkillOnlyPackage(store, strings.TrimPrefix(err.Error(), "skill-only:"))
				}
				return err
			}
			remaining = args[1:]
		} else if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			// Space format: anyclaw run pkg cmd [flags]
			combined := first + "/" + args[1]
			pkgName, target, err = resolveCommand(store, combined)
			if err != nil {
				// Fallback: maybe first arg is a single-command package
				pkgName, target, err = resolveCommand(store, first)
				if err != nil {
					if strings.HasPrefix(err.Error(), "skill-only:") {
						return printSkillOnlyPackage(store, strings.TrimPrefix(err.Error(), "skill-only:"))
					}
					return fmt.Errorf("command %q not found in package %q", args[1], first)
				}
				remaining = args[1:]
			} else {
				remaining = args[2:]
			}
		} else {
			// Single arg: package with one command, or command auto-find
			// Check for help on package
			if len(args) == 1 || (len(args) > 1 && (args[1] == "--help" || args[1] == "-h")) {
				if m, mErr := store.Get(first); mErr == nil && len(m.Commands) > 1 {
					printPackageHelp(m)
					return nil
				}
			}
			pkgName, target, err = resolveCommand(store, first)
			if err != nil {
				if strings.HasPrefix(err.Error(), "skill-only:") {
					return printSkillOnlyPackage(store, strings.TrimPrefix(err.Error(), "skill-only:"))
				}
				return err
			}
			remaining = args[1:]
		}

		// Check for --help
		for _, a := range remaining {
			if a == "--help" || a == "-h" {
				printCommandHelp(pkgName, target)
				return nil
			}
		}

		// Parse remaining flags as params
		params := parseRunArgs(cmd, remaining, target)

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

		// If command has columns defined, render as table
		jsonOutput := hasFlag(remaining, "--json")
		if !jsonOutput && len(target.Columns) > 0 {
			printTable(result.Content, target.Columns)
		} else {
			fmt.Println(result.Content)
		}
		return nil
	},
}

// parseRunArgs extracts --key value pairs from remaining args.
func parseRunArgs(cmd *cobra.Command, args []string, target *pkg.Command) map[string]any {
	params := make(map[string]any)

	// Fill defaults (skip nil/empty defaults for required args)
	for name, arg := range target.Args {
		if arg.Default != "" && arg.Default != "<nil>" {
			params[name] = arg.Default
		}
	}

	// Build short flag lookup: "-a" -> arg name
	shortToName := make(map[string]string)
	if target != nil {
		for name, arg := range target.Args {
			if arg.Short != "" {
				shortToName[arg.Short] = name
			}
		}
	}

	// Parse --key value / -x value pairs and collect positional args
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		// Skip output format flags
		if arg == "--json" {
			continue
		}
		if len(arg) > 2 && arg[:2] == "--" {
			key := arg[2:]
			if i+1 < len(args) && (len(args[i+1]) < 2 || args[i+1][:2] != "--") {
				params[key] = args[i+1]
				i++
			} else {
				params[key] = "true"
			}
		} else if len(arg) == 2 && arg[0] == '-' && arg[1] != '-' {
			shortKey := string(arg[1])
			name, ok := shortToName[shortKey]
			if !ok {
				positional = append(positional, arg)
				continue
			}
			// Check if this is a bool arg (no value needed)
			argDef := target.Args[name]
			if argDef.Type == "bool" {
				params[name] = "true"
			} else if i+1 < len(args) && (len(args[i+1]) < 1 || args[i+1][0] != '-') {
				params[name] = args[i+1]
				i++
			} else {
				params[name] = "true"
			}
		} else {
			positional = append(positional, arg)
		}
	}

	// Map positional args
	if len(positional) > 0 && target != nil {
		// First, try to map to required args that don't have values yet
		posIdx := 0
		for _, name := range sortedRequiredArgs(target) {
			if posIdx >= len(positional) {
				break
			}
			if _, exists := params[name]; !exists {
				params[name] = positional[posIdx]
				posIdx++
			}
		}
		// Remaining positional args go to "args" (for CLI wrapper commands)
		if posIdx < len(positional) {
			remaining := positional[posIdx:]
			existing, _ := params["args"].(string)
			if existing != "" {
				params["args"] = existing + " " + strings.Join(remaining, " ")
			} else {
				params["args"] = strings.Join(remaining, " ")
			}
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
		if len(m.Commands) == 0 {
			return "", nil, fmt.Errorf("skill-only:%s", m.Name)
		}
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
			cmds = append(cmds, fmt.Sprintf("  %s %s%s", m.Name, c.Name, desc))
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

func printSkillOnlyPackage(store *pkg.Store, pkgName string) error {
	skillPath := filepath.Join(store.PackageDir(pkgName), "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		fmt.Printf("Package %q has no runnable commands. Check: anyclaw show %s\n", pkgName, pkgName)
		return nil
	}
	fmt.Printf("[%s] This is a skill package (no direct commands).\n\n", pkgName)
	fmt.Println("SKILL.md:")
	fmt.Println("---")
	fmt.Print(string(data))
	if !strings.HasSuffix(string(data), "\n") {
		fmt.Println()
	}
	fmt.Println("---")
	fmt.Printf("\nTo use: read the SKILL.md above and call the scripts directly, or:\n")
	fmt.Printf("  anyclaw skills %s -o ~/.claude/skills/%s\n", pkgName, pkgName)
	return nil
}

func printPackageHelp(m *pkg.Manifest) {
	desc := m.Name
	if m.Description != "" {
		desc = m.Name + " — " + m.Description
	}
	fmt.Println(desc)
	fmt.Println()
	fmt.Println("Commands:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, c := range m.Commands {
		fmt.Fprintf(w, "  %s\t%s\n", c.Name, c.Description)
	}
	w.Flush()
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  anyclaw run %s <command> [flags]\n", m.Name)
	fmt.Printf("  anyclaw %s <command> [flags]\n", m.Name)
}

func printCommandHelp(pkgName string, cmd *pkg.Command) {
	if cmd.Description != "" {
		fmt.Println(cmd.Description)
		fmt.Println()
	}

	fmt.Printf("Usage:\n  anyclaw run %s %s [flags]\n", pkgName, cmd.Name)

	if len(cmd.Args) > 0 {
		fmt.Println("\nFlags:")
		for name, arg := range cmd.Args {
			short := ""
			if arg.Short != "" {
				short = fmt.Sprintf("-%s, ", arg.Short)
			}
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
			fmt.Printf("  %s--%s%s%s%s\n", short, name, req, def, desc)
		}
	}
}

// sortedRequiredArgs returns required arg names in a stable order.
func sortedRequiredArgs(target *pkg.Command) []string {
	var required []string
	for name, arg := range target.Args {
		if arg.Required {
			required = append(required, name)
		}
	}
	// Sort for deterministic ordering
	sort.Strings(required)
	return required
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// printTable renders JSON array data as a bordered table using the given column order.
func printTable(content string, columns []string) {
	var rows []map[string]any
	if err := json.Unmarshal([]byte(content), &rows); err != nil {
		fmt.Println(content)
		return
	}

	if len(rows) == 0 {
		fmt.Println("(no results)")
		return
	}

	// If no columns specified, derive from first row
	if len(columns) == 0 {
		for k := range rows[0] {
			columns = append(columns, k)
		}
	}

	// Calculate column widths
	widths := make([]int, len(columns))
	for i, c := range columns {
		// Capitalize first letter for header
		header := strings.ToUpper(c[:1]) + c[1:]
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i, c := range columns {
			val := fmt.Sprintf("%v", row[c])
			if len(val) > widths[i] {
				widths[i] = len(val)
			}
		}
	}

	// Cap max column width
	for i := range widths {
		if widths[i] > 80 {
			widths[i] = 80
		}
	}

	// Build separator line
	sepParts := make([]string, len(columns))
	for i, w := range widths {
		sepParts[i] = strings.Repeat("─", w+2)
	}
	sep := "├" + strings.Join(sepParts, "┼") + "┤"
	topBorder := "┌" + strings.Join(sepParts, "┬") + "┐"
	botBorder := "└" + strings.Join(sepParts, "┴") + "┘"

	// Print header
	fmt.Println(topBorder)
	headerParts := make([]string, len(columns))
	for i, c := range columns {
		header := strings.ToUpper(c[:1]) + c[1:]
		headerParts[i] = fmt.Sprintf(" %-*s ", widths[i], header)
	}
	fmt.Println("│" + strings.Join(headerParts, "│") + "│")
	fmt.Println(sep)

	// Print rows
	for _, row := range rows {
		valParts := make([]string, len(columns))
		for i, c := range columns {
			val := fmt.Sprintf("%v", row[c])
			if len(val) > widths[i] {
				val = val[:widths[i]-1] + "…"
			}
			valParts[i] = fmt.Sprintf(" %-*s ", widths[i], val)
		}
		fmt.Println("│" + strings.Join(valParts, "│") + "│")
	}

	fmt.Println(botBorder)
	fmt.Fprintf(os.Stderr, "\n%d items\n", len(rows))
}

func init() {
	runCmd.DisableFlagParsing = true
	rootCmd.AddCommand(runCmd)
}
