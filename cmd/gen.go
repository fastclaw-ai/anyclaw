package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/gen"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Generate SKILL.md from config",
	Long: `Generate SKILL.md from config.

Examples:
  anyclaw skill -c translator.yaml                                # write to ./translator/SKILL.md
  anyclaw skill -c translator.yaml -o ./output/SKILL.md           # write to ./output/SKILL.md
  anyclaw skill -c translator.yaml -o ~/.claude/skills/translator  # write to ~/.claude/skills/translator/SKILL.md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		output, _ := cmd.Flags().GetString("output")

		// Default: ./{name}/SKILL.md
		if output == "" {
			output = filepath.Join(cfg.Name, "SKILL.md")
		} else if !strings.HasSuffix(strings.ToLower(output), ".md") {
			// Treat as directory, append SKILL.md
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
	},
}

func init() {
	skillCmd.Flags().StringP("output", "o", "", "Output path: file (.md) or directory (default: ./{name}/SKILL.md)")
	rootCmd.AddCommand(skillCmd)
}
