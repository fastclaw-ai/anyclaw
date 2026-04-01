---
name: anyclaw
description: The universal tool adapter for AI agents. Search, install, and run packages from the anyclaw registry. Use anyclaw to access web APIs, data pipelines, CLI tools, and scripts as unified commands.
---

# anyclaw

anyclaw turns any API, website, or CLI tool into agent-ready commands. Use it to search for packages, install them, and run commands directly from the terminal.

## When to use

- When the user asks to fetch data from websites (Hacker News, Stack Overflow, Reddit, Bilibili, etc.)
- When the user asks to call an API (translation, IP lookup, etc.)
- When the user needs a CLI tool wrapped for easier use
- When the user wants to discover available tools or data sources

## Core workflow

### 1. Search for packages

```bash
# Search by keyword
anyclaw search news
anyclaw search chinese
anyclaw search finance

# Browse all available packages
anyclaw list --all
```

### 2. Install a package

```bash
# Install from registry by name
anyclaw install hackernews
anyclaw install translator

# Install from GitHub URL
anyclaw install https://github.com/user/repo

# Install a local YAML file
anyclaw install path/to/spec.yaml

# Wrap a system CLI tool
anyclaw install gh
anyclaw install docker
```

### 3. List installed packages

```bash
anyclaw list
```

### 4. Run commands

```bash
# Space-separated format (primary)
anyclaw run <package> <command> [--arg value ...]

# Examples
anyclaw run hackernews top --limit 5
anyclaw run hackernews search --query "AI" --limit 10
anyclaw run translator translate --q "hello world" --langpair "en|zh"

# Shorthand format
anyclaw hackernews top --limit 5
anyclaw gh pr list
anyclaw docker ps

# Show available commands for a package
anyclaw run hackernews
anyclaw hackernews --help
```

### 5. Manage packages

```bash
# Uninstall
anyclaw uninstall hackernews

# Set API key for packages that require auth
anyclaw auth <package> <api-key>
```

## Available registry packages

Common packages you can install:

| Package | Description | Install |
|---------|-------------|---------|
| hackernews | Hacker News - top, search, best, jobs, new, ask, show, user | `anyclaw install hackernews` |
| translator | Translation service | `anyclaw install translator` |
| lobsters | Lobsters - hot, active, newest, tag | `anyclaw install lobsters` |
| stackoverflow | Stack Overflow - hot, search, bounties, unanswered | `anyclaw install stackoverflow` |
| v2ex | V2EX developer community | `anyclaw install v2ex` |
| juejin | 掘金 developer community | `anyclaw install juejin` |
| bilibili | Bilibili video platform | `anyclaw install bilibili` |
| zhihu | 知乎 Q&A community | `anyclaw install zhihu` |
| douban | 豆瓣 movies, books, music | `anyclaw install douban` |
| reddit | Reddit discussions | `anyclaw install reddit` |
| twitter | Twitter/X social media | `anyclaw install twitter` |
| youtube | YouTube video platform | `anyclaw install youtube` |
| arxiv | arXiv scientific papers | `anyclaw install arxiv` |
| wikipedia | Wikipedia encyclopedia | `anyclaw install wikipedia` |

Run `anyclaw search <keyword>` or `anyclaw list --all` to discover more.

## Output format

Commands return JSON output. Parse the JSON and present results in a human-readable format (table, list, or summary) based on the user's request.

## Tips

- Always check if a package is installed (`anyclaw list`) before running commands
- If a package is not installed, install it first with `anyclaw install <name>`
- Use `--help` on any command for detailed usage: `anyclaw run hackernews top --help`
- If a command fails with auth errors, set the API key: `anyclaw auth <package> <key>`
