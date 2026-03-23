package http

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

	"github.com/fastclaw-ai/anyclaw/internal/backend"
	"github.com/fastclaw-ai/anyclaw/internal/config"
)

// Client implements backend.Backend for HTTP APIs.
type Client struct {
	baseURL    string
	auth       *config.Auth
	httpClient *http.Client
}

// NewClient creates a new HTTP backend client.
func NewClient(cfg *config.Backend) *Client {
	return &Client{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		auth:       cfg.Auth,
		httpClient: &http.Client{},
	}
}

// Execute calls the HTTP API for the given skill.
func (c *Client) Execute(ctx context.Context, skill *config.Skill, params map[string]any) (*backend.Response, error) {
	method := strings.ToUpper(skill.Backend.Method)
	if method == "" {
		method = "POST"
	}

	url := c.baseURL + skill.Backend.Path

	var body io.Reader
	if method == "POST" || method == "PUT" || method == "PATCH" {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		body = bytes.NewReader(data)
	} else if len(params) > 0 {
		u, _ := neturl.Parse(url)
		q := u.Query()
		for k, v := range params {
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

	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
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
	if err := json.Unmarshal(respBody, &data); err == nil {
		pretty, _ := json.MarshalIndent(data, "", "  ")
		return &backend.Response{
			Content: string(pretty),
			Data:    data,
		}, nil
	}

	return &backend.Response{
		Content: string(respBody),
	}, nil
}

func (c *Client) applyAuth(req *http.Request) {
	if c.auth == nil {
		return
	}

	token := os.Getenv(c.auth.TokenEnv)
	if token == "" {
		return
	}

	switch c.auth.Type {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+token)
	case "basic":
		req.Header.Set("Authorization", "Basic "+token)
	case "api_key":
		header := c.auth.Header
		if header == "" {
			header = "X-API-Key"
		}
		req.Header.Set(header, token)
	}
}
