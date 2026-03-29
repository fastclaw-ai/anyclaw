package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
)

// BbSiteAdapter executes bb-sites JS adapters via the browser bridge.
type BbSiteAdapter struct{}

func (a *BbSiteAdapter) Execute(ctx context.Context, cmd *pkg.Command, params map[string]any, packageDir string) (*Result, error) {
	if cmd.Script == nil {
		return nil, fmt.Errorf("command %q missing script", cmd.Name)
	}

	EnsureDaemon()

	// Wait for extension
	var connected bool
	for i := 0; i < 10; i++ {
		connected, _ = BridgeStatus()
		if connected {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !connected {
		return nil, fmt.Errorf("browser extension not connected. Run: anyclaw daemon start")
	}

	// Navigate to domain
	if cmd.Script.Domain != "" {
		url := cmd.Script.Domain
		if url[:4] != "http" {
			url = "https://" + url
		}
		if err := BridgeNavigate(url); err != nil {
			return nil, fmt.Errorf("navigate: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	// Build args JSON
	argsJSON, _ := json.Marshal(params)

	script := fmt.Sprintf(`(async () => {
  const args = %s;
  const fn = %s;
  return await fn(args);
})()`, string(argsJSON), cmd.Script.Code)

	result, err := BridgeEvaluate(script)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &Result{Content: fmt.Sprintf("%v", result)}, nil
	}
	return &Result{Content: string(out)}, nil
}
