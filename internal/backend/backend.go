package backend

import (
	"context"

	"github.com/fastclaw-ai/anyclaw/internal/config"
)

// Response is the result of a backend execution.
type Response struct {
	Content string         // text content
	Data    map[string]any // structured data (optional)
}

// Backend is the interface for executing skill calls against a target service.
type Backend interface {
	Execute(ctx context.Context, skill *config.Skill, params map[string]any) (*Response, error)
}
