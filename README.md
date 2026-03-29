# anyclaw

The universal tool adapter for AI agents. Turn any API, website, script into agent-ready tools via MCP, Skills, CLI and more.

```
  Sources                    anyclaw                       Outputs
                       ┌───────────────────┐
  OpenAPI spec    ──→  │                   │  ──→  CLI (anyclaw run)
  Pipeline YAML   ──→  │  install → run    │  ──→  MCP Server
  System CLI      ──→  │                   │  ──→  SKILL.md
  Script          ──→  │                   │
                       └───────────────────┘
```

## Install

One-line install (macOS / Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/fastclaw-ai/anyclaw/main/.github/install.sh | bash
```

Or build from source:

```bash
git clone https://github.com/fastclaw-ai/anyclaw.git
cd anyclaw && go build -o anyclaw .
```

## Quick Start

### Browse & search packages

```bash
# Browse all available packages from registry
anyclaw list --all

# Paginate through packages
anyclaw list --all --page 2
anyclaw list --all --size 50

# Search by keyword or tag
anyclaw search news
anyclaw search chinese
anyclaw search finance
```

### Install packages

```bash
# Install from registry (by name)
anyclaw install hackernews
anyclaw install translator

# Install from GitHub URL
anyclaw install https://github.com/Astro-Han/opencli-plugin-juejin

# Install from local file or directory
anyclaw install examples/openapi.yaml
anyclaw install examples/web.yaml
anyclaw install registry/packages/query-domains

# Wrap a system CLI tool
anyclaw install docker
anyclaw install gh
```

### List & manage installed packages

```bash
# List installed packages and their commands
anyclaw list

# Uninstall a package
anyclaw uninstall hackernews

# Set API key for a package that requires auth
anyclaw auth translator <your-api-key>
```

### Run commands

```bash
# Run with package and command (space-separated)
anyclaw run hackernews top --limit 5
anyclaw run translator translate --q hello --langpair "en|zh"
anyclaw run query-domains search --keyword anyclaw -a
anyclaw run query-domains whois --domain anyclaw.com

# Shorthand (package name as subcommand)
anyclaw hackernews top --limit 5
anyclaw gh pr list
anyclaw docker ps

# Show available commands for a package
anyclaw run hackernews
anyclaw hackernews --help

# Output as JSON instead of table
anyclaw reddit hot --limit 5 --json
```

### Export to MCP server

```bash
# Start MCP server exposing all installed packages as tools
anyclaw mcp

# Expose a single package
anyclaw mcp hackernews
```

Add to your Claude Code / Cursor MCP settings:

```json
{
  "mcpServers": {
    "anyclaw": {
      "command": "anyclaw",
      "args": ["mcp"]
    }
  }
}
```

Or expose a single package:

```json
{
  "mcpServers": {
    "hackernews": {
      "command": "anyclaw",
      "args": ["mcp", "hackernews"]
    }
  }
}
```

### Generate Skills for Claude Code

```bash
# Generate SKILL.md for all installed packages
anyclaw skills

# Generate for a specific package
anyclaw skills hackernews

# Generate to a custom directory
anyclaw skills hackernews -o ~/.claude/skills/hackernews
```

## Browser Extension

Some packages require browser access (e.g., Reddit, Bilibili). anyclaw includes its own browser extension and daemon for this.

### Setup

1. Open `chrome://extensions` in Chrome
2. Enable **Developer Mode**
3. Click **Load unpacked** → select the `extension/` directory from this repo

### Usage

The daemon starts automatically when a browser command is needed. You can also manage it manually:

```bash
anyclaw daemon start     # start daemon (foreground)
anyclaw daemon status    # check daemon & extension status
anyclaw daemon stop      # stop daemon
```

### How it works

```
Chrome (real browser, with cookies & login state)
    │
AnyClaw extension (background.js)
    │ WebSocket
    ▼
AnyClaw daemon (auto-started, port 19825)
    │ HTTP
    ▼
AnyClaw CLI (anyclaw run pkg cmd)
```

The extension runs commands in an isolated Chrome window, separate from your normal browsing. Sessions auto-close after 30 seconds of inactivity.

## Package Formats

anyclaw supports four YAML formats. See `examples/` for complete examples.

### OpenAPI spec (`examples/openapi.yaml`)

Standard OpenAPI 3.x — any existing spec works:

```yaml
openapi: 3.0.0
info:
  title: Translator
servers:
  - url: https://api.mymemory.translated.net
paths:
  /get:
    get:
      operationId: translate
      parameters:
        - name: q
          in: query
          required: true
          schema:
            type: string
```

### Pipeline (`examples/web.yaml`)

Declarative data pipeline for fetching web data:

```yaml
anyclaw: "1.0"
name: hackernews
description: Hacker News data tools
commands:
  - name: top
    description: Top stories
    args:
      limit:
        type: int
        default: 20
    pipeline:
      - fetch:
          url: https://hacker-news.firebaseio.com/v0/topstories.json
      - limit: ${{ args.limit }}
      - map:
          id: ${{ item }}
      - fetch:
          url: https://hacker-news.firebaseio.com/v0/item/${{ item.id }}.json
      - map:
          title: ${{ item.title }}
          score: ${{ item.score }}
```

### CLI wrapper (`examples/cli.yaml`)

Wrap existing CLI tools with templated commands:

```yaml
anyclaw: "1.0"
name: git-helper
description: Git shortcuts
commands:
  - name: recent
    description: Show recent commits
    args:
      count:
        type: int
        default: 10
    run: "git log --oneline -n {{count}}"
```

### Script (`examples/script.yaml`)

Inline Python or Node.js scripts:

```yaml
anyclaw: "1.0"
name: ip-tools
description: IP address utilities
commands:
  - name: myip
    description: Get your public IP
    script:
      runtime: python
      code: |
        import urllib.request, json
        data = json.load(urllib.request.urlopen("https://ipinfo.io/json"))
        print(json.dumps({"ip": data["ip"], "city": data["city"]}, indent=2))
```

## Commands

| Command | Description |
|---------|-------------|
| `anyclaw list` | List installed packages and commands |
| `anyclaw list --all` | Browse all available packages from registry |
| `anyclaw list --all --page N` | Paginate registry packages |
| `anyclaw search <keyword>` | Search packages by name, description, or tag |
| `anyclaw install <name\|url\|file\|dir>` | Install a package |
| `anyclaw uninstall <name>` | Remove a package |
| `anyclaw run <pkg> <cmd> [flags]` | Run a command |
| `anyclaw mcp [pkg]` | Start MCP server (stdin/stdout) |
| `anyclaw skills [pkg]` | Generate SKILL.md for Claude Code |
| `anyclaw auth <pkg> <api-key>` | Set API key for a package |
| `anyclaw daemon start\|stop\|status` | Manage browser bridge daemon |
| `anyclaw version` | Print version |
| `anyclaw update` | Self-update to latest version |

## Agent Integration

anyclaw ships with a `SKILL.md` at the project root that teaches AI agents how to use anyclaw. Link it to your agent's skills directory:

```bash
# Claude Code
ln -s /path/to/anyclaw/SKILL.md ~/.claude/skills/anyclaw/SKILL.md

# Or generate skills for individual packages
anyclaw skills hackernews
# → ~/.anyclaw/skills/hackernews/SKILL.md
```

## Registry

The [package registry](registry/) indexes 30+ packages from multiple sources:

- **anyclaw native** — packages maintained in this repo
- **community** — compatible YAML pipeline packages from the ecosystem
- **third-party plugins** — community-contributed packages

To add a package to the registry, submit a PR adding an entry to `registry/index.yaml`.

Packages not in the registry can still be installed directly by URL:

```bash
anyclaw install https://github.com/user/repo
```

## Credits

anyclaw's pipeline format is inspired by and compatible with [opencli](https://github.com/jackwener/opencli) by [@jackwener](https://github.com/jackwener). The anyclaw registry references several community packages from the opencli project. Thanks to opencli and its contributors for building a great collection of data tools.

## License

[MIT](LICENSE)
