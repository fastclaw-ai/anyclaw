package gen

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/config"
)

// WriteSkillMD writes a SKILL.md file from the config.
func WriteSkillMD(w io.Writer, cfg *config.Config) {
	fmt.Fprintf(w, "---\n")
	fmt.Fprintf(w, "name: %s\n", cfg.Name)
	fmt.Fprintf(w, "description: %s\n", cfg.Description)
	if cfg.Backend.BaseURL != "" {
		fmt.Fprintf(w, "base_url: %s\n", cfg.Backend.BaseURL)
	}
	fmt.Fprintf(w, "---\n\n")

	fmt.Fprintf(w, "# %s\n\n", cfg.Name)
	if cfg.Description != "" {
		fmt.Fprintf(w, "%s\n\n", cfg.Description)
	}

	fmt.Fprintf(w, "## Skills\n\n")

	for _, skill := range cfg.Skills {
		writeSkill(w, cfg, &skill)
	}
}

func writeSkill(w io.Writer, cfg *config.Config, skill *config.Skill) {
	fmt.Fprintf(w, "### %s\n\n", skill.Name)
	if skill.Description != "" {
		fmt.Fprintf(w, "%s\n\n", skill.Description)
	}

	// Endpoint
	method := strings.ToUpper(skill.Backend.Method)
	if method == "" {
		method = "POST"
	}
	fmt.Fprintf(w, "**Endpoint:** `%s %s%s`\n\n", method, cfg.Backend.BaseURL, skill.Backend.Path)

	// Input table
	if len(skill.Input) > 0 {
		fmt.Fprintf(w, "**Input:**\n\n")
		fmt.Fprintf(w, "| Field | Type | Required | Description |\n")
		fmt.Fprintf(w, "|-------|------|----------|-------------|\n")

		// Sort fields for deterministic output
		names := make([]string, 0, len(skill.Input))
		for name := range skill.Input {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			field := skill.Input[name]
			typ := field.Type
			if typ == "" {
				typ = "string"
			}
			req := "no"
			if field.Required {
				req = "yes"
			}
			fmt.Fprintf(w, "| %s | %s | %s | %s |\n", name, typ, req, field.Description)
		}
		fmt.Fprintf(w, "\n")
	}

	// Example curl
	fmt.Fprintf(w, "**Example:**\n\n")
	fmt.Fprintf(w, "```bash\ncurl -X %s %s%s", method, cfg.Backend.BaseURL, skill.Backend.Path)
	if method == "POST" || method == "PUT" {
		fmt.Fprintf(w, " \\\n  -H \"Content-Type: application/json\" \\\n  -d '{...}'")
	}
	fmt.Fprintf(w, "\n```\n\n")
}
