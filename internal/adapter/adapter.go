package adapter

import (
	"context"
	"fmt"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
)

// Result holds the output of a command execution.
type Result struct {
	Content string         // human-readable text
	Data    map[string]any // structured data (optional)
}

// Adapter executes a command with given parameters.
type Adapter interface {
	Execute(ctx context.Context, cmd *pkg.Command, params map[string]any, packageDir string) (*Result, error)
}

// New creates an adapter based on the manifest's adapter type.
func New(adapterType string) (Adapter, error) {
	switch adapterType {
	case "openapi":
		return &OpenAPIAdapter{}, nil
	case "cli":
		return &CLIAdapter{}, nil
	case "script":
		return &ScriptAdapter{}, nil
	case "pipeline":
		return &PipelineAdapter{}, nil
	default:
		return nil, fmt.Errorf("unknown adapter type: %s", adapterType)
	}
}
