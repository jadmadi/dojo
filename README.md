# Dojo

[![Go Reference](https://pkg.go.dev/badge/github.com/jadmadi/dojo.svg)](https://pkg.go.dev/github.com/jadmadi/dojo)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/release-v1.1.0-green.svg)](https://github.com/jadmadi/dojo/releases)

`dojo` is a static Go command-line tool and Model Context Protocol (MCP) server for finding, streaming, auditing, and synchronizing **Agent Skills** (`SKILL.md`) across local workstations and cloud VMs (`agy-vm`). Coding agents can inspect a skill's Table of Contents (`dojo toc`), stream individual `#sections` (`dojo load <skill>#<section>`) to save 70–90% of context tokens, maintain a portable `skills.json` manifest (`dojo profile`), and bi-directionally reconcile and project skills across **Antigravity (Desktop & CLI)**, Claude Code, Cursor, Windsurf, OpenCode, and Sila (`dojo sila` / `dojo sync`).

`dojo` implements the [Cloudflare Agent Skills Discovery RFC (`v0.2.0`)](https://github.com/cloudflare/agent-skills-discovery-rfc) (`/.well-known/agent-skills/index.json`), verifies SHA-256 content digests, resolves transitive `dependencies` in topological order, and maintains backward compatibility with [KungFu (`danicat/kungfu`)](https://github.com/danicat/kungfu).

## Installation and quickstart

Install `dojo` via the one-line installer (Linux / macOS / `agy-vm`) or `go install` (Linux, macOS, Windows), then initialize your workspace or stream your first skill section:

```bash
# Install prebuilt binary (Linux / macOS / agy-vm) or build with Go
curl -fsSL https://raw.githubusercontent.com/jadmadi/dojo/main/scripts/install.sh | sh
# Or via Go toolchain (Linux / macOS / Windows):
go install github.com/jadmadi/dojo/cmd/dojo@latest

# 1. Initialize workspace (.agents/skills/, skills.json, and active harness projections)
dojo init --workspace

# 2. Search federated catalogs with hybrid BM25 + fuzzy matching
dojo find "go testing"

# 3. Inspect section headings, #slugs, line ranges, and byte sizes
dojo toc godoctor

# 4. Stream only the section you need into context
dojo load godoctor#quick-start

# 5. Install a skill, update skills.json, and project across all coding harnesses
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
| `dojo sync` / `dojo sila` | Bi-directionally reconcile and project skills across Antigravity, Claude, Cursor, Windsurf, OpenCode, and Sila | `-t, --target <list>`, `-g, --global`, `--one-way` |
| `dojo profile <subcommand>` | Export, apply, or sync a portable `skills.json` manifest (local, HTTP, or `github:owner/repo`) | `export`, `apply`, `sync`, `-g, --global`, `--push` |
| `dojo init [--workspace \| <name>]` | Initialize a multi-harness workspace (`-w`) or scaffold a new skill package | `-w, --workspace`, `-d, --dir <path>`, `-c, --category <name>` |
| `dojo completion <shell>` | Generate shell tab-completion scripts (`bash`, `zsh`, `fish`, `powershell`) | Dynamic skill, `#section`, and `--target` completions |
| `dojo mcp` | Start the JSON-RPC 2.0 MCP server over `stdio` | `--print-schema` |
| `dojo doctor` | Check environment health, YAML frontmatter, token budgets, and security rules | `--skills`, `--security`, `-g, --global`, `--json` |
| `dojo list` / `dojo show` | List catalog skills or inspect a single skill's metadata and files | `--json`, `--raw` |
| `dojo status` / `dojo update` | Check installed skills for upstream drift and apply updates | `--all`, `-g, --global`, `-f, --force` |
| `dojo catalog` | Manage federated skill catalogs (`list`, `add`, `remove`) | `-n, --name`, `-b, --ref`, `-p, --subpath` |
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

### Bi-directional cross-harness continuity (`learn`, `sync`, and `sila`)

`dojo` treats `.agents/skills/<name>/SKILL.md` as the canonical store while supporting **bi-directional reconciliation**: if you or an agent create or edit a skill directly inside `.gemini/skills` (Antigravity), `.claude/skills`, `.cursor/rules/<name>.mdc`, `.windsurf/rules/<name>.md`, `.opencode/skills`, or `.sila/skills`, running `dojo sync` (or `dojo sila`) imports new skills and newer modifications back into `.agents/skills/<name>` (including reverse-transpiling Cursor `.mdc` frontmatter into standard `SKILL.md`) before projecting updates out to every active harness:

| Target (`--target`) | Workspace projection path | Agent Harness & Projection method |
| :--- | :--- | :--- |
| `agents` | `.agents/skills/<name>/SKILL.md` | Universal Agent Skills canonical directory |
| `antigravity` / `gemini` | `.gemini/skills/<name>` or `.antigravity/skills/<name>` | **Antigravity Desktop & CLI** (Relative symlink / copy + reverse sync) |
| `claude` | `.claude/skills/<name>/SKILL.md` | Claude Code (Relative symlink + reverse sync) |
| `cursor` | `.cursor/rules/<name>.mdc` | Cursor (Bi-directional `.mdc` <-> `SKILL.md` transpilation) |
| `windsurf` | `.windsurf/rules/<name>.md` | Windsurf (Bi-directional Markdown rule sync) |
| `opencode` | `.opencode/skills/<name>/SKILL.md` | OpenCode (Relative symlink + reverse sync) |
| `sila` | `.sila/skills/<name>/SKILL.md` | Sila continuity engine (Relative symlink + reverse sync) |

```bash
# Install a skill and project it to all harnesses at once
dojo learn godoctor --with-deps --target all

# Bi-directionally reconcile edits across harnesses and project to Antigravity, Cursor, and Claude
dojo sila --target antigravity,cursor,claude

# Project canonical .agents/skills outward only (skip reverse reconciliation)
dojo sync --one-way
```

### Traveling `skills.json` and local-to-cloud continuity (`profile` and `init --workspace`)

Commit a declarative `skills.json` manifest to your project repository or personal dotfiles/VM repo (such as `jadmadi/agy-vm`) so your exact catalogs, skills, and harness projections travel between your local laptop and cloud VMs:

```bash
# Bootstrap a workspace: reconciles existing harness skills, creates skills.json, and projects harnesses
dojo init --workspace

# Export your current workspace or global (-g) skills into skills.json
dojo profile export

# Apply a local skills.json, HTTP URL, or GitHub repository manifest on a fresh machine or cloud VM
dojo profile apply
dojo profile apply github:jadmadi/agy-vm

# Reconcile harness edits, apply skills.json, update skills.json, and optionally git commit & push
dojo profile sync --push
```

Example `skills.json`:

```json
{
  "$schema": "https://raw.githubusercontent.com/jadmadi/dojo/main/.well-known/agent-skills/skills-manifest.schema.json",
  "version": "1.0.0",
  "harnesses": ["antigravity", "claude", "cursor", "sila"],
  "skills": [
    { "name": "dojo", "catalog": "dojo" },
    { "name": "sila" }
  ]
}
```

### Shell tab-completion (`completion`)

Enable dynamic tab-completion for subcommands, flags, installed and catalog skill names, live `#<section>` slugs (e.g. `dojo load dojo#<TAB>`), and `--target` harness names:

```bash
# Bash
source <(dojo completion bash)

# Zsh
source <(dojo completion zsh)

# Fish
dojo completion fish | source

# PowerShell
dojo completion powershell | Out-String | Invoke-Expression
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

`dojo` is organized into focused internal packages with zero external runtime daemons and `CGO_ENABLED=0` static compilation across Linux, macOS, and Windows:

| Package | Responsibility |
| :--- | :--- |
| `cmd/dojo` | CLI binary entrypoint (`go install github.com/jadmadi/dojo/cmd/dojo@latest`) |
| `internal/catalog` | RFC `v0.2.0` manifest discovery, SHA-256 verification, Markdown section parser (`sections.go`), and DAG dependency resolver (`deps.go`) |
| `internal/commands` | Subcommand flag parsing, dynamic shell completion (`completion.go`), exit-code mapping, and human/JSON/TSV output formatting |
| `internal/harness` | Bi-directional harness reconciliation, relative symlink projection, and Cursor `.mdc` <-> `SKILL.md` transpilation |
| `internal/profile` | Portable `skills.json` manifest builder and loader across local files, HTTP URLs, and GitHub repositories |
| `internal/mcp` | JSON-RPC 2.0 Model Context Protocol `stdio` server and tool router |
| `internal/runner` | Polyglot companion script runner (`uv`, `python3`, `bun`, `deno`, `node`, `sh`) |
| `internal/search` | Hybrid BM25, TF-IDF, prefix, and Levenshtein fuzzy search engine |
| `internal/state` | Cross-OS file-locked (`flock` / `LockFileEx`) state tracking (`state.json`) for installed skills and catalogs |

## Why the name Dojo?

The project originated as a fork of `kungfu` (功夫), paying homage to the *Matrix* line: *"I know Kung Fu."* While Kung Fu represents individual skill acquired through practice, a **Dojo** (道場, literally *"place of the Way"*) is the **training ground itself**.

Transitioning from Chinese martial arts terminology (*Kung Fu*) to Japanese (*Dojo*) reflects this architectural shift: rather than only loading a single skill on demand, `dojo` provides the local tooling to author and lint skills (`dojo init`, `dojo doctor`), verify token budgets, maintain portable `skills.json` manifests (`dojo profile`), and bi-directionally synchronize installed skills across multiple agent directories (`dojo sync` / `dojo sila` for Antigravity Desktop & CLI, Claude, Cursor, Windsurf, OpenCode, and Sila).

## License and contributing

Licensed under the [Apache License 2.0](LICENSE). Contributions are welcome—see [CONTRIBUTING.md](CONTRIBUTING.md) and [BACKLOG.md](BACKLOG.md).
