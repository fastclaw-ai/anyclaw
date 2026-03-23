package core

import (
	"context"
	"fmt"

	"github.com/fastclaw-ai/anyclaw/internal/backend"
	"github.com/fastclaw-ai/anyclaw/internal/config"
)

// Router maps skill invocations to backend calls.
type Router struct {
	config  *config.Config
	backend backend.Backend
	skills  map[string]*config.Skill
}

// NewRouter creates a router from config and backend.
func NewRouter(cfg *config.Config, b backend.Backend) *Router {
	skills := make(map[string]*config.Skill, len(cfg.Skills))
	for i := range cfg.Skills {
		skills[cfg.Skills[i].Name] = &cfg.Skills[i]
	}
	return &Router{
		config:  cfg,
		backend: b,
		skills:  skills,
	}
}

// ListSkills returns all configured skills.
func (r *Router) ListSkills() []config.Skill {
	return r.config.Skills
}

// SkillNames returns names of all configured skills.
func (r *Router) SkillNames() []string {
	names := make([]string, 0, len(r.config.Skills))
	for _, s := range r.config.Skills {
		names = append(names, s.Name)
	}
	return names
}

// Execute invokes a skill by name with the given parameters.
func (r *Router) Execute(ctx context.Context, skillName string, params map[string]any) (*backend.Response, error) {
	skill, ok := r.skills[skillName]
	if !ok {
		return nil, fmt.Errorf("unknown skill %q, available: %v", skillName, r.SkillNames())
	}
	return r.backend.Execute(ctx, skill, params)
}

// Config returns the underlying config.
func (r *Router) Config() *config.Config {
	return r.config
}
