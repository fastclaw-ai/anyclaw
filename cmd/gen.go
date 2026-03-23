package cmd

import (
	"fmt"
	"os"

	"github.com/fastclaw-ai/anyclaw/internal/gen"
	"github.com/spf13/cobra"
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate SKILL.md from config",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		output, _ := cmd.Flags().GetString("output")
		var w *os.File
		if output != "" {
			f, err := os.Create(output)
			if err != nil {
				return fmt.Errorf("create output file: %w", err)
			}
			defer f.Close()
			w = f
		} else {
			w = os.Stdout
		}

		gen.WriteSkillMD(w, cfg)
		if output != "" {
			fmt.Fprintf(os.Stderr, "SKILL.md written to %s\n", output)
		}
		return nil
	},
}

func init() {
	genCmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
	rootCmd.AddCommand(genCmd)
}
