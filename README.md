# anyclaw

Agent protocol reverse proxy. Make any HTTP API callable by agent platforms via MCP.

```
Agent Platforms                    anyclaw                         Your API
                          ┌─────────────────────────┐
Claude Code  ── MCP ──→  │                         │
Cursor       ── MCP ──→  │  anyclaw (reverse proxy) │ ──→  HTTP API
Any MCP Host ── MCP ──→  │                         │
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

## Quick Start

### 1. Write an OpenAPI spec

anyclaw uses standard [OpenAPI 3.x](https://spec.openapis.org/oas/v3.0.4.html) as its config format. Any existing OpenAPI spec works out of the box.

```yaml
# translator.yaml
openapi: 3.0.0
info:
  title: Translator
  description: Translation service
  version: "1.0"

servers:
  - url: https://api.mymemory.translated.net

paths:
  /get:
    get:
      operationId: translate
      summary: Translate text between languages
      parameters:
        - name: q
          in: query
          required: true
          description: Text to translate
          schema:
            type: string
        - name: langpair
          in: query
          required: true
          description: "Language pair (e.g. en|zh)"
          schema:
            type: string
            default: "en|zh"
```

### 2. Start MCP server

```bash
anyclaw mcp --config translator.yaml
```

### 3. Generate SKILL.md

```bash
anyclaw skill --config translator.yaml
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

## Commands

| Command | Description |
|---------|-------------|
| `anyclaw mcp -c spec.yaml` | Start MCP server (stdio) |
| `anyclaw skill -c spec.yaml` | Generate SKILL.md from spec |
| `anyclaw skill -c spec.yaml -o dir/` | Generate SKILL.md to custom path |
| `anyclaw version` | Print version info |
| `anyclaw update` | Self-update to latest release |
| `anyclaw upgrade` | Alias for `update` |

## Config Format

anyclaw uses **OpenAPI 3.x** as the only config format. Each `path + method` becomes an MCP tool.

| OpenAPI | anyclaw |
|---------|---------|
| `info.title` | Server name |
| `servers[0].url` | Backend base URL |
| `operationId` | Tool name |
| `parameters` | Tool input fields |
| `requestBody` | Tool input fields (POST) |
| `components.securitySchemes` | Auth config |
| Path params `{id}` | Auto-substituted at runtime |

### Authentication

```yaml
components:
  securitySchemes:
    bearer_auth:
      type: http
      scheme: bearer    # reads token from $API_TOKEN env var
    api_key:
      type: apiKey
      name: X-API-Key   # header name
      in: header        # reads key from $API_KEY env var
```

## Architecture

```
main.go
cmd/
  mcp.go                    # anyclaw mcp
  gen.go                    # anyclaw skill (generate SKILL.md)
  version.go                # anyclaw version
  update.go                 # anyclaw update/upgrade
internal/
  config/                   # OpenAPI spec parsing
    openapi.go              # OpenAPI 3.x → internal Config
  core/                     # Router: skill lookup + dispatch
  frontend/
    mcp/                    # MCP server via mcp-go
  backend/
    http/                   # HTTP backend client
  gen/                      # SKILL.md generator
  version/                  # Build-time version info
```

## License

[MIT](LICENSE)
