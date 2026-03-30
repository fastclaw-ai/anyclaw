package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/gen"
	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:     "skills",
	Aliases: []string{"skill"},
	Short:   "Generate SKILL.md for installed packages or an OpenAPI spec",
	Long: `Generate SKILL.md for installed packages or an OpenAPI spec.

Examples:
  anyclaw skills                                                 # generate for all installed packages
  anyclaw skills gh                                               # generate for a specific package
  anyclaw skills --package gh                                     # same as above
  anyclaw skills -c translator.yaml                              # from OpenAPI spec
  anyclaw skills -c translator.yaml -o ~/.claude/skills/translator  # to specific directory`,
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")

		// Legacy mode: --config specified
		if configFile != "" {
			return skillFromConfig(output)
		}

		// Package filter: positional arg or --package flag
		pkgFilter, _ := cmd.Flags().GetString("package")
		if pkgFilter == "" && len(args) > 0 {
			pkgFilter = args[0]
		}
		return skillFromPackages(output, pkgFilter)
	},
}

func skillFromConfig(output string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if output == "" {
		home, _ := os.UserHomeDir()
		output = filepath.Join(home, ".anyclaw", "skills", cfg.Name, "SKILL.md")
	} else if !strings.HasSuffix(strings.ToLower(output), ".md") {
		output = filepath.Join(output, "SKILL.md")
	}

	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	f, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	gen.WriteSkillMD(f, cfg)
	fmt.Fprintf(os.Stderr, "SKILL.md written to %s\n", output)
	return nil
}

func skillFromPackages(output string, pkgFilter string) error {
	store, err := pkg.NewStore()
	if err != nil {
		return err
	}

	var manifests []*pkg.Manifest
	if pkgFilter != "" {
		m, err := store.Get(pkgFilter)
		if err != nil {
			return fmt.Errorf("package %q not installed", pkgFilter)
		}
		manifests = []*pkg.Manifest{m}
	} else {
		manifests, err = store.List()
		if err != nil {
			return err
		}
	}

	if len(manifests) == 0 {
		fmt.Println("No packages installed. Run 'anyclaw install <name>' first.")
		return nil
	}

	for _, m := range manifests {
		out := output
		if out == "" {
			home, _ := os.UserHomeDir()
			out = filepath.Join(home, ".anyclaw", "skills", m.Name, "SKILL.md")
		} else if !strings.HasSuffix(strings.ToLower(out), ".md") {
			out = filepath.Join(out, m.Name, "SKILL.md")
		}

		if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}

		f, err := os.Create(out)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}

		// Check if package has an author-written SKILL.md
		authorSkillPath := filepath.Join(store.PackageDir(m.Name), "SKILL.md")
		authorSkill, readErr := os.ReadFile(authorSkillPath)

		if readErr == nil && len(authorSkill) > 0 && len(m.Commands) > 0 {
			// Hybrid: author SKILL.md + auto-generated commands
			f.Write(authorSkill)
			if !strings.HasSuffix(string(authorSkill), "\n") {
				fmt.Fprintf(f, "\n")
			}
			fmt.Fprintf(f, "\n## Auto-generated Commands\n\n")
			fmt.Fprintf(f, "The following commands are also available via `anyclaw run %s <command>`:\n\n", m.Name)
			gen.WriteManifestSkillMDCommands(f, m)
		} else if readErr == nil && len(authorSkill) > 0 {
			// Skill-only: use author SKILL.md as-is
			f.Write(authorSkill)
		} else {
			// No author SKILL.md: auto-generate from commands
			gen.WriteManifestSkillMD(f, m)
		}

		f.Close()
		fmt.Fprintf(os.Stderr, "SKILL.md written to %s\n", out)
	}

	return nil
}

func init() {
	skillCmd.Flags().StringP("output", "o", "", "Output path: file (.md) or directory")
	skillCmd.Flags().StringP("package", "p", "", "Only generate for a specific package")
	rootCmd.AddCommand(skillCmd)
}
