---
name: dojo
description: >
  "Enter the Dojo!": Agent Skills Training Ground, Native MCP Gateway, and Multi-Harness
  Continuity Engine. Activate this skill to discover skills with hybrid BM25+RRF search,
  inspect Markdown section TOCs (`dojo toc`), JIT load specific #sections or full skills
  with dependency graphs (`dojo load`), synchronize skills across Claude/Cursor/Gemini/Windsurf/OpenCode/Sila
  (`dojo sync` / `dojo sila`), scaffold and audit skills (`dojo init` / `dojo doctor`),
  run companion scripts (`dojo run`), or expose Dojo as a native MCP server (`dojo mcp`).
license: Apache-2.0
metadata:
  category: agents
  tags: "dojo, gateway, mcp, multi-harness, sila, agent-skills, discovery, search, catalog, cli"
  author: Jad Madi (github.com/jadmadi)
  version: "1.0.0"
  catalog: https://github.com/jadmadi/dojo
---

# Dojo — Agent Skills Training Ground & Multi-Harness Continuity Engine

`dojo` is a 10x CLI & native MCP gateway that discovers, streams, audits, and synchronizes Agent Skills across every AI coding harness.

## 1. Search & Discovery (`dojo find` / `::?`)

```bash
dojo find go testing
dojo find security
dojo find react
```

## 2. Section-Level Hypertext Paging (`dojo toc` & `dojo load <skill>#<section>`)

Save context tokens by inspecting a skill's Table of Contents first and loading only the exact Markdown section you need:

```bash
# Inspect headings, #slugs, line ranges, and byte sizes
dojo toc godoctor

# Load only a single section by #slug or --section
dojo load godoctor#quick-start
dojo load godoctor --section quick-start

# Load a skill with all transitive dependencies in topological order
dojo load fullstack-go --with-deps
```

## 3. Multi-Harness Continuity Engine (`dojo sync` / `dojo sila`)

Write or learn a skill once in `.agents/skills/` and project it across every agent harness in your workspace or global home directory:

- **Claude Code**: `.claude/skills/<name>`
- **Cursor**: `.cursor/rules/<name>.mdc` (auto-converted to `.mdc` rule format)
- **Windsurf**: `.windsurf/rules/<name>.md`
- **Gemini / Antigravity**: `.gemini/skills/<name>`
- **OpenCode**: `.opencode/skills/<name>`
- **Sila**: `.sila/skills/<name>`

```bash
# Learn a skill and immediately sync to all harnesses
dojo learn godoctor --with-deps --target all

# Sync all installed workspace skills across harnesses
dojo sync --target all
dojo sila --target claude,cursor,gemini,sila
```

## 4. Native MCP Server (`dojo mcp`)

Expose Dojo's entire skill catalog and execution engine to any MCP-compatible client over stdio:

```bash
# Start JSON-RPC 2.0 MCP server on stdio
dojo mcp

# Inspect MCP tool definitions (`dojo_find`, `dojo_toc`, `dojo_load`, `dojo_run`, `dojo_learn`, `dojo_doctor`)
dojo mcp --print-schema
```

## 5. Skill Authoring, Linting & Security Auditing (`dojo init` & `dojo doctor`)

```bash
# Scaffold a new compliant skill package (SKILL.md, scripts/, references/, evals/evals.json)
dojo init my-new-skill --category coding

# Audit config, frontmatter compliance, token budgets, and scan for prompt injection / unsafe shell pipes
dojo doctor
dojo doctor --skills --security
```

## 6. Companion Scripts & Self-Upgrade (`dojo run` & `dojo upgrade`)

```bash
# Execute bundled skill scripts
dojo run godoctor smart_build

# Check for newer Dojo releases and self-upgrade
dojo upgrade --check
dojo upgrade
```
