# anyclaw

Agent protocol reverse proxy. Make any HTTP API callable by agent platforms via ACP, MCP, or HTTP.

```
Agent Platforms                    anyclaw                         Your API
                          ┌─────────────────────────┐
weclaw/Zed   ── ACP ──→  │                         │
Claude Code  ── MCP ──→  │  anyclaw (reverse proxy) │ ──→  HTTP API
Web Apps     ── HTTP ──→  │                         │
                          └─────────────────────────┘
```

## Install

One-line install (macOS / Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/fastclaw-ai/anyclaw/main/.github/install.sh | bash
```

Or via `go install`:

```bash
go install github.com/fastclaw-ai/anyclaw@latest
```

Or build from source:

```bash
git clone https://github.com/fastclaw-ai/anyclaw.git
cd anyclaw
go build -o anyclaw .
```

## Update

```bash
anyclaw update
# or
anyclaw upgrade
```

## Quick Start

### 1. Write a config file

```yaml
# translator.yaml
name: translator
description: "Translation service"

backend:
  type: http
  base_url: https://api.mymemory.translated.net

skills:
  - name: translate
    description: "Translate text between languages"
    input:
      q:
        type: string
        required: true
        description: "Text to translate"
      langpair:
        type: string
        required: true
        description: "Language pair (e.g. en|zh)"
    backend:
      method: GET
      path: /get
```

### 2. Run in your preferred mode

```bash
# MCP server (for Claude Code / Cursor)
anyclaw mcp --config translator.yaml

# ACP agent (for weclaw / Zed)
anyclaw acp --config translator.yaml

# HTTP API (for any web app)
anyclaw http --config translator.yaml --port 8080
```

### 3. Generate SKILL.md

```bash
anyclaw gen --config translator.yaml
```

## Integration

### Claude Code / Cursor

Add to your MCP settings:

```json
{
  "mcpServers": {
    "translator": {
      "command": "anyclaw",
      "args": ["mcp", "--config", "/path/to/translator.yaml"]
    }
  }
}
```

### weclaw

Add to `~/.weclaw/config.json`:

```json
{
  "agents": {
    "translator": {
      "type": "acp",
      "command": "anyclaw",
      "args": ["acp", "--config", "/path/to/translator.yaml"]
    }
  }
}
```

### HTTP

```bash
anyclaw http --config translator.yaml --port 8080

# List skills
curl http://localhost:8080/skills

# Execute a skill
curl -X POST http://localhost:8080/skills/translate/execute \
  -H "Content-Type: application/json" \
  -d '{"q": "Hello", "langpair": "en|zh"}'
```

## Config Format

anyclaw supports YAML, TOML, and JSON config files.

```yaml
name: my-service
description: "Service description"

backend:
  type: http
  base_url: https://api.example.com
  auth:
    type: bearer          # bearer, basic, or api_key
    token_env: API_KEY    # reads token from this env var

skills:
  - name: skill-name
    description: "What this skill does"
    input:
      param1:
        type: string
        required: true
        description: "Parameter description"
    backend:
      method: POST
      path: /api/endpoint
```

## Architecture

```
main.go
cmd/                        # CLI commands (cobra)
  acp.go                    # anyclaw acp
  mcp.go                    # anyclaw mcp
  http.go                   # anyclaw http
  gen.go                    # anyclaw gen
internal/
  config/                   # Config loading (viper)
  core/                     # Router: skill lookup + dispatch
  frontend/
    acp/                    # ACP JSON-RPC over stdio
    mcp/                    # MCP server via mcp-go
    httpapi/                # REST API server
  backend/
    http/                   # HTTP backend client
  gen/                      # SKILL.md generator
```

## License

[MIT](LICENSE)
