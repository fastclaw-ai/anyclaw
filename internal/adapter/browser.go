package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync/atomic"
	"time"
)

const defaultDaemonPort = 19825

var cmdCounter int64

// BridgeStatus checks if the AnyClaw browser extension is connected.
func BridgeStatus() (connected bool, version string) {
	port := daemonPort()
	resp, err := bridgeGet(port, "/status")
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()

	var status struct {
		Running            bool   `json:"running"`
		ExtensionConnected bool   `json:"extensionConnected"`
		ExtensionVersion   string `json:"extensionVersion"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return false, ""
	}
	return status.ExtensionConnected, status.ExtensionVersion
}

// BridgeNavigate opens a URL in the browser via the extension bridge.
func BridgeNavigate(url string) error {
	_, err := bridgeCommand("navigate", map[string]any{"url": url})
	return err
}

// BridgeEvaluate executes JavaScript in the browser via the extension bridge.
func BridgeEvaluate(script string) (any, error) {
	result, err := bridgeCommand("exec", map[string]any{
		"code": script,
	})
	if err != nil {
		return nil, err
	}

	// Extract the result value
	if data, ok := result["data"]; ok {
		return data, nil
	}
	return result, nil
}

func bridgeCommand(action string, params map[string]any) (map[string]any, error) {
	port := daemonPort()
	id := fmt.Sprintf("cmd_%d_%d", time.Now().UnixMilli(), atomic.AddInt64(&cmdCounter, 1))

	body := map[string]any{
		"id":     id,
		"action": action,
	}
	for k, v := range params {
		body[k] = v
	}

	data, _ := json.Marshal(body)
	resp, err := bridgePost(port, "/command", data)
	if err != nil {
		return nil, fmt.Errorf("bridge command %q: %w", action, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("bridge response: %w", err)
	}

	if ok, _ := result["ok"].(bool); !ok {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = string(respBody)
		}
		return nil, fmt.Errorf("bridge error: %s", errMsg)
	}

	return result, nil
}

func bridgeGet(port int, path string) (*http.Response, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
	req.Header.Set("X-OpenCLI", "1")
	return client.Do(req)
}

func bridgePost(port int, path string, body []byte) (*http.Response, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d%s", port, path), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenCLI", "1")
	return client.Do(req)
}

func daemonPort() int {
	if env := os.Getenv("ANYCLAW_DAEMON_PORT"); env != "" {
		if p, err := strconv.Atoi(env); err == nil {
			return p
		}
	}
	return defaultDaemonPort
}

// EnsureDaemon starts the daemon if it's not already running.
func EnsureDaemon() {
	port := daemonPort()
	// Check if daemon is already reachable
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/status", port))
	if err == nil {
		resp.Body.Close()
		return // daemon already running
	}

	// Find anyclaw binary path
	exe, err := os.Executable()
	if err != nil {
		return
	}

	// Start daemon in background
	cmd := exec.Command(exe, "daemon", "start", "--port", strconv.Itoa(port))
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	// Detach from parent process
	cmd.SysProcAttr = nil
	if err := cmd.Start(); err != nil {
		return
	}

	// Save PID
	WriteDaemonPID(cmd.Process.Pid)

	// Release the process so it runs independently
	cmd.Process.Release()

	// Wait briefly for daemon to start
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/status", port))
		if err == nil {
			resp.Body.Close()
			return
		}
	}
}
