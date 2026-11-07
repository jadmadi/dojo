# Dojo

[![Go Reference](https://pkg.go.dev/badge/github.com/jadmadi/dojo.svg)](https://pkg.go.dev/github.com/jadmadi/dojo)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/release-v1.0.0-green.svg)](https://github.com/jadmadi/dojo/releases)

`dojo` is a command-line tool and Model Context Protocol (MCP) server for finding, streaming, auditing, and syncing **Agent Skills** (`SKILL.md`). Coding agents can inspect a skill's Table of Contents (`dojo toc`), stream individual `#sections` (`dojo load <skill>#<section>`) to save 70–90% of context tokens, or project installed skills across Claude Code, Cursor, Windsurf, Gemini CLI, OpenCode, and Sila (`dojo sila`).

`dojo` implements the [Cloudflare Agent Skills Discovery RFC (`v0.2.0`)](https://github.com/cloudflare/agent-skills-discovery-rfc) (`/.well-known/agent-skills/index.json`), verifies SHA-256 content digests, resolves transitive `dependencies` in topological order, and maintains full backward compatibility with [KungFu (`danicat/kungfu`)](https://github.com/danicat/kungfu).

## Installation and quickstart

Install `dojo` with Go 1.25 or newer and run your first search, section load, and multi-harness sync immediately:

```bash
# Install the binary
go install github.com/jadmadi/dojo/cmd/dojo@latest

# 1. Search federated catalogs with hybrid BM25 + fuzzy matching
dojo find "go testing"

# 2. Inspect section headings, #slugs, line ranges, and byte sizes
dojo toc godoctor

# 3. Stream only the section you need into context
dojo load godoctor#quick-start

# 4. Install a skill and project it across all coding harnesses
dojo learn godoctor --with-deps --target all
```

## Core workflows and commands

Every `dojo` command supports human-readable terminal output by default, structured JSON (`--json`), and tab-separated output (`--raw`).

| Command | Purpose | Primary flags |
| :--- | :--- | :--- |
| `dojo find <keywords...>` | Search across skill names, descriptions, categories, and tags | `-n, --limit <int>`, `--json`, `--raw` |
| `dojo toc <skill>` | List Markdown headings, `#slugs`, line ranges, and byte sizes | `--local`, `-u, --update`, `--json`, `--raw` |
| `dojo load <skill>[#section]` | Stream skill instructions, a single `#section`, or bundled files | `-s, --section <slug>`, `--with-deps`, `-p <page>`, `--full` |
| `dojo run <skill> <script>` | Run a bundled script (`uv`, `python3`, `bun`, `deno`, `node`, `bash`) | Passes trailing arguments directly to script |
| `dojo learn <skill>` | Install a skill locally (`.agents/skills/`) or globally (`-g`) | `-g, --global`, `--with-deps`, `-t, --target <harnesses>` |
| `dojo sync` / `dojo sila` | Project installed skills across Claude, Cursor, Windsurf, Gemini, OpenCode, and Sila | `-t, --target <list>`, `-g, --global` |
| `dojo mcp` | Start the JSON-RPC 2.0 MCP server over `stdio` | `--print-schema` |
| `dojo init <name>` | Scaffold a compliant skill package (`SKILL.md`, `references/`, `scripts/`, `evals/`) | `-d, --dir <path>`, `-c, --category <name>`, `-f, --force` |
| `dojo doctor` | Check environment health, YAML frontmatter, token budgets, and security rules | `--skills`, `--security`, `-g, --global`, `--json` |
| `dojo list` / `dojo show` | List catalog skills or inspect a single skill's metadata and files | `--json`, `--raw` |
| `dojo status` / `dojo update` | Check installed skills for upstream drift and apply updates | `--check`, `-g, --global` |
| `dojo catalog` | Manage federated skill catalogs (`list`, `add`, `remove`, `update`) | `-n, --name`, `-b, --ref`, `-p, --subpath` |
| `dojo upgrade` | Check GitHub Releases for a newer `dojo` binary or upgrade in place | `-c, --check`, `-f, --force` |

### Inspect and stream sections (`toc` and `load`)

Inspecting a skill's Table of Contents before loading lets agents read only the relevant heading block instead of paging through an entire `SKILL.md` file:

```bash
# View all headings, #slugs, line ranges, and byte sizes
dojo toc draft

# Load a single section (including nested subsections) by #slug
dojo load draft#b-built-in-inverted-pyramid-geo-ready-section-leads

# Load a companion reference file bundled with a skill
dojo load godoctor/references/architecture.md

# Load a skill together with all transitive dependencies in topological order
dojo load fullstack-go --with-deps
```

### Sync skills across agent harnesses (`learn`, `sync`, and `sila`)

`dojo` stores canonical skills in `.agents/skills/<name>` and projects them into every active agent harness in your workspace or home directory:

| Target (`--target`) | Workspace projection path | Projection method |
| :--- | :--- | :--- |
| `agents` | `.agents/skills/<name>/SKILL.md` | Canonical directory |
| `claude` | `.claude/skills/<name>/SKILL.md` | Relative symlink (copy fallback) |
| `cursor` | `.cursor/rules/<name>.mdc` | Transpiled Cursor `.mdc` frontmatter |
| `windsurf` | `.windsurf/rules/<name>.md` | Markdown rule copy |
| `gemini` | `.gemini/skills/<name>/SKILL.md` | Relative symlink (copy fallback) |
| `opencode` | `.opencode/skills/<name>/SKILL.md` | Relative symlink (copy fallback) |
| `sila` | `.sila/skills/<name>/SKILL.md` | Relative symlink (copy fallback) |

```bash
# Install a skill and project it to all harnesses at once
dojo learn godoctor --with-deps --target all

# Project all existing skills in .agents/skills/ to specific harnesses
dojo sila --target claude,cursor,gemini,sila
```

### Serve skills over Model Context Protocol (`mcp`)

`dojo mcp` runs a zero-dependency JSON-RPC 2.0 MCP server over `stdio`, exposing `dojo_find`, `dojo_toc`, `dojo_load`, `dojo_run`, `dojo_learn`, and `dojo_doctor` as native tools:

```json
{
  "mcpServers": {
    "dojo": {
      "command": "dojo",
      "args": ["mcp"]
    }
  }
}
```

Inspect the complete JSON Schema tool definitions from the terminal:

```bash
dojo mcp --print-schema
```

### Scaffold and audit skills (`init` and `doctor`)

Create new skills from a tested template and validate frontmatter, single-page token budgets (`6,000` bytes / ~`1,500` tokens), and security patterns before publishing:

```bash
# Create .agents/skills/my-skill/{SKILL.md,references/guide.md,scripts/check.sh,evals/evals.json}
dojo init my-skill --category coding --description "Workflow rules for Go services"

# Run frontmatter validation, token budget checks, and static prompt-injection / curl|sh scans
dojo doctor --skills --security
```

## Configuration and KungFu compatibility

`dojo` reads `~/.config/dojo/config.yaml` and `~/.cache/dojo` by default, and falls back automatically to `~/.config/kungfu` and `~/.cache/kungfu` when migrating from an existing `kungfu` setup:

| Variable | Fallback variable | Default value | Description |
| :--- | :--- | :--- | :--- |
| `DOJO_URL` | `KUNGFU_URL` | `https://skills.danicat.dev/.well-known/agent-skills/index.json` | Default remote catalog manifest URL |
| `DOJO_CONFIG_DIR` | `KUNGFU_CONFIG_DIR` | `~/.config/dojo` | Directory for `config.yaml` and `state.json` |
| `DOJO_CACHE_DIR` | `KUNGFU_CACHE_DIR` | `~/.cache/dojo` | Directory for ETag-cached manifests and skills |
| `DOJO_TELEMETRY` | `KUNGFU_TELEMETRY` | `0` (opt-in) | Set to `0` or `DO_NOT_TRACK=1` to disable telemetry |

## Development and testing

Clone the repository and run the test suite and static skill audit locally:

```bash
git clone https://github.com/jadmadi/dojo.git
cd dojo
go test -race ./...
go run . doctor --skills --security
```

## Architecture

`dojo` is organized into focused internal packages with zero external runtime daemons:

| Package | Responsibility |
| :--- | :--- |
| `cmd/dojo` | CLI binary entrypoint (`go install github.com/jadmadi/dojo/cmd/dojo@latest`) |
| `internal/catalog` | RFC `v0.2.0` manifest discovery, SHA-256 verification, Markdown section parser (`sections.go`), and DAG dependency resolver (`deps.go`) |
| `internal/commands` | Subcommand flag parsing, exit-code mapping, and human/JSON/TSV output formatting |
| `internal/harness` | Multi-harness detection, relative symlink projection, and Cursor `.mdc` transpilation |
| `internal/mcp` | JSON-RPC 2.0 Model Context Protocol `stdio` server and tool router |
| `internal/runner` | Polyglot companion script runner (`uv`, `python3`, `bun`, `deno`, `node`, `sh`) |
| `internal/search` | Hybrid BM25, TF-IDF, prefix, and Levenshtein fuzzy search engine |
| `internal/state` | POSIX `flock`-synchronized state tracking (`state.json`) for installed skills and catalogs |

## License and contributing

Licensed under the [Apache License 2.0](LICENSE). Contributions are welcome—see [CONTRIBUTING.md](CONTRIBUTING.md) and [BACKLOG.md](BACKLOG.md).
