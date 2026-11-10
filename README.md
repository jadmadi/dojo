# Dojo

[![Go Reference](https://pkg.go.dev/badge/github.com/jadmadi/dojo.svg)](https://pkg.go.dev/github.com/jadmadi/dojo)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/release-v1.5.1-green.svg)](https://github.com/jadmadi/dojo/releases)

Dojo gives agents less to read and gives you one place to keep skills.

It is a static Go CLI and Model Context Protocol server that streams a single
section of an Agent Skill instead of the whole file, keeps one canonical
`.agents/skills` store, projects it into every coding harness you use, and
carries the same library between machines through a linked Git registry.

Three claims follow from that. Load one section, not the whole skill. Keep one
store, not six copies. Move the library, not your dotfiles.

## The short version

| Need | Dojo gives you |
|---|---|
| Less context per turn | `dojo toc` lists headings and byte sizes, `dojo load skill#section` returns one section |
| One source of truth | `.agents/skills/<name>/SKILL.md` is canonical; `dojo sync` reconciles edits from every harness and projects them back out |
| Same skills everywhere | `dojo profile link` and `dojo profile sync` keep a Git-backed `skills.json` registry across a laptop and a cloud VM |
| Agent-native tools | `dojo mcp` exposes 8 tools over stdio JSON-RPC 2.0 |
| Trust before install | SHA-256 digest verification, secret and prompt-injection scanning, and `dojo doctor` |

No daemon, no database, no runtime dependency beyond the binary. Skills are
plain `SKILL.md` files with YAML frontmatter, the format defined by the Agent
Skills specification.

## Install

```bash
# Prebuilt binary (Linux, macOS, agy-vm)
curl -fsSL https://raw.githubusercontent.com/jadmadi/dojo/main/scripts/install.sh | sh

# Go toolchain (Linux, macOS, Windows)
go install github.com/jadmadi/dojo/cmd/dojo@latest

# Or install the Dojo skill through the skills CLI (any supported agent)
npx skills add jadmadi/dojo
```

Then initialize a workspace and stream your first section:

```bash
dojo init --workspace
dojo find "go testing"
dojo toc dojo
dojo load dojo#1-search-section-level-paging-dojo-find-dojo-toc-dojo-load
dojo learn dojo --with-deps --target all
```

Reporting commands support terminal output by default, `--json` for structured
output, and `--raw` for tab-separated output. Streaming and passthrough
commands (`run`, `completion`, `mcp`) write their native output instead.

## Command reference

| Command | Purpose | Primary flags |
| :--- | :--- | :--- |
| `dojo find <keywords...>` | Search names, descriptions, categories, and tags with hybrid BM25 and fuzzy matching | `-n, --limit <int>`, `--json`, `--raw` |
| `dojo toc <skill>` | List Markdown headings, `#slugs`, line ranges, and byte sizes | `--local`, `-u, --update`, `-f, --force`, `--json`, `--raw` |
| `dojo load <skill>[#section]` | Stream a skill, a single `#section`, or a bundled file | `-s, --section <slug>`, `--with-deps`, `-p <page>`, `--full` |
| `dojo run <skill> <script>` | Run a bundled script (`uv`, `python3`, `bun`, `deno`, `node`, `bash`) | Trailing arguments pass through |
| `dojo learn <skill>` | Install a skill to `.agents/skills/` or the global store | `-g, --global`, `--with-deps`, `-t, --target <harnesses>` |
| `dojo sync` / `dojo sila` | Reconcile and project skills across harnesses | `-t, --target <list>`, `-g, --global`, `--one-way` |
| `dojo profile <subcommand>` | Link and sync a Personal Skills Registry | `link`, `sync`, `pull`, `push`, `apply`, `export`, `-g`, `-f` |
| `dojo init [--workspace \| <name>]` | Initialize a workspace or scaffold a skill package | `-w, --workspace`, `-d, --dir <path>`, `-c, --category <name>` |
| `dojo completion <shell>` | Generate tab completion for `bash`, `zsh`, `fish`, `powershell` | Dynamic skills, `#section` slugs, and `--target` names |
| `dojo mcp` | Start the JSON-RPC 2.0 MCP server over stdio | `--print-schema` |
| `dojo doctor` | Check frontmatter, byte budgets, and security rules | `--skills`, `--security`, `-g, --global`, `--json` |
| `dojo list` / `dojo show` | List catalog skills by number or inspect metadata | `--json`, `--raw` |
| `dojo status` | Show installed, forgotten, and used skills | `-g, --global`, `--json`, `--raw` |
| `dojo update` | Check installed skills for upstream drift and apply updates | `--all`, `-g, --global`, `-y, --yes`, `-f, --force` |
| `dojo catalog` | Manage federated skill catalogs | `list`, `add`, `remove`, `-n`, `-b`, `-p` |
| `dojo upgrade` | Check GitHub Releases and upgrade in place | `-c, --check`, `-f, --force` |

## Context control

An agent does not need a whole skill to follow one procedure. `dojo toc` shows
what is inside a skill and how many bytes each part costs. `dojo load` then
returns only the part that was asked for. The byte count is measured from the
file itself, so you can see the trade before you load anything.

The claim is enforced, not asserted. In the bundled benchmark
(`docs/benchmarks/context-savings.json`), `TestContextSavings` enforces a 45
percent ceiling on the largest leaf section as a share of the skill body, and
fails if the published numbers drift from the corpus.

```bash
# Inspect headings, #slugs, line ranges, and byte sizes
dojo toc dojo

# Load one section, including its nested subsections
dojo load dojo#3-personal-skills-registry-local-to-cloud-agy-vm-continuity

# Load a companion reference file bundled with a skill
dojo load dojo/references/migration-guide.md

# Load a skill with its transitive dependencies in topological order
dojo load dojo --with-deps
```

Dependency resolution reads the `dependencies` frontmatter field, orders the
graph topologically, and rejects cycles.

## One store, many harnesses

`.agents/skills/<name>/SKILL.md` is the canonical store. Dojo reconciles edits
made directly inside a harness folder back into that store, then projects the
canonical copy outward. Reconciliation is bi-directional by default; pass
`--one-way` to project outward only.

| Target (`--target`) | Projection path | Harness |
| :--- | :--- | :--- |
| `agents` | `.agents/skills/<name>/SKILL.md` | Universal canonical store |
| `antigravity` / `gemini` | `.gemini/skills/<name>` or `.antigravity/skills/<name>` | Antigravity Desktop and CLI |
| `claude` | `.claude/skills/<name>/SKILL.md` | Claude Code |
| `cursor` | `.cursor/rules/<name>.mdc` | Cursor, with `.mdc` to `SKILL.md` transpilation |
| `windsurf` | `.windsurf/rules/<name>.md` | Windsurf |
| `opencode` | `.opencode/skills/<name>/SKILL.md` | OpenCode |
| `sila` | `.sila/skills/<name>/SKILL.md` | Sila continuity engine |

```bash
# Install and project to every detected harness
dojo learn dojo --with-deps --target all

# Reconcile edits and project to Antigravity, Cursor, and Claude
dojo sila --target antigravity,cursor,claude

# Project outward without importing harness edits
dojo sync --one-way
```

## Portable skill library

Link a Git repository once as your Personal Skills Registry. `dojo profile
sync` reconciles local harness skills, pulls missing skills from the registry,
rewrites `skills.json`, projects the result, and pushes local additions back.
On a second machine or a cloud VM, `dojo profile pull` restores the library in
one command.

```bash
# Link once per machine
dojo profile link github:jadmadi/agy-vm -g

# Bi-directional sync: local harnesses <-> skills.json <-> Git registry
dojo profile sync

# Restore the library on a VM or new machine
dojo profile pull -g

# Push specific skills, list numbers, rules, or slash commands
dojo profile push my-skill my-slash-command -g
dojo profile push github:jadmadi/agy-vm 1,4,6 -g
dojo profile push github:jadmadi/agy-vm ~/.claude/commands/deploy.md
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

Before a push, Dojo scans packaged skill directories for hardcoded secrets,
prompt-injection overrides, and unsafe remote execution pipes. `--force`
overrides the gate when you have reviewed the finding.

## Agents and automation

`dojo mcp` starts a zero-dependency JSON-RPC 2.0 server over stdio and exposes
`dojo_find`, `dojo_toc`, `dojo_load`, `dojo_run`, `dojo_learn`, `dojo_doctor`,
`dojo_sync`, and `dojo_profile`.

Tool results return JSON text plus `structuredContent` that matches each
tool's `outputSchema`. Every tool carries MCP annotations, so a client can
auto-approve `find`, `toc`, `load`, and `doctor` while prompting on `run`,
`learn`, `sync`, and `profile`. The server negotiates protocol version
`2025-06-18` or an older version the client requests, and sends usage
instructions once at initialization. `dojo_learn`, `dojo_sync`, and
`dojo_profile push` accept `dry_run` to preview work without writing.

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

```bash
# Print the JSON Schema definitions for every tool
dojo mcp --print-schema

# Dynamic tab completion, including live section slugs
source <(dojo completion bash)     # bash
source <(dojo completion zsh)      # zsh
dojo completion fish | source      # fish
```

## Trust and provenance

- Catalogs are verified with SHA-256 digests before content is used.
- Dojo publishes an Agent Skills Discovery RFC v0.2.0 index at
  `.well-known/agent-skills/index.json` in this repository.
- `dojo doctor --skills --security` validates frontmatter, checks single-page
  byte budgets, and scans for secrets, prompt-injection patterns, and `curl | sh`
  pipes. Security findings exit non-zero so CI can gate on them.
- `dojo init <name>` scaffolds `SKILL.md`, `references/`, `scripts/`, and
  `evals/` so a new skill starts in a checkable shape.

## Configuration

Dojo reads `~/.config/dojo/config.yaml` and caches under `~/.cache/dojo` by
default.

| Variable | Default value | Description |
| :--- | :--- | :--- |
| `DOJO_URL` | `https://raw.githubusercontent.com/jadmadi/dojo/main/.well-known/agent-skills/index.json` | Default remote catalog manifest URL |
| `DOJO_PROFILE_REPO` | *(empty)* | Default Personal Skills Registry target (`github:<owner>/<repo>` or a directory) |
| `DOJO_CONFIG_DIR` | `~/.config/dojo` | Directory for `config.yaml` and `state.json` |
| `DOJO_CACHE_DIR` | `~/.cache/dojo` | Directory for cached manifests and skills |
| `DOJO_TELEMETRY` | `0` (opt-in) | Set `DO_NOT_TRACK=1` to disable telemetry |

## Development

```bash
git clone https://github.com/jadmadi/dojo.git
cd dojo
go test -race ./...
go run ./cmd/dojo doctor --skills --security
```

## Architecture

Dojo is a set of focused internal packages with no runtime daemons, compiled
with `CGO_ENABLED=0` for Linux, macOS, and Windows.

| Package | Responsibility |
| :--- | :--- |
| `cmd/dojo` | CLI entrypoint |
| `internal/catalog` | RFC v0.2.0 discovery, SHA-256 verification, Markdown section parser, dependency resolver |
| `internal/commands` | Subcommand parsing, completion, exit-code mapping, output formatting |
| `internal/harness` | Harness discovery, reconciliation, symlink projection, Cursor `.mdc` transpilation |
| `internal/profile` | `skills.json` manifest builder and loader for local files, HTTP, and GitHub |
| `internal/mcp` | JSON-RPC 2.0 MCP server and tool router |
| `internal/runner` | Companion script runner |
| `internal/search` | Hybrid BM25, TF-IDF, prefix, and fuzzy search |
| `internal/state` | Cross-platform locked state for installed skills, catalogs, and usage history |

## Why the name Dojo?

A dojo (道場) is a training ground. Dojo is the training ground for agent
skills: author and lint them with `dojo init` and `dojo doctor`, keep them
consistent with `dojo sync`, and carry them with `dojo profile`.

## License and contributing

Licensed under the [Apache License 2.0](LICENSE). Contributions are welcome.
See [CONTRIBUTING.md](CONTRIBUTING.md), [BACKLOG.md](BACKLOG.md), and
[CREDITS.md](CREDITS.md).
