package acp

import "encoding/json"

// JSON-RPC 2.0 types

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any         `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ACP protocol types

type initializeParams struct {
	ProtocolVersion    int                `json:"protocolVersion"`
	ClientCapabilities clientCapabilities `json:"clientCapabilities"`
}

type clientCapabilities struct {
	FS   *fsCapabilities `json:"fs,omitempty"`
	Auth *authCaps       `json:"auth,omitempty"`
}

type fsCapabilities struct {
	ReadTextFile  bool `json:"readTextFile"`
	WriteTextFile bool `json:"writeTextFile"`
}

type authCaps struct {
	Terminal bool `json:"terminal,omitempty"`
}

type initializeResult struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities agentCapabilities `json:"agentCapabilities"`
	AgentInfo         agentInfo         `json:"agentInfo"`
	AuthMethods       []any             `json:"authMethods"`
}

type agentCapabilities struct {
	PromptCapabilities promptCapabilities `json:"promptCapabilities"`
}

type promptCapabilities struct {
	Image           bool `json:"image,omitempty"`
	EmbeddedContext bool `json:"embeddedContext,omitempty"`
}

type agentInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

type newSessionParams struct {
	Cwd        string `json:"cwd"`
	McpServers []any  `json:"mcpServers"`
}

type newSessionResult struct {
	SessionID string `json:"sessionId"`
}

type promptParams struct {
	SessionID string        `json:"sessionId"`
	Prompt    []promptEntry `json:"prompt"`
}

type promptEntry struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type promptResult struct {
	StopReason string `json:"stopReason"`
}

type sessionUpdateParams struct {
	SessionID string       `json:"sessionId"`
	Update    sessionUpdate `json:"update"`
}

type sessionUpdate struct {
	SessionUpdate string `json:"sessionUpdate"`
	Content       any    `json:"content,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type cancelParams struct {
	SessionID string `json:"sessionId"`
}

type permissionRequestParams struct {
	ToolCall json.RawMessage `json:"toolCall"`
	Options  []permissionOption `json:"options"`
}

type permissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}
