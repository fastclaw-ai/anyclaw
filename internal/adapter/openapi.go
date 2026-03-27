package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
)

// OpenAPIAdapter executes commands via HTTP API calls.
type OpenAPIAdapter struct{}

func (a *OpenAPIAdapter) Execute(ctx context.Context, cmd *pkg.Command, params map[string]any, packageDir string) (*Result, error) {
	if cmd.HTTP == nil {
		return nil, fmt.Errorf("command %q missing http config", cmd.Name)
	}

	method := strings.ToUpper(cmd.HTTP.Method)
	if method == "" {
		method = "GET"
	}

	path := cmd.HTTP.Path
	// Substitute path parameters
	remaining := make(map[string]any, len(params))
	for k, v := range params {
		placeholder := "{" + k + "}"
		if strings.Contains(path, placeholder) {
			path = strings.ReplaceAll(path, placeholder, fmt.Sprintf("%v", v))
		} else {
			remaining[k] = v
		}
	}

	url := strings.TrimRight(cmd.HTTP.BaseURL, "/") + path

	var body io.Reader
	if method == "POST" || method == "PUT" || method == "PATCH" {
		data, err := json.Marshal(remaining)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		body = bytes.NewReader(data)
	} else if len(remaining) > 0 {
		u, _ := neturl.Parse(url)
		q := u.Query()
		for k, v := range remaining {
			q.Set(k, fmt.Sprintf("%v", v))
		}
		u.RawQuery = q.Encode()
		url = u.String()
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply authentication
	if cmd.HTTP.Auth != nil {
		applyAuth(req, cmd.HTTP.Auth)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var data map[string]any
	if err = json.Unmarshal(respBody, &data); err == nil {
		pretty, _ := json.MarshalIndent(data, "", "  ")
		return &Result{Content: string(pretty), Data: data}, nil
	}

	return &Result{Content: string(respBody)}, nil
}

func applyAuth(req *http.Request, auth *pkg.Auth) {
	// Try credentials file first, then env var
	var token string
	if creds, err := pkg.LoadCredentials(); err == nil {
		token = creds.Get(auth.TokenEnv)
	}
	if token == "" {
		token = os.Getenv(auth.TokenEnv)
	}
	if token == "" {
		return
	}

	switch auth.Type {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+token)
	case "basic":
		req.Header.Set("Authorization", "Basic "+token)
	case "api_key":
		header := auth.Header
		if header == "" {
			header = "X-API-Key"
		}
		// api_key: user provides the full value (e.g. "Key xxx" or just "xxx")
		req.Header.Set(header, token)
	}
}
