---
notion-id: 3141c3ac-906d-810c-aef9-ee3a4015bcd1
---

# notion-cli

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.22-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

A command-line interface for Notion using the remote MCP (Model Context Protocol).

Inspired by [linear-cli](https://github.com/schpet/linear-cli) - stay in the terminal while managing your Notion workspace.

**Works great with AI agents** — includes a [skill](#skills) that lets agents search, create, and manage your Notion workspace alongside your code.

## Installation

### From Source

```bash
go install github.com/lox/notion-cli@latest
```

### Build Locally

```bash
git clone https://github.com/lox/notion-cli
cd notion-cli
mise run build
```

## Quick Start

```bash
# Authenticate with Notion (opens browser for OAuth)
notion-cli auth login

# Search your workspace
notion-cli search "meeting notes"

# View a page
notion-cli page view "https://notion.so/My-Page-abc123"

# List your pages
notion-cli page list

# Create a page
notion-cli page create --title "New Page" --content "# Hello World"
```

## Commands

### Authentication

```bash
notion-cli auth login                          # Authenticate with Notion via OAuth (default account)
notion-cli auth login --account work           # Authenticate a named account
notion-cli auth refresh --account work         # Refresh one account token
notion-cli auth status                         # Show current account status
notion-cli auth status --all                   # Show all accounts
notion-cli auth list                           # List saved accounts
notion-cli auth use work                       # Set active account
notion-cli auth logout --account work          # Clear one account token
notion-cli auth logout --all                   # Clear all account tokens
```

### Pages

```bash
notion-cli page list                           # List pages
notion-cli page list --limit 50                # Limit results
notion-cli page list --json                    # Output as JSON

notion-cli page view <url>                     # View page content
notion-cli page view <url> --raw               # View raw Notion markup
notion-cli page view <url> --json              # Output as JSON

notion-cli page create --title "Title"         # Create a page
notion-cli page create --title "T" --content "Body text"
notion-cli page create --title "T" --parent <page-id>
notion-cli page create --title "T" --icon "✅" # Set page icon

# Upload a markdown file as a new page
notion-cli page upload ./document.md                        # Title from # heading or filename
notion-cli page upload ./document.md --title "Custom Title" # Explicit title
notion-cli page upload ./document.md --parent "Engineering" # Parent by name or ID
notion-cli page upload ./document.md --icon "📄"             # Set emoji icon
notion-cli page upload ./document.md --icon "https://cdn.example.com/icon.png" # Set external icon URL
notion-cli page upload ./document.md --icon "none"           # Clear icon
notion-cli page upload ./document.md --asset-base-url "https://cdn.example.com/docs" # Rewrite local image embeds
notion-cli page upload ./document.md --props "Status=Todo;Priority=High"

# Sync a markdown file (create or update)
notion-cli page sync ./document.md                          # Creates page, writes notion-id to frontmatter
notion-cli page sync ./document.md                          # Updates page using notion-id from frontmatter
notion-cli page sync ./document.md --parent "Engineering"   # Set parent on first sync
notion-cli page sync ./document.md --asset-base-url "https://cdn.example.com/docs"
notion-cli page sync ./document.md --props "Status=Todo;Priority=High"
notion-cli page sync ./document.md --prop "Priority=Urgent" # --prop overrides values set via --props/frontmatter
notion-cli page sync ./document.md --property-mode off      # Disable property sync
notion-cli page sync ./document.md --icon "🔥"              # Set emoji icon
notion-cli page sync ./document.md --icon "none"            # Clear icon

# Edit an existing page
notion-cli page edit <url> --replace "New content"                      # Replace all content
notion-cli page edit <url> --find "old text" --replace-with "new text"  # Find and replace
notion-cli page edit <url> --find "section" --append "extra content"    # Append after match
notion-cli page edit <url> --icon "✅"                                  # Update icon only
notion-cli page edit <url> --replace "Body" --icon "none"               # Content + icon in one command
```

### Search

```bash
notion-cli search "query"                      # Search workspace
notion-cli search "query" --limit 10           # Limit results
notion-cli search "query" --json               # Output as JSON
```

### Databases

```bash
notion-cli db list                             # List databases
notion-cli db list --json                      # Output as JSON

notion-cli db query <database-id>              # Query database
notion-cli db query <id> --json                # Output as JSON
```

### Comments

```bash
notion-cli comment list <page-id>              # List comments on a page
notion-cli comment list <page-id> --json       # Output as JSON

notion-cli comment create <page-id> --content "Comment text"
```

### Other

```bash
notion-cli version                             # Show version
notion-cli --help                              # Show help
```

## Configuration

Configuration is stored at:

- `~/.config/notion-cli/config.json` (active account selection)
- `~/.config/notion-cli/accounts/<account>.json` (OAuth tokens per account)

Legacy `~/.config/notion-cli/token.json` is migrated automatically to `accounts/default.json` for backward compatibility.

The CLI uses Notion's remote MCP server with OAuth authentication. On first run, `notion-cli auth login` will open your browser to authorize the CLI with your Notion workspace.

**Note:** Access tokens expire after 1 hour. The CLI automatically refreshes tokens when they expire or are about to expire, so you typically don't need to think about this. Use `notion-cli auth refresh` to manually refresh if needed.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `NOTION_ACCESS_TOKEN` | Access token for CI/headless usage (skips OAuth) |
| `NOTION_ACCOUNT` | Account profile to use (`default` fallback when unset) |
| `NOTION_CLI_ASSET_BASE_URL` | Base URL for rewriting local markdown image embeds during `page upload`/`page sync` |
| `NOTION_CLI_ASSET_ROOT` | Optional local root mapped to `NOTION_CLI_ASSET_BASE_URL` when building image URLs |
| `NOTION_API_TOKEN` | Official Notion REST API token (required for `--icon`) |
| `NOTION_API_BASE_URL` | Override Notion REST base URL (default `https://api.notion.com/v1`) |
| `NOTION_API_NOTION_VERSION` | Override Notion-Version header (default `2022-06-28`) |

## Local Image Embeds

Notion MCP's markdown format accepts image URLs (`![Caption](URL)`) but does not ingest local file paths directly.

Use `--asset-base-url` (or `NOTION_CLI_ASSET_BASE_URL`) to rewrite local image paths before upload/sync:

```bash
notion-cli page sync ./notes.md \
  --asset-base-url "https://cdn.example.com/project"
```

You can also set an explicit local root for URL path mapping:

```bash
notion-cli page sync ./notes.md \
  --asset-base-url "https://cdn.example.com/project" \
  --asset-root "/Users/you/project"
```

This works well with static hosts such as miniserve.

## Property Sync

`page sync` and `page upload` can send property updates along with content.

- `--property-mode warn` (default): attempt property sync, continue on property errors.
- `--property-mode strict`: fail on property parse/update/create errors.
- `--property-mode off`: disable property sync completely.
- `--props "A=1;B=2"`: set multiple properties in one flag.
- `--prop "A=1"`: set a single property (repeatable, overrides `--props`).

For `page sync`, top-level frontmatter keys (excluding `notion-id`/`notion`) are also treated as property candidates.

## How It Works

This CLI connects to [Notion's remote MCP server](https://developers.notion.com/guides/mcp/mcp) at `https://mcp.notion.com/mcp` using the Model Context Protocol.
For icon updates (`--icon`), it also uses Notion's official REST API (`https://api.notion.com/v1`) with `NOTION_API_TOKEN` / `api.token`.

MCP provides:

- **OAuth authentication** for MCP operations
- **Notion-flavoured Markdown** - Create/edit content naturally
- **Semantic search** - Search across connected apps too
- **Optimised for CLI** - Efficient responses

## Skills

notion-cli includes a skill that helps AI agents use the CLI effectively.

### Amp / Claude Code

Install the skill using [skills.sh](https://skills.sh):

```bash
npx skills add lox/notion-cli
```

Or manually add to your Amp/Claude config:

```bash
# Amp
amp skill add https://github.com/lox/notion-cli/tree/main/skills/notion-cli

# Claude Code
claude plugin marketplace add lox/notion-cli
claude plugin install notion-cli@notion-cli
```

View the skill at: [skills/notion/SKILL.md](skills/notion/SKILL.md)

## Links

- [Notion MCP Documentation](https://developers.notion.com/guides/mcp/mcp)
- [Notion API Reference](https://developers.notion.com/reference/intro)
- [Model Context Protocol](https://modelcontextprotocol.io/)

## License

MIT License - see [LICENSE](LICENSE) for details.
