package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fastclaw-ai/anyclaw/internal/core"
)

// Server implements a REST API frontend.
type Server struct {
	router *core.Router
	addr   string
}

// NewServer creates an HTTP API server.
func NewServer(router *core.Router, port int) *Server {
	return &Server{
		router: router,
		addr:   fmt.Sprintf(":%d", port),
	}
}

// Serve starts the HTTP server.
func (s *Server) Serve(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /skills", s.handleListSkills)
	mux.HandleFunc("POST /skills/{name}/execute", s.handleExecuteSkill)

	srv := &http.Server{Addr: s.addr, Handler: mux}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	fmt.Printf("anyclaw HTTP server listening on %s\n", s.addr)
	return srv.ListenAndServe()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	skills := s.router.ListSkills()
	type skillInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	var list []skillInfo
	for _, sk := range skills {
		list = append(list, skillInfo{Name: sk.Name, Description: sk.Description})
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": list})
}

func (s *Server) handleExecuteSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var params map[string]any
	if r.Body != nil {
		defer r.Body.Close()
		json.NewDecoder(r.Body).Decode(&params)
	}
	if params == nil {
		params = make(map[string]any)
	}

	resp, err := s.router.Execute(r.Context(), name, params)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if resp.Data != nil {
		writeJSON(w, http.StatusOK, resp.Data)
	} else {
		writeJSON(w, http.StatusOK, map[string]string{"result": resp.Content})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
