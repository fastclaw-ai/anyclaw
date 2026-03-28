package gen

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/fastclaw-ai/anyclaw/internal/pkg"
)

// WriteSkillMD writes a Claude Code compatible SKILL.md from the config.
func WriteSkillMD(w io.Writer, cfg *config.Config) {
	// Frontmatter
	fmt.Fprintf(w, "---\n")
	fmt.Fprintf(w, "name: %s\n", cfg.Name)
	fmt.Fprintf(w, "description: %s\n", cfg.Description)
	fmt.Fprintf(w, "---\n\n")

	// Title
	fmt.Fprintf(w, "# %s\n\n", cfg.Name)
	if cfg.Description != "" {
		fmt.Fprintf(w, "%s\n\n", cfg.Description)
	}

	// Instructions
	fmt.Fprintf(w, "When the user's request matches one of the skills below, call the corresponding API using `curl`. Parse the response and reply in natural language.\n\n")

	// Auth instructions
	if cfg.Backend.Auth != nil {
		writeAuthInstructions(w, cfg.Backend.Auth)
	}

	fmt.Fprintf(w, "## Available Skills\n\n")

	for _, skill := range cfg.Skills {
		writeSkill(w, cfg, &skill)
	}
}

func writeAuthInstructions(w io.Writer, auth *config.Auth) {
	fmt.Fprintf(w, "## Authentication\n\n")
	switch auth.Type {
	case "bearer":
		fmt.Fprintf(w, "Add header: `-H \"Authorization: Bearer $%s\"`\n\n", auth.TokenEnv)
	case "basic":
		fmt.Fprintf(w, "Add header: `-H \"Authorization: Basic $%s\"`\n\n", auth.TokenEnv)
	case "api_key":
		header := auth.Header
		if header == "" {
			header = "X-API-Key"
		}
		fmt.Fprintf(w, "Add header: `-H \"%s: $%s\"`\n\n", header, auth.TokenEnv)
	}
}

func writeSkill(w io.Writer, cfg *config.Config, skill *config.Skill) {
	fmt.Fprintf(w, "### %s\n\n", skill.Name)
	if skill.Description != "" {
		fmt.Fprintf(w, "%s\n\n", skill.Description)
	}

	method := strings.ToUpper(skill.Backend.Method)
	if method == "" {
		method = "POST"
	}
	url := cfg.Backend.BaseURL + skill.Backend.Path

	// Parameters
	if len(skill.Input) > 0 {
		fmt.Fprintf(w, "**Parameters:**\n\n")

		names := make([]string, 0, len(skill.Input))
		for name := range skill.Input {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			field := skill.Input[name]
			parts := []string{}
			if field.Type != "" {
				parts = append(parts, field.Type)
			}
			if field.Required {
				parts = append(parts, "required")
			}
			if field.Default != "" {
				parts = append(parts, fmt.Sprintf("default: %q", field.Default))
			}
			meta := ""
			if len(parts) > 0 {
				meta = fmt.Sprintf(" (%s)", strings.Join(parts, ", "))
			}
			desc := ""
			if field.Description != "" {
				desc = " - " + field.Description
			}
			fmt.Fprintf(w, "- `%s`%s%s\n", name, meta, desc)
		}
		fmt.Fprintf(w, "\n")
	}

	// Response extraction hint
	if skill.Backend.Response != nil && skill.Backend.Response.Field != "" {
		fmt.Fprintf(w, "**Response:** Extract `%s` from the JSON response.\n\n", skill.Backend.Response.Field)
	}

	// Example curl
	// Substitute path params into URL, collect remaining params for query/body
	exampleURL := url
	var queryParams []string
	names := sortedFieldNames(skill.Input)
	for _, name := range names {
		field := skill.Input[name]
		val := "<value>"
		if field.Default != "" {
			val = field.Default
		}
		placeholder := "{" + name + "}"
		if strings.Contains(exampleURL, placeholder) {
			exampleURL = strings.ReplaceAll(exampleURL, placeholder, val)
		} else {
			queryParams = append(queryParams, name)
		}
	}

	fmt.Fprintf(w, "**Example:**\n\n")
	fmt.Fprintf(w, "```bash\n")
	if method == "GET" || method == "DELETE" {
		fmt.Fprintf(w, "curl -s \"%s", exampleURL)
		if len(queryParams) > 0 {
			for i, name := range queryParams {
				field := skill.Input[name]
				val := "<value>"
				if field.Default != "" {
					val = field.Default
				}
				if i == 0 {
					fmt.Fprintf(w, "?")
				} else {
					fmt.Fprintf(w, "&")
				}
				fmt.Fprintf(w, "%s=%s", name, val)
			}
		}
		fmt.Fprintf(w, "\"\n")
	} else {
		fmt.Fprintf(w, "curl -s -X %s %s \\\n", method, exampleURL)
		fmt.Fprintf(w, "  -H \"Content-Type: application/json\" \\\n")
		fmt.Fprintf(w, "  -d '")
		fmt.Fprintf(w, "{")
		for i, name := range queryParams {
			field := skill.Input[name]
			val := "..."
			if field.Default != "" {
				val = field.Default
			}
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			fmt.Fprintf(w, "\"%s\": \"%s\"", name, val)
		}
		fmt.Fprintf(w, "}'\n")
	}
	fmt.Fprintf(w, "```\n\n")
}

func sortedFieldNames(input map[string]config.Field) []string {
	names := make([]string, 0, len(input))
	for name := range input {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// WriteManifestSkillMD generates a SKILL.md from a package manifest.
func WriteManifestSkillMD(w io.Writer, m *pkg.Manifest) {
	fmt.Fprintf(w, "---\n")
	fmt.Fprintf(w, "name: %s\n", m.Name)
	fmt.Fprintf(w, "description: %s\n", m.Description)
	fmt.Fprintf(w, "---\n\n")

	fmt.Fprintf(w, "# %s\n\n", m.Name)
	if m.Description != "" {
		fmt.Fprintf(w, "%s\n\n", m.Description)
	}

	// Instructions based on adapter type
	switch m.InferAdapter() {
	case "openapi":
		fmt.Fprintf(w, "When the user's request matches one of the commands below, use `anyclaw run %s <command>` to execute. Parse the response and reply in natural language.\n\n", m.Name)
	case "cli":
		fmt.Fprintf(w, "When the user's request matches one of the commands below, use `anyclaw run %s <command>` to execute. Parse the response and reply in natural language.\n\n", m.Name)
	case "script":
		fmt.Fprintf(w, "When the user's request matches one of the commands below, use `anyclaw run %s <command>` to execute.\n\n", m.Name)
	}

	fmt.Fprintf(w, "## Available Commands\n\n")

	for _, cmd := range m.Commands {
		writeManifestCommand(w, m.Name, &cmd)
	}
}

func writeManifestCommand(w io.Writer, pkgName string, cmd *pkg.Command) {
	fmt.Fprintf(w, "### %s\n\n", cmd.Name)
	if cmd.Description != "" {
		fmt.Fprintf(w, "%s\n\n", cmd.Description)
	}

	// Parameters
	if len(cmd.Args) > 0 {
		fmt.Fprintf(w, "**Parameters:**\n\n")
		names := sortedArgNames(cmd.Args)
		for _, name := range names {
			arg := cmd.Args[name]
			parts := []string{}
			if arg.Type != "" {
				parts = append(parts, arg.Type)
			}
			if arg.Required {
				parts = append(parts, "required")
			}
			if arg.Default != "" {
				parts = append(parts, fmt.Sprintf("default: %q", arg.Default))
			}
			meta := ""
			if len(parts) > 0 {
				meta = fmt.Sprintf(" (%s)", strings.Join(parts, ", "))
			}
			desc := ""
			if arg.Description != "" {
				desc = " - " + arg.Description
			}
			fmt.Fprintf(w, "- `%s`%s%s\n", name, meta, desc)
		}
		fmt.Fprintf(w, "\n")
	}

	// Usage example
	fmt.Fprintf(w, "**Usage:**\n\n")
	fmt.Fprintf(w, "```bash\nanyclaw run %s %s", pkgName, cmd.Name)
	names := sortedArgNames(cmd.Args)
	for _, name := range names {
		arg := cmd.Args[name]
		if arg.Required {
			fmt.Fprintf(w, " --%s <value>", name)
		}
	}
	fmt.Fprintf(w, "\n```\n\n")
}

func sortedArgNames(args map[string]pkg.Arg) []string {
	names := make([]string, 0, len(args))
	for name := range args {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
