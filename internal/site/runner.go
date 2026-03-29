package site

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fastclaw-ai/anyclaw/internal/adapter"
)

// Runner executes site adapters via the browser extension bridge.
type Runner struct{}

// NewRunner creates a new Runner.
func NewRunner() *Runner { return &Runner{} }

// Run executes a site adapter with the given parameters.
func (r *Runner) Run(ctx context.Context, a *SiteAdapter, params map[string]string) (string, error) {
	// Ensure daemon is running
	adapter.EnsureDaemon()

	// Wait for extension connection (up to 5 seconds)
	var connected bool
	for i := 0; i < 10; i++ {
		connected, _ = adapter.BridgeStatus()
		if connected {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !connected {
		return "", fmt.Errorf(`browser extension not connected.

To use site adapters:
  1. Open chrome://extensions
  2. Enable Developer Mode
  3. Load unpacked → select the 'extension' directory from anyclaw
  4. Keep Chrome open and try again`)
	}

	// Navigate to domain if specified
	if a.Domain != "" {
		url := "https://" + a.Domain
		if err := adapter.BridgeNavigate(url); err != nil {
			return "", fmt.Errorf("navigate to %s: %w", url, err)
		}
		// Wait for page to load
		time.Sleep(2 * time.Second)
	}

	// Build args JSON
	argsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshal args: %w", err)
	}

	// Wrap adapter code in an IIFE
	script := fmt.Sprintf(`(async () => {
  const args = %s;
  const fn = %s;
  const result = await fn(args);
  return result;
})()`, string(argsJSON), a.Code)

	// Execute in browser
	result, err := adapter.BridgeEvaluate(script)
	if err != nil {
		return "", fmt.Errorf("execute adapter: %w", err)
	}

	// Format result as JSON
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", result), nil
	}
	return string(out), nil
}

// BuildParams merges user-provided params with adapter defaults.
func BuildParams(a *SiteAdapter, userParams map[string]string) map[string]string {
	result := make(map[string]string)

	// Fill defaults first
	for name, arg := range a.Args {
		if arg.Default != "" {
			result[name] = arg.Default
		}
	}

	// Override with user params
	for k, v := range userParams {
		result[k] = v
	}

	return result
}

// FirstRequiredArg returns the name of the first required argument (alphabetically).
func FirstRequiredArg(a *SiteAdapter) string {
	var names []string
	for name, arg := range a.Args {
		if arg.Required {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	// Sort for determinism
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[i] > names[j] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names[0]
}
