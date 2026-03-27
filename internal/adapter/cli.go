package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"golang.org/x/term"
)

var placeholderRe = regexp.MustCompile(`\{\{\w+\}\}`)

// CLIAdapter executes commands by shelling out to CLI tools (e.g. opencli).
type CLIAdapter struct{}

func (a *CLIAdapter) Execute(ctx context.Context, cmd *pkg.Command, params map[string]any, packageDir string) (*Result, error) {
	if cmd.Run == "" {
		return nil, fmt.Errorf("command %q missing run template", cmd.Name)
	}

	// Check if the base command exists
	binName := strings.Fields(cmd.Run)[0]
	if _, err := exec.LookPath(binName); err != nil {
		return nil, fmt.Errorf("%q not found. Install it first: https://google.com/search?q=install+%s", binName, binName)
	}

	// Substitute {{arg}} placeholders in the run template
	cmdStr := cmd.Run
	for k, v := range params {
		cmdStr = strings.ReplaceAll(cmdStr, "{{"+k+"}}", fmt.Sprintf("%v", v))
	}

	// Fill in defaults for any remaining placeholders
	for name, arg := range cmd.Args {
		placeholder := "{{" + name + "}}"
		if strings.Contains(cmdStr, placeholder) && arg.Default != "" {
			cmdStr = strings.ReplaceAll(cmdStr, placeholder, arg.Default)
		}
	}

	// Remove any remaining unresolved placeholders
	cmdStr = placeholderRe.ReplaceAllString(cmdStr, "")
	cmdStr = strings.Join(strings.Fields(cmdStr), " ")

	// If running in a terminal, pass through stdin/stdout/stderr directly
	// so interactive commands (QR codes, prompts, etc.) work properly.
	if term.IsTerminal(int(os.Stdout.Fd())) {
		c := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		c.Dir = packageDir
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return nil, fmt.Errorf("run command: %w", err)
		}
		return &Result{Content: ""}, nil
	}

	// Non-terminal: capture output for programmatic use
	var buf bytes.Buffer
	c := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	c.Dir = packageDir
	c.Stdout = io.MultiWriter(&buf, os.Stdout)
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return nil, fmt.Errorf("run command: %w\noutput: %s", err, buf.String())
	}

	content := strings.TrimSpace(buf.String())

	// Try to parse as JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		return &Result{Content: content, Data: data}, nil
	}

	return &Result{Content: content}, nil
}
