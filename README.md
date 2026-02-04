# notion-cli

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.22-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

A command-line interface for Notion using the remote MCP (Model Context Protocol).

Inspired by [linear-cli](https://github.com/schpet/linear-cli) - stay in the terminal while managing your Notion workspace.

## Installation

### From Source

```bash
go install github.com/lox/notion-cli@latest
```

### Build Locally

```bash
git clone https://github.com/lox/notion-cli
cd notion-cli
task build
```

## Quick Start

```bash
# Authenticate with Notion (opens browser for OAuth)
notion config auth

# Search your workspace
notion search "meeting notes"

# View a page
notion page view "https://notion.so/My-Page-abc123"

# List your pages
notion page list

# Create a page
notion page create --title "New Page" --content "# Hello World"
```

## Commands

### Configuration

```bash
notion config auth     # Run OAuth flow to authenticate
notion config show     # Show current configuration
notion config clear    # Clear stored credentials
```

### Pages

```bash
notion page list                           # List pages
notion page list --limit 50                # Limit results
notion page list --json                    # Output as JSON

notion page view <url>                     # View page content
notion page view <url> --json              # Output as JSON

notion page create --title "Title"         # Create a page
notion page create --title "T" --content "Body text"
notion page create --title "T" --parent <page-id>
```

### Search

```bash
notion search "query"                      # Search workspace
notion search "query" --limit 10           # Limit results
notion search "query" --json               # Output as JSON
```

### Databases

```bash
notion db list                             # List databases
notion db list --json                      # Output as JSON

notion db query <database-id>              # Query database
notion db query <id> --json                # Output as JSON
```

### Comments

```bash
notion comment list <page-id>              # List comments on a page
notion comment list <page-id> --json       # Output as JSON

notion comment create <page-id> --content "Comment text"
```

### Other

```bash
notion version                             # Show version
notion --help                              # Show help
```

## Configuration

Configuration is stored at `~/.config/notion-cli/config.json`.

The CLI uses Notion's remote MCP server with OAuth authentication. On first run, `notion config auth` will open your browser to authorize the CLI with your Notion workspace.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `NOTION_TOKEN` | OAuth token (overrides config file) |
| `NOTION_CLI_CONFIG` | Custom config file path |

## How It Works

This CLI connects to [Notion's remote MCP server](https://developers.notion.com/guides/mcp/mcp) at `https://mcp.notion.com/mcp` using the Model Context Protocol. This provides:

- **OAuth authentication** - No API tokens to manage
- **Notion-flavoured Markdown** - Create/edit content naturally
- **Semantic search** - Search across connected apps too
- **Optimised for CLI** - Efficient responses

## Links

- [Notion MCP Documentation](https://developers.notion.com/guides/mcp/mcp)
- [Notion API Reference](https://developers.notion.com/reference/intro)
- [Model Context Protocol](https://modelcontextprotocol.io/)

## License

MIT License - see [LICENSE](LICENSE) for details.
