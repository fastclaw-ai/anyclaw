package mcp

import (
	"context"
	"fmt"
	"os"

	"github.com/fastclaw-ai/anyclaw/internal/adapter"
	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/fastclaw-ai/anyclaw/internal/core"
	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server implements the MCP protocol, exposing skills as MCP tools.
type Server struct {
	router    *core.Router // for legacy --config mode
	mcpServer *server.MCPServer
}

// NewServer creates an MCP server from a router (legacy --config mode).
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

// NewServerFromPackages creates an MCP server from all installed packages.
func NewServerFromPackages(store *pkg.Store) (*Server, error) {
	manifests, err := store.List()
	if err != nil {
		return nil, err
	}
	return NewServerFromManifests(store, manifests)
}

// NewServerFromManifests creates an MCP server from the given manifests.
func NewServerFromManifests(store *pkg.Store, manifests []*pkg.Manifest) (*Server, error) {
	s := server.NewMCPServer("anyclaw", "0.1.0",
		server.WithToolCapabilities(true),
	)

	srv := &Server{mcpServer: s}

	for _, m := range manifests {
		a, err := adapter.New(m.InferAdapter())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skip package %s: %v\n", m.Name, err)
			continue
		}
		packageDir := store.PackageDir(m.Name)
		for i := range m.Commands {
			srv.registerPackageCommand(m.Name, &m.Commands[i], a, packageDir)
		}
	}

	return srv, nil
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

func (s *Server) registerPackageCommand(pkgName string, cmd *pkg.Command, a adapter.Adapter, packageDir string) {
	toolName := pkgName + "_" + cmd.Name

	opts := []mcp.ToolOption{
		mcp.WithDescription(cmd.Description),
	}

	for name, arg := range cmd.Args {
		propOpts := []mcp.PropertyOption{
			mcp.Description(arg.Description),
		}
		if arg.Required {
			propOpts = append(propOpts, mcp.Required())
		}
		if arg.Default != "" {
			propOpts = append(propOpts, mcp.DefaultString(arg.Default))
		}
		opts = append(opts, mcp.WithString(name, propOpts...))
	}

	tool := mcp.NewTool(toolName, opts...)

	// Capture for closure
	command := cmd
	s.mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := request.GetArguments()
		if params == nil {
			params = make(map[string]any)
		}

		result, err := a.Execute(ctx, command, params, packageDir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		return mcp.NewToolResultText(result.Content), nil
	})
}

// Serve starts the MCP server over stdio.
func (s *Server) Serve(ctx context.Context) error {
	stdio := server.NewStdioServer(s.mcpServer)
	return stdio.Listen(ctx, os.Stdin, os.Stdout)
}
