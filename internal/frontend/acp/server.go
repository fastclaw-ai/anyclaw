package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/fastclaw-ai/anyclaw/internal/core"
)

// Server implements the ACP agent protocol over stdin/stdout ndjson.
type Server struct {
	router  *core.Router
	handler *handler
	writer  io.Writer
}

// NewServer creates an ACP server.
func NewServer(router *core.Router) *Server {
	return &Server{
		router:  router,
		handler: newHandler(router),
		writer:  os.Stdout,
	}
}

// Serve starts the ACP server, reading from stdin and writing to stdout.
func (s *Server) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			fmt.Fprintf(os.Stderr, "[anyclaw-acp] failed to parse: %v\n", err)
			continue
		}

		s.dispatch(ctx, &req)
	}

	return scanner.Err()
}

func (s *Server) dispatch(ctx context.Context, req *rpcRequest) {
	switch req.Method {
	case "initialize":
		result, err := s.handler.handleInitialize(req.ID, req.Params)
		s.sendResponse(req.ID, result, err)

	case "session/new":
		result, err := s.handler.handleNewSession(req.Params)
		s.sendResponse(req.ID, result, err)

	case "session/prompt":
		notify := func(n rpcNotification) {
			s.sendNotification(n)
		}
		result, err := s.handler.handlePrompt(ctx, req.Params, notify)
		s.sendResponse(req.ID, result, err)

	case "cancel":
		// Acknowledge cancel
		s.sendResponse(req.ID, map[string]any{}, nil)

	case "session/request_permission":
		// This is a request FROM the agent, not applicable here.
		// If we receive it as a method, ignore.

	case "setSessionMode", "session/setMode":
		s.sendResponse(req.ID, map[string]any{}, nil)

	case "session/close", "unstable/closeSession":
		s.sendResponse(req.ID, map[string]any{}, nil)

	case "session/list", "unstable/listSessions":
		s.sendResponse(req.ID, map[string]any{"sessions": []any{}}, nil)

	default:
		fmt.Fprintf(os.Stderr, "[anyclaw-acp] unhandled method: %s\n", req.Method)
		s.sendResponse(req.ID, nil, fmt.Errorf("method not found: %s", req.Method))
	}
}

func (s *Server) sendResponse(id json.RawMessage, result any, err error) {
	resp := rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
	}

	if err != nil {
		resp.Error = &rpcError{
			Code:    -32603,
			Message: err.Error(),
		}
	} else {
		resp.Result = result
	}

	s.writeLine(resp)
}

func (s *Server) sendNotification(n rpcNotification) {
	s.writeLine(n)
}

func (s *Server) writeLine(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[anyclaw-acp] marshal error: %v\n", err)
		return
	}
	fmt.Fprintf(s.writer, "%s\n", data)
}
