# Dojo

[![Go Reference](https://pkg.go.dev/badge/github.com/jadmadi/dojo.svg)](https://pkg.go.dev/github.com/jadmadi/dojo)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/release-v1.3.0-green.svg)](https://github.com/jadmadi/dojo/releases)

`dojo` is a static Go command-line tool and Model Context Protocol (MCP) server for finding, streaming, auditing, and synchronizing **Agent Skills** (`SKILL.md`) across local workstations and cloud VMs (`agy-vm`). Coding agents can inspect a skill's Table of Contents (`dojo toc`), stream individual `#sections` (`dojo load <skill>#<section>`) to save 70–90% of context tokens, maintain a **Personal Skills Registry** (`dojo profile link` / `dojo profile sync` / `dojo profile pull`), and bi-directionally reconcile and project skills across **Antigravity (Desktop & CLI)**, Claude Code, Cursor, Windsurf, OpenCode, and Sila (`dojo sila` / `dojo sync`).

`dojo` implements the [Cloudflare Agent Skills Discovery RFC (`v0.2.0`)](https://github.com/cloudflare/agent-skills-discovery-rfc) (`/.well-known/agent-skills/index.json`), verifies SHA-256 content digests, resolves transitive `dependencies` in topological order, and scans skill packages for secrets and prompt injection before publishing.

## Installation and quickstart

Install `dojo` via the one-line installer (Linux / macOS / `agy-vm`), `go install` (Linux, macOS, Windows), or the `skills` CLI, then initialize your workspace or stream your first skill section:

```bash
# Install prebuilt binary (Linux / macOS / agy-vm) or build with Go
curl -fsSL https://raw.githubusercontent.com/jadmadi/dojo/main/scripts/install.sh | sh
# Or via Go toolchain (Linux / macOS / Windows):
go install github.com/jadmadi/dojo/cmd/dojo@latest
# Or install the Dojo skill with the skills CLI (any supported agent):
npx skills add jadmadi/dojo

# 1. Initialize workspace (.agents/skills/, skills.json, and active harness projections)
dojo init --workspace

# 2. Search federated catalogs with hybrid BM25 + fuzzy matching
dojo find "go testing"

# 3. Inspect section headings, #slugs, line ranges, and byte sizes
dojo toc dojo

# 4. Stream only the section you need into context
dojo load dojo#1-search-section-level-paging-dojo-find-dojo-toc-dojo-load

# 5. Install a skill, update skills.json, and project across all coding harnesses
dojo learn dojo --with-deps --target all
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
| `dojo profile <subcommand>` | Link a Personal Skills Registry and bi-directionally sync `skills.json` and skills across local machines and `agy-vm` | `link`, `sync`, `pull`, `push`, `apply`, `export`, `-g, --global`, `-f, --force` |
| `dojo init [--workspace \| <name>]` | Initialize a multi-harness workspace (`-w`) or scaffold a new skill package | `-w, --workspace`, `-d, --dir <path>`, `-c, --category <name>` |
| `dojo completion <shell>` | Generate shell tab-completion scripts (`bash`, `zsh`, `fish`, `powershell`) | Dynamic skill, `#section`, and `--target` completions |
| `dojo mcp` | Start the JSON-RPC 2.0 MCP server over `stdio` | `--print-schema` |
| `dojo doctor` | Check environment health, YAML frontmatter, token budgets, and security/secret rules | `--skills`, `--security`, `-g, --global`, `--json` |
| `dojo list` / `dojo show` | List catalog skills with numbered `#` indices or inspect metadata by name or number | `--json`, `--raw` |
| `dojo status` / `dojo update` | Check installed skills for upstream drift and apply updates | `--all`, `-g, --global`, `-f, --force` |
| `dojo catalog` | Manage federated skill catalogs (`list`, `add`, `remove`) | `-n, --name`, `-b, --ref`, `-p, --subpath` |
| `dojo upgrade` | Check GitHub Releases for a newer `dojo` binary or upgrade in place | `-c, --check`, `-f, --force` |

### Inspect and stream sections (`toc` and `load`)

Inspecting a skill's Table of Contents before loading lets agents read only the relevant heading block instead of paging through an entire `SKILL.md` file:

```bash
# View all headings, #slugs, line ranges, and byte sizes
dojo toc dojo

# Load a single section (including nested subsections) by #slug
dojo load dojo#3-personal-skills-registry-local-to-cloud-agy-vm-continuity

# Load a companion reference file bundled with a skill
dojo load dojo/references/migration-guide.md

# Load a skill together with all transitive dependencies in topological order
dojo load dojo --with-deps
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
dojo learn dojo --with-deps --target all

# Bi-directionally reconcile edits across harnesses and project to Antigravity, Cursor, and Claude
dojo sila --target antigravity,cursor,claude

# Project canonical .agents/skills outward only (skip reverse reconciliation)
dojo sync --one-way
```

### Personal Skills Registry & Local-to-Cloud (`agy-vm`) Continuity (`dojo profile`)

Link a GitHub repository (such as `github:jadmadi/agy-vm` or `github:<you>/my-skills`) once as your **Personal Skills Registry**. `dojo profile sync` then discovers all skills, `.mdc` rules, and slash commands across your local harnesses, generates `skills.json`, scans packaged files for accidental secrets or prompt injection, and bi-directionally synchronizes them with GitHub. On a remote VM (`agy-vm`) or second workstation, `dojo profile pull` (or `dojo profile sync`) pulls and projects your entire personal skill library in one command:

```bash
# 1. Link your GitHub repository as your Personal Skills Registry (once per machine)
dojo profile link github:jadmadi/agy-vm -g

# 2. Bi-directionally sync local harness skills <-> skills.json <-> GitHub registry
dojo profile sync

# 3. On a remote VM (agy-vm) or new machine: pull and project all skills from your linked registry
dojo profile pull -g

# 4. Push specific skills, list numbers, .mdc rules, or slash commands to your registry:
dojo profile push my-skill my-slash-command -g
dojo profile push github:jadmadi/agy-vm 1,4,6 -g
dojo profile push github:jadmadi/agy-vm ~/.claude/commands/deploy.md ~/.agents/skills/go-review

# 5. Bootstrap a project workspace: reconciles existing harness skills, creates skills.json, and projects harnesses
dojo init --workspace
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

`dojo mcp` runs a zero-dependency JSON-RPC 2.0 MCP server over `stdio`, exposing `dojo_find`, `dojo_toc`, `dojo_load`, `dojo_run`, `dojo_learn`, `dojo_doctor`, `dojo_sync`, and `dojo_profile` as native tools:

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

Create new skills from a tested template and validate frontmatter, single-page token budgets (`6,000` bytes / ~`1,500` tokens), and security/secret patterns before publishing:

```bash
# Create .agents/skills/my-skill/{SKILL.md,references/guide.md,scripts/check.sh,evals/evals.json}
dojo init my-skill --category coding --description "Workflow rules for Go services"

# Run frontmatter validation, token budget checks, and static secret / prompt-injection / curl|sh scans
dojo doctor --skills --security
```

## Configuration

`dojo` reads `~/.config/dojo/config.yaml` and `~/.cache/dojo` by default:

| Variable | Default value | Description |
| :--- | :--- | :--- |
| `DOJO_URL` | `https://raw.githubusercontent.com/jadmadi/dojo/main/.well-known/agent-skills/index.json` | Default remote catalog manifest URL |
| `DOJO_PROFILE_REPO` | *(empty)* | Default Personal Skills Registry target (`github:<owner>/<repo>` or directory) |
| `DOJO_CONFIG_DIR` | `~/.config/dojo` | Directory for `config.yaml` and `state.json` |
| `DOJO_CACHE_DIR` | `~/.cache/dojo` | Directory for ETag-cached manifests and skills |
| `DOJO_TELEMETRY` | `0` (opt-in) | Set to `0` or `DO_NOT_TRACK=1` to disable telemetry |

## Development and testing

Clone the repository and run the test suite and static skill audit locally:

```bash
git clone https://github.com/jadmadi/dojo.git
cd dojo
go test -race ./...
go run ./cmd/dojo doctor --skills --security
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

In Japanese martial arts, a **Dojo** (道場, literally *"place of the Way"*) is the **training ground** where practitioners hone, refine, and share their craft.

`dojo` provides the complete training ground for AI coding agents: local tooling to author and lint skills (`dojo init`, `dojo doctor`), enforce token budgets and secret hygiene, maintain portable `skills.json` manifests (`dojo profile`), and bi-directionally synchronize installed skills across every agent harness (`dojo sync` / `dojo sila` for Antigravity Desktop & CLI, Claude, Cursor, Windsurf, OpenCode, and Sila).

## License and contributing

Licensed under the [Apache License 2.0](LICENSE). Contributions are welcome—see [CONTRIBUTING.md](CONTRIBUTING.md) and [BACKLOG.md](BACKLOG.md).
