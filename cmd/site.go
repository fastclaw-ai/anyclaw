package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/site"
	"github.com/spf13/cobra"
)

var siteCmd = &cobra.Command{
	Use:   "site <platform/command> [args...]",
	Short: "Run a website as a CLI command via browser extension",
	Long: `Turn any website into a CLI command using the AnyClaw browser extension.

Site adapters run JavaScript inside your real browser, using your existing
login state — no API keys, no scraping, no anti-bot bypass needed.

Examples:
  anyclaw site update                   # pull community adapters from GitHub
  anyclaw site list                     # list all available adapters
  anyclaw site list zhihu               # list zhihu adapters
  anyclaw site info zhihu/hot           # show adapter details
  anyclaw site zhihu/hot                # run zhihu hot list
  anyclaw site twitter/search "AI"      # search Twitter
  anyclaw site hackernews/top --limit 5 # top 5 HN stories`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		// Route subcommands
		switch args[0] {
		case "update":
			return runSiteUpdate(args[1:])
		case "list":
			return runSiteList(args[1:])
		case "info":
			if len(args) < 2 {
				return fmt.Errorf("usage: anyclaw site info <platform/command>")
			}
			return runSiteInfo(args[1])
		default:
			return runSiteExec(args[0], args[1:])
		}
	},
}

func runSiteUpdate(_ []string) error {
	s, err := site.NewStore()
	if err != nil {
		return err
	}
	fmt.Println("Updating bb-sites from https://github.com/epiral/bb-sites.git...")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := site.UpdateFromGitHub(ctx, s.BBSitesDir()); err != nil {
		return err
	}

	adapters, _ := s.List()
	fmt.Printf("✓ Done. %d adapters available.\n", len(adapters))
	return nil
}

func runSiteList(args []string) error {
	s, err := site.NewStore()
	if err != nil {
		return err
	}

	var adapters []*site.SiteAdapter
	if len(args) > 0 {
		adapters, err = s.ListByPlatform(args[0])
		if err != nil {
			return err
		}
	} else {
		adapters, err = s.List()
		if err != nil {
			return err
		}
	}

	if len(adapters) == 0 {
		fmt.Println("No site adapters found. Run: anyclaw site update")
		return nil
	}

	// Sort by name
	sort.Slice(adapters, func(i, j int) bool {
		return adapters[i].Name < adapters[j].Name
	})

	// Print table
	fmt.Printf("%-25s %-12s %s\n", "COMMAND", "PLATFORM", "DESCRIPTION")
	fmt.Println(strings.Repeat("─", 70))
	for _, a := range adapters {
		desc := a.Description
		if len(desc) > 45 {
			desc = desc[:42] + "..."
		}
		fmt.Printf("%-25s %-12s %s\n", a.Name, a.Platform(), desc)
	}
	fmt.Printf("\n%d adapters\n", len(adapters))
	return nil
}

func runSiteInfo(name string) error {
	s, err := site.NewStore()
	if err != nil {
		return err
	}
	a, err := s.Get(name)
	if err != nil {
		return err
	}

	fmt.Printf("Name:    %s\n", a.Name)
	fmt.Printf("Desc:    %s\n", a.Description)
	if a.Domain != "" {
		fmt.Printf("Domain:  %s\n", a.Domain)
	}
	if a.Example != "" {
		fmt.Printf("Example: %s\n", a.Example)
	}
	if len(a.Args) > 0 {
		fmt.Println("\nArguments:")
		for name, arg := range a.Args {
			req := ""
			if arg.Required {
				req = " (required)"
			}
			def := ""
			if arg.Default != "" {
				def = fmt.Sprintf(" [default: %s]", arg.Default)
			}
			fmt.Printf("  --%s%s%s  %s\n", name, req, def, arg.Description)
		}
	}
	return nil
}

func runSiteExec(name string, rawArgs []string) error {
	s, err := site.NewStore()
	if err != nil {
		return err
	}

	a, err := s.Get(name)
	if err != nil {
		return err
	}

	// Parse args: positional + --key value flags
	params := make(map[string]string)
	var positional []string

	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if arg == "--json" {
			continue
		}
		if strings.HasPrefix(arg, "--") {
			key := arg[2:]
			if i+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[i+1], "--") {
				params[key] = rawArgs[i+1]
				i++
			} else {
				params[key] = "true"
			}
		} else {
			positional = append(positional, arg)
		}
	}

	// Map first positional to first required arg
	if len(positional) > 0 {
		first := site.FirstRequiredArg(a)
		if first != "" {
			params[first] = positional[0]
		}
	}

	// Apply defaults and merge
	params = site.BuildParams(a, params)

	// Execute
	runner := site.NewRunner()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	result, err := runner.Run(ctx, a, params)
	if err != nil {
		return err
	}

	fmt.Println(result)
	return nil
}

func init() {
	rootCmd.AddCommand(siteCmd)
}
