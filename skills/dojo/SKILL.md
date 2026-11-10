---
name: dojo
description: >
  "Enter the Dojo!": Agent Skills Training Ground, Native MCP Gateway, and Multi-Harness
  Continuity Engine. Activate this skill to discover skills with hybrid BM25+RRF search,
  inspect Markdown section TOCs (`dojo toc`), JIT load specific #sections or full skills
  with dependency graphs (`dojo load`), bi-directionally synchronize skills across Antigravity/Claude/Cursor/Windsurf/OpenCode/Sila
  (`dojo sync` / `dojo sila`), link a Personal Skills Registry and sync portable `skills.json` profiles and local commands/skills between local machines and `agy-vm` (`dojo profile link|sync|pull|push|apply`),
  initialize workspaces (`dojo init --workspace`), scaffold and audit skills (`dojo init` / `dojo doctor`),
  run companion scripts (`dojo run`), or expose Dojo as a native MCP server (`dojo mcp`).
license: Apache-2.0
metadata:
  category: agents
  tags: "dojo, gateway, mcp, multi-harness, antigravity, sila, profile, agy-vm, agent-skills, discovery, search, catalog, cli"
  author: Jad Madi (github.com/jadmadi)
  version: "1.4.1"
  catalog: https://github.com/jadmadi/dojo
---

# Dojo — Agent Skills Training Ground & Multi-Harness Continuity Engine

`dojo` is a static Go CLI and native MCP server for finding, streaming, auditing, and bi-directionally synchronizing Agent Skills (`SKILL.md`) and agentic commands across local workstations and cloud VMs (`agy-vm`).

## 1. Search & Section-Level Paging (`dojo find`, `dojo toc`, `dojo load`)

Inspect a skill's Table of Contents first, then stream only the relevant
`#section` so the agent reads what the task needs. `dojo toc` reports the byte
size of every section before anything is loaded:

```bash
# Search catalogs with hybrid BM25 + fuzzy matching
dojo find "go testing"

# Inspect headings, #slugs, line ranges, and byte sizes
dojo toc godoctor

# Load only a single section by #slug or --section
dojo load godoctor#quick-start

# Load a skill with all transitive dependencies in topological order
dojo load fullstack-go --with-deps
```

## 2. Bi-Directional Multi-Harness Continuity (`dojo sync` / `dojo sila`)

`dojo` reconciles newly created or edited skills from any harness back into `.agents/skills/<name>/SKILL.md` (including reverse-transpiling Cursor `.mdc` rules) and projects canonical skills across all active harnesses:

- **Antigravity (Desktop & CLI)**: `.gemini/skills/<name>` or `.antigravity/skills/<name>`
- **Claude Code**: `.claude/skills/<name>`
- **Cursor**: `.cursor/rules/<name>.mdc` (bi-directional `.mdc` <-> `SKILL.md` conversion)
- **Windsurf**: `.windsurf/rules/<name>.md`
- **OpenCode**: `.opencode/skills/<name>`
- **Sila**: `.sila/skills/<name>`

```bash
# Learn a skill and immediately sync to all harnesses
dojo learn godoctor --with-deps --target all

# Bi-directionally reconcile and project skills across harnesses
dojo sila --target antigravity,cursor,claude
```

## 3. Personal Skills Registry & Local-to-Cloud (`agy-vm`) Continuity

Link a GitHub repository as your Personal Skills Registry once, then use `dojo profile sync` or `dojo profile pull` across local workstations and cloud VMs (`agy-vm`):

```bash
# 1. Link your personal skills registry (once per machine):
dojo profile link github:jadmadi/agy-vm -g

# 2. Two-way sync (reconciles local harnesses, pulls remote skills, updates skills.json, and pushes local skills to GitHub):
dojo profile sync

# 3. On agy-vm (or any target machine): pull and project all skills from your linked registry:
dojo profile pull -g

# 4. Push specific skills, list indices, rules (.mdc), or slash commands (.claude/commands/*.md):
dojo profile push <x> <y>
dojo profile push github:jadmadi/agy-vm ~/.claude/commands/x.md ~/.agents/skills/y
```

What `dojo profile sync` and `dojo profile push` search and package automatically:
- **Skill directories** (including `SKILL.md`, `scripts/`, `references/`, `assets/`): `.agents/skills/<name>`, `.gemini/skills/<name>`, `.antigravity/skills/<name>`, `.claude/skills/<name>`, `.opencode/skills/<name>`, `.sila/skills/<name>` (in current workspace and `~/`).
- **Rules & Slash Commands** (auto-converted into compliant `SKILL.md` packages with frontmatter): `.cursor/rules/<name>.mdc`, `.windsurf/rules/<name>.md`, `.claude/commands/<name>.md`, `.gemini/commands/<name>.md`, `.agents/workflows/<name>.md`, `.windsurf/workflows/<name>.md`.
- For complex multi-file command migrations, load the full reference guide: `dojo load dojo/references/migration-guide.md`.

## 4. Native MCP Server & Shell Completions (`dojo mcp` & `dojo completion`)

```bash
# Start JSON-RPC 2.0 MCP server on stdio or inspect tool schema
dojo mcp
dojo mcp --print-schema

# Enable dynamic shell tab-completion (skills, #sections, flags, targets)
source <(dojo completion bash)
source <(dojo completion zsh)
```

## 5. Skill Authoring, Linting & Security Auditing (`dojo init` & `dojo doctor`)

```bash
# Initialize a multi-harness workspace (.agents/skills, skills.json, and harness projections)
dojo init --workspace

# Scaffold a new compliant skill package (SKILL.md, scripts/, references/, evals/evals.json)
dojo init my-new-skill --category coding

# Audit config, frontmatter compliance, token budgets, and scan for prompt injection / unsafe shell pipes
dojo doctor --skills --security
```
