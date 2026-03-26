package mcp

import (
	"context"
	"fmt"
	"os"

	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/fastclaw-ai/anyclaw/internal/core"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server implements the MCP protocol, exposing skills as MCP tools.
type Server struct {
	router    *core.Router
	mcpServer *server.MCPServer
}

// NewServer creates an MCP server from a router.
func NewServer(router *core.Router) *Server {
	cfg := router.Config()
	s := server.NewMCPServer(cfg.Name, "0.1.0",
		server.WithToolCapabilities(true),
	)

	srv := &Server{
		router:    router,
		mcpServer: s,
	}

	for _, skill := range router.ListSkills() {
		srv.registerSkill(skill)
	}

	return srv
}

func (s *Server) registerSkill(skill config.Skill) {
	opts := []mcp.ToolOption{
		mcp.WithDescription(skill.Description),
	}

	for name, field := range skill.Input {
		propOpts := []mcp.PropertyOption{
			mcp.Description(field.Description),
		}
		if field.Required {
			propOpts = append(propOpts, mcp.Required())
		}
		if field.Default != "" {
			propOpts = append(propOpts, mcp.DefaultString(field.Default))
		}
		opts = append(opts, mcp.WithString(name, propOpts...))
	}

	tool := mcp.NewTool(skill.Name, opts...)

	skillName := skill.Name
	s.mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := request.GetArguments()
		if params == nil {
			params = make(map[string]any)
		}

		resp, err := s.router.Execute(ctx, skillName, params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		return mcp.NewToolResultText(resp.Content), nil
	})
}

// Serve starts the MCP server over stdio.
func (s *Server) Serve(ctx context.Context) error {
	stdio := server.NewStdioServer(s.mcpServer)
	return stdio.Listen(ctx, os.Stdin, os.Stdout)
}
