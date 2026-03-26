package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/fastclaw-ai/anyclaw/internal/core"
	"github.com/google/uuid"
)

// Server implements an OpenAI-compatible HTTP API frontend.
type Server struct {
	router *core.Router
	addr   string
}

// NewServer creates an OpenAI-compatible API server.
func NewServer(router *core.Router, port int) *Server {
	return &Server{
		router: router,
		addr:   fmt.Sprintf(":%d", port),
	}
}

// Serve starts the HTTP server.
func (s *Server) Serve(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/models", s.handleModels)
	mux.HandleFunc("GET /v1/tools", s.handleTools)
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)

	srv := &http.Server{Addr: s.addr, Handler: mux}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	fmt.Printf("anyclaw OpenAI-compatible server listening on %s\n", s.addr)
	return srv.ListenAndServe()
}

// --- OpenAI request/response types ---

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []tool        `json:"tools,omitempty"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type tool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type toolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function toolCallFunction `json:"function"`
}

type toolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []choice       `json:"choices"`
	Usage   map[string]int `json:"usage"`
}

type choice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// --- Handlers ---

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	skills := s.router.ListSkills()
	tools := make([]tool, 0, len(skills))
	for _, sk := range skills {
		tools = append(tools, skillToToolDef(sk))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	cfg := s.router.Config()
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []map[string]any{
			{
				"id":       cfg.Name,
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "anyclaw",
			},
		},
	})
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "Invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "messages is required")
		return
	}

	// Check if last message is a tool result -> execute the skill
	last := req.Messages[len(req.Messages)-1]
	if last.Role == "tool" && last.ToolCallID != "" {
		// Find the assistant message with the matching tool_call to get skill name
		skillName, params := s.extractToolResult(req.Messages)
		if skillName == "" {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "cannot resolve tool call")
			return
		}
		_ = params // tool result content is in last.Content, but we already executed
		// The tool result is provided by the client; just echo it as assistant response
		s.writeCompletion(w, req.Model, last.Content, "stop")
		return
	}

	// Check if last message is from user -> respond with tool_calls for matching skill
	if last.Role == "user" {
		// Try to parse user message as JSON skill invocation: {"skill": "name", "params": {...}}
		var invocation struct {
			Skill  string         `json:"skill"`
			Params map[string]any `json:"params"`
		}
		if err := json.Unmarshal([]byte(last.Content), &invocation); err == nil && invocation.Skill != "" {
			// Direct execution mode: parse and execute
			resp, err := s.router.Execute(r.Context(), invocation.Skill, invocation.Params)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
				return
			}
			s.writeCompletion(w, req.Model, resp.Content, "stop")
			return
		}

		// Try to match user message as "skillName param1=val1 param2=val2" or just execute if single skill
		skills := s.router.ListSkills()

		// If there's only one skill, return a tool_call for it
		if len(skills) == 1 {
			skill := skills[0]
			args := map[string]any{}
			// Put the user message as the first required string field
			for name, field := range skill.Input {
				if field.Required {
					args[name] = last.Content
					break
				}
			}
			s.writeToolCallResponse(w, req.Model, skill.Name, args)
			return
		}

		// Multiple skills: return available tools so the client knows what to call
		s.writeToolCallList(w, req.Model, skills)
		return
	}

	// Default: return available tools as function definitions
	s.writeToolCallList(w, req.Model, s.router.ListSkills())
}


func (s *Server) extractToolResult(messages []chatMessage) (string, map[string]any) {
	// Walk backwards to find the assistant message with tool_calls
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			tc := msg.ToolCalls[0]
			var params map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &params)
			return tc.Function.Name, params
		}
	}
	return "", nil
}

func (s *Server) writeCompletion(w http.ResponseWriter, model, content, finishReason string) {
	if model == "" {
		model = s.router.Config().Name
	}
	writeJSON(w, http.StatusOK, chatResponse{
		ID:      "chatcmpl-" + uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []choice{
			{
				Index: 0,
				Message: chatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: map[string]int{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
	})
}

func (s *Server) writeToolCallResponse(w http.ResponseWriter, model, skillName string, args map[string]any) {
	if model == "" {
		model = s.router.Config().Name
	}
	argsJSON, _ := json.Marshal(args)
	writeJSON(w, http.StatusOK, chatResponse{
		ID:      "chatcmpl-" + uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []choice{
			{
				Index: 0,
				Message: chatMessage{
					Role: "assistant",
					ToolCalls: []toolCall{
						{
							ID:   "call_" + uuid.New().String(),
							Type: "function",
							Function: toolCallFunction{
								Name:      skillName,
								Arguments: string(argsJSON),
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: map[string]int{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
	})
}

func (s *Server) writeToolCallList(w http.ResponseWriter, model string, skills []config.Skill) {
	if model == "" {
		model = s.router.Config().Name
	}
	var buf strings.Builder
	buf.WriteString("Available tools:\n")
	for _, sk := range skills {
		fmt.Fprintf(&buf, "- %s: %s\n", sk.Name, sk.Description)
	}
	buf.WriteString("\nSend a message with {\"skill\": \"<name>\", \"params\": {...}} to invoke a tool.")

	writeJSON(w, http.StatusOK, chatResponse{
		ID:      "chatcmpl-" + uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []choice{
			{
				Index: 0,
				Message: chatMessage{
					Role:    "assistant",
					Content: buf.String(),
				},
				FinishReason: "stop",
			},
		},
		Usage: map[string]int{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
	})
}

func skillToToolDef(skill config.Skill) tool {
	props := map[string]any{}
	var required []string
	for name, field := range skill.Input {
		props[name] = map[string]any{
			"type":        field.Type,
			"description": field.Description,
		}
		if field.Required {
			required = append(required, name)
		}
	}
	params, _ := json.Marshal(map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	})
	return tool{
		Type: "function",
		Function: toolFunction{
			Name:        skill.Name,
			Description: skill.Description,
			Parameters:  params,
		},
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, errType, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    errType,
		},
	})
}
