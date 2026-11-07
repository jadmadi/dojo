---
name: dojo
description: >
  "Enter the Dojo!": Agent Skills Training Ground, Native MCP Gateway, and Multi-Harness
  Continuity Engine. Activate this skill to discover skills with hybrid BM25+RRF search,
  inspect Markdown section TOCs (`dojo toc`), JIT load specific #sections or full skills
  with dependency graphs (`dojo load`), bi-directionally synchronize skills across Antigravity/Claude/Cursor/Windsurf/OpenCode/Sila
  (`dojo sync` / `dojo sila`), manage portable `skills.json` manifests (`dojo profile`),
  initialize workspaces (`dojo init --workspace`), scaffold and audit skills (`dojo init` / `dojo doctor`),
  run companion scripts (`dojo run`), or expose Dojo as a native MCP server (`dojo mcp`).
license: Apache-2.0
metadata:
  category: agents
  tags: "dojo, gateway, mcp, multi-harness, antigravity, sila, profile, agent-skills, discovery, search, catalog, cli"
  author: Jad Madi (github.com/jadmadi)
  version: "1.1.0"
  catalog: https://github.com/jadmadi/dojo
---

# Dojo — Agent Skills Training Ground & Multi-Harness Continuity Engine

`dojo` is a static Go CLI and native MCP server for finding, streaming, auditing, and bi-directionally synchronizing Agent Skills (`SKILL.md`) across local workstations and cloud VMs (`agy-vm`).

## 1. Search & Section-Level Paging (`dojo find`, `dojo toc`, `dojo load`)

Inspect a skill's Table of Contents first and stream only the relevant `#section` to save 70–90% of context tokens:

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

## 3. Traveling `skills.json` & Workspace Onboarding (`dojo profile` & `dojo init --workspace`)

Keep a portable `skills.json` manifest in your project or Git profile repo (`jadmadi/agy-vm`) so your skills travel across local machines and cloud VMs:

```bash
# One-command workspace bootstrap (.agents/skills, skills.json, and harness projections)
dojo init --workspace

# Export, apply, or sync skills.json (local file, HTTP URL, or github:owner/repo)
dojo profile export
dojo profile apply github:jadmadi/agy-vm
dojo profile sync --push
```

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
# Scaffold a new compliant skill package (SKILL.md, scripts/, references/, evals/evals.json)
dojo init my-new-skill --category coding

# Audit config, frontmatter compliance, token budgets, and scan for prompt injection / unsafe shell pipes
dojo doctor --skills --security
```
