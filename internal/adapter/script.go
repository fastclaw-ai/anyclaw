package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
)

// ScriptAdapter executes commands by running Python or Node scripts.
// Convention: script reads JSON params from stdin, writes result to stdout.
type ScriptAdapter struct{}

func (a *ScriptAdapter) Execute(ctx context.Context, cmd *pkg.Command, params map[string]any, packageDir string) (*Result, error) {
	if cmd.Script == nil {
		return nil, fmt.Errorf("command %q missing script config", cmd.Name)
	}

	runtime := cmd.Script.Runtime
	if runtime == "" {
		runtime = "python3"
	}

	// Check runtime exists
	if _, err := exec.LookPath(runtime); err != nil {
		return nil, fmt.Errorf("%q not found. Install it first", runtime)
	}

	var scriptPath string
	if cmd.Script.Code != "" {
		// Inline code: write to temp file
		ext := ".py"
		if strings.Contains(runtime, "node") {
			ext = ".js"
		}
		tmp, err := os.CreateTemp("", "anyclaw-*"+ext)
		if err != nil {
			return nil, fmt.Errorf("create temp script: %w", err)
		}
		defer os.Remove(tmp.Name())
		if _, err := tmp.WriteString(cmd.Script.Code); err != nil {
			tmp.Close()
			return nil, fmt.Errorf("write temp script: %w", err)
		}
		tmp.Close()
		scriptPath = tmp.Name()
	} else if cmd.Script.File != "" {
		scriptPath = filepath.Join(packageDir, cmd.Script.File)
	} else {
		return nil, fmt.Errorf("command %q: script missing 'code' or 'file'", cmd.Name)
	}

	// Serialize params as JSON to stdin
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	c := exec.CommandContext(ctx, runtime, scriptPath)
	c.Dir = packageDir
	c.Stdin = bytes.NewReader(paramsJSON)

	output, err := c.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run script: %w\noutput: %s", err, string(output))
	}

	content := strings.TrimSpace(string(output))

	var data map[string]any
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		return &Result{Content: content, Data: data}, nil
	}

	return &Result{Content: content}, nil
}
