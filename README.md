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

### Search packages

```bash
# Search by keyword
anyclaw search news
anyclaw search finance

# Search within a specific repo
anyclaw search news --repo myrepo
```

### Install packages

```bash
# Install from registry (by name)
anyclaw install hackernews
anyclaw install translator

# Install from GitHub URL
anyclaw install https://github.com/user/repo

# Install from a configured repo
anyclaw install myrepo/pkgname

# Install from local file or directory
anyclaw install examples/openapi.yaml
anyclaw install examples/web.yaml

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

# Shorthand (package name as subcommand)
anyclaw hackernews top --limit 5
anyclaw gh pr list
anyclaw docker ps

# Show available commands for a package
anyclaw run hackernews
anyclaw hackernews --help

# Output as JSON instead of table
anyclaw hackernews top --limit 5 --json
```

### Manage repos

```bash
# Add a custom repo
anyclaw repo add myrepo https://example.com/index.yaml
anyclaw repo add myskills https://github.com/user/skills/tree/main/packages --type github-skills

# List configured repos
anyclaw repo list

# Update repo caches (for fast search)
anyclaw repo update

# Remove a repo
anyclaw repo remove myrepo
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

Some packages require browser access. anyclaw includes its own browser extension and daemon for this.

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
| `anyclaw search <keyword>` | Search packages by name, description, or tag |
| `anyclaw install <name\|url\|file\|dir>` | Install a package |
| `anyclaw uninstall <name>` | Remove a package |
| `anyclaw list` | List installed packages and commands |
| `anyclaw run <pkg> <cmd> [flags]` | Run a command |
| `anyclaw mcp [pkg]` | Start MCP server (stdin/stdout) |
| `anyclaw skills [pkg]` | Generate SKILL.md for Claude Code |
| `anyclaw auth <pkg> <api-key>` | Set API key for a package |
| `anyclaw repo add\|remove\|list\|update` | Manage package repositories |
| `anyclaw show <pkg>` | Show package details |
| `anyclaw upgrade [pkg]` | Upgrade installed packages |
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

## Registry & Repos

The [package registry](registry/) hosts anyclaw-native packages. You can also add custom repos:

```bash
# Add a GitHub-based skills repo
anyclaw repo add myskills https://github.com/user/skills/tree/main/packages --type github-skills

# Add an anyclaw index repo
anyclaw repo add myrepo https://example.com/index.yaml

# Install from a repo
anyclaw install myrepo/pkgname
```

To add a package to the registry, submit a PR adding an entry to `registry/index.yaml`.

Packages not in the registry can still be installed directly by URL:

```bash
anyclaw install https://github.com/user/repo
```

## License

[MIT](LICENSE)
