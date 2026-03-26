package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/fastclaw-ai/anyclaw/internal/core"
	"github.com/fastclaw-ai/anyclaw/internal/version"
	"github.com/google/uuid"
)

type session struct {
	id  string
	cwd string
}

type handler struct {
	router   *core.Router
	sessions map[string]*session
}

func newHandler(router *core.Router) *handler {
	return &handler{
		router:   router,
		sessions: make(map[string]*session),
	}
}

func (h *handler) handleInitialize(id json.RawMessage, params json.RawMessage) (any, error) {
	cfg := h.router.Config()
	return initializeResult{
		ProtocolVersion: 1,
		AgentCapabilities: agentCapabilities{
			PromptCapabilities: promptCapabilities{},
		},
		AgentInfo: agentInfo{
			Name:    cfg.Name,
			Title:   cfg.Description,
			Version: version.Version,
		},
		AuthMethods: []any{},
	}, nil
}

func (h *handler) handleNewSession(params json.RawMessage) (any, error) {
	var p newSessionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parse newSession params: %w", err)
	}

	sid := uuid.New().String()
	h.sessions[sid] = &session{id: sid, cwd: p.Cwd}

	return newSessionResult{SessionID: sid}, nil
}

func (h *handler) handlePrompt(ctx context.Context, params json.RawMessage, notify func(notification rpcNotification)) (any, error) {
	var p promptParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parse prompt params: %w", err)
	}

	// Extract text from prompt
	var message string
	for _, entry := range p.Prompt {
		if entry.Type == "text" && entry.Text != "" {
			message = entry.Text
			break
		}
	}

	if message == "" {
		return promptResult{StopReason: "end_turn"}, nil
	}

	// Resolve skill and params from message
	skillName, skillParams := h.resolveSkill(message)

	// Execute
	resp, err := h.router.Execute(ctx, skillName, skillParams)
	if err != nil {
		// Send error as message chunk
		notify(rpcNotification{
			JSONRPC: "2.0",
			Method:  "session/update",
			Params: sessionUpdateParams{
				SessionID: p.SessionID,
				Update: sessionUpdate{
					SessionUpdate: "agent_message_chunk",
					Content:       textContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
				},
			},
		})
		return promptResult{StopReason: "end_turn"}, nil
	}

	// Send result as message chunk
	notify(rpcNotification{
		JSONRPC: "2.0",
		Method:  "session/update",
		Params: sessionUpdateParams{
			SessionID: p.SessionID,
			Update: sessionUpdate{
				SessionUpdate: "agent_message_chunk",
				Content:       textContent{Type: "text", Text: resp.Content},
			},
		},
	})

	return promptResult{StopReason: "end_turn"}, nil
}

// resolveSkill maps a natural language message to a skill name and parameters.
func (h *handler) resolveSkill(message string) (string, map[string]any) {
	skills := h.router.ListSkills()

	// Try JSON params first: {"skill": "translate", "params": {...}}
	var structured struct {
		Skill  string         `json:"skill"`
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal([]byte(message), &structured); err == nil && structured.Skill != "" {
		return structured.Skill, structured.Params
	}

	// Single skill: map message to its input fields
	if len(skills) == 1 {
		return skills[0].Name, buildParams(skills[0], message)
	}

	// Try to match skill name at start of message
	lower := strings.ToLower(message)
	for _, s := range skills {
		if strings.HasPrefix(lower, strings.ToLower(s.Name)) {
			rest := strings.TrimSpace(message[len(s.Name):])
			return s.Name, buildParams(s, rest)
		}
	}

	// Fallback: use first skill
	names := h.router.SkillNames()
	return skills[0].Name, map[string]any{
		"text": fmt.Sprintf("Available skills: %s. Message: %s", strings.Join(names, ", "), message),
	}
}

// buildParams maps a plain text message to a skill's input fields.
// It assigns the message to the first required string field, and fills
// any other fields that have defaults.
func buildParams(skill config.Skill, message string) map[string]any {
	params := make(map[string]any)

	// Fill defaults first
	for name, field := range skill.Input {
		if field.Default != "" {
			params[name] = field.Default
		}
	}

	// Assign message to the first required string field
	for name, field := range skill.Input {
		if field.Required && field.Type == "string" {
			if _, hasDefault := params[name]; !hasDefault {
				params[name] = message
				break
			}
		}
	}

	return params
}
