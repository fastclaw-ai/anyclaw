package adapter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Daemon bridges the browser extension (WebSocket) and CLI commands (HTTP).
// Bridges the AnyClaw browser extension and CLI commands.
type Daemon struct {
	port     int
	mu       sync.Mutex
	extConn  *websocket.Conn // connection to browser extension
	extVer   string
	pending  map[string]chan map[string]any // command ID → response channel
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// NewDaemon creates a daemon on the given port.
func NewDaemon(port int) *Daemon {
	return &Daemon{
		port:    port,
		pending: make(map[string]chan map[string]any),
	}
}

// Start starts the daemon HTTP/WebSocket server.
func (d *Daemon) Start() error {
	mux := http.NewServeMux()

	// WebSocket endpoint for browser extension
	mux.HandleFunc("/ws", d.handleExtensionWS)

	// HTTP endpoints for CLI
	mux.HandleFunc("/status", d.handleStatus)
	mux.HandleFunc("/command", d.handleCommand)

	addr := fmt.Sprintf("127.0.0.1:%d", d.port)
	fmt.Fprintf(os.Stderr, "[anyclaw] Daemon listening on %s\n", addr)

	return http.ListenAndServe(addr, mux)
}

// handleExtensionWS handles WebSocket connection from browser extension.
func (d *Daemon) handleExtensionWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	d.mu.Lock()
	d.extConn = conn
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.extConn = nil
		d.extVer = ""
		d.mu.Unlock()
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var data map[string]any
		if err := json.Unmarshal(msg, &data); err != nil {
			continue
		}

		// Handle hello message from extension
		if t, _ := data["type"].(string); t == "hello" {
			d.mu.Lock()
			d.extVer, _ = data["version"].(string)
			d.mu.Unlock()
			continue
		}

		// Handle log messages
		if t, _ := data["type"].(string); t == "log" {
			continue
		}

		// Handle command response (has "id" field)
		if id, ok := data["id"].(string); ok {
			d.mu.Lock()
			ch, exists := d.pending[id]
			if exists {
				delete(d.pending, id)
			}
			d.mu.Unlock()
			if exists {
				ch <- data
			}
		}
	}
}

// handleStatus returns daemon and extension connection status.
func (d *Daemon) handleStatus(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	connected := d.extConn != nil
	version := d.extVer
	pending := len(d.pending)
	d.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]any{
		"ok":                 true,
		"running":            true,
		"extensionConnected": connected,
		"extensionVersion":   version,
		"pending":            pending,
	})
}

// handleCommand receives a command from CLI, forwards to extension, waits for response.
func (d *Daemon) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var cmd map[string]any
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid JSON"})
		return
	}

	d.mu.Lock()
	conn := d.extConn
	d.mu.Unlock()

	if conn == nil {
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "Browser extension not connected"})
		return
	}

	// Get or generate command ID
	id, _ := cmd["id"].(string)
	if id == "" {
		id = fmt.Sprintf("cmd_%d", time.Now().UnixMilli())
		cmd["id"] = id
	}

	// Create response channel
	ch := make(chan map[string]any, 1)
	d.mu.Lock()
	d.pending[id] = ch
	d.mu.Unlock()

	// Forward to extension
	data, _ := json.Marshal(cmd)
	d.mu.Lock()
	err := conn.WriteMessage(websocket.TextMessage, data)
	d.mu.Unlock()
	if err != nil {
		d.mu.Lock()
		delete(d.pending, id)
		d.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "Failed to send to extension"})
		return
	}

	// Wait for response with timeout
	select {
	case resp := <-ch:
		json.NewEncoder(w).Encode(resp)
	case <-time.After(30 * time.Second):
		d.mu.Lock()
		delete(d.pending, id)
		d.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "Timeout waiting for extension response"})
	}
}

// WriteDaemonPID saves daemon PID to a file for management.
func WriteDaemonPID(pid int) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".anyclaw", "daemon.pid")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

// ReadDaemonPID reads the saved daemon PID.
func ReadDaemonPID() int {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".anyclaw", "daemon.pid")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(string(data))
	return pid
}

// RemoveDaemonPID removes the PID file.
func RemoveDaemonPID() {
	home, _ := os.UserHomeDir()
	os.Remove(filepath.Join(home, ".anyclaw", "daemon.pid"))
}
