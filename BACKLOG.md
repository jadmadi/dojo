# Dojo Project Backlog

This backlog tracks roadmap items and completed releases for `dojo` (`github.com/jadmadi/dojo`).

## Completed (`v1.3.0` — Prebuilt Binary Releases, Pre-Push Secret Gate, Full MCP Continuity & Standalone Catalog)

- [x] **Automated Multi-OS Release Binaries & Archive Self-Upgrade (`G-01`)**: Added `.github/workflows/release.yml` and `.github/workflows/ci.yml` and updated `dojo upgrade` (`internal/commands/upgrade.go`) to extract `.tar.gz` and `.zip` release archives across `linux`, `darwin`, and `windows` (`amd64` and `arm64`).
- [x] **RFC `v0.2.0` `.well-known` Digest Integrity Gate (`G-02`)**: Updated `.well-known/agent-skills/index.json` SHA-256 digest and added automated test verification (`TestWellKnownIndexDigestsMatchSkills`).
- [x] **Pre-Push Secret & Security Gate (`G-03` & `G-07`)**: Wired `ScanDirSecurity` into `dojo profile push` / `dojo profile sync` to block accidental secret leaks (`ghp_`, `github_pat_`, `sk-`, `AIza`, `AKIA`, private keys) and prompt injection before `git push`, and updated `dojo doctor --security` to return a non-zero exit code on security violations.
- [x] **Complete MCP Continuity Surface (`G-04`)**: Added `dojo_sync` and `dojo_profile` tools to `dojo mcp` (`internal/mcp/server.go`, `internal/commands/mcp.go`).
- [x] **Standalone Dojo Catalog & Telemetry (`G-06`)**: Pointed `DefaultRemoteURL` and `DefaultCatalogURL` directly to Dojo's canonical `.well-known/agent-skills/index.json` and decoupled all configuration, state, cache, and telemetry from legacy upstream endpoints.

## Completed (`v1.2.0` — Personal Skills Registry & Two-Way Cloud Continuity)

- [x] **Personal Skills Registry (`dojo profile link` / `unlink`)**: Link a persistent GitHub repository (`github:<owner>/<repo>`) or directory as your personal skills registry in `~/.config/dojo/config.yaml` (or via `DOJO_PROFILE_REPO`).
- [x] **Two-Way Registry Continuity (`dojo profile sync`)**: Automatically reconciles local harness skills, pulls missing skills from the remote registry, updates `skills.json`, projects across harnesses, and pushes local skills back to the registry.
- [x] **Zero-Argument `dojo profile pull` & `dojo profile push`**: Automatically uses the linked personal registry when invoked without a target repository argument.

## Completed (`v1.1.0` — Bi-Directional Continuity, Traveling `skills.json`, Workspace Init & Shell Completions)

- [x] **Bi-Directional Cross-Harness Reconciliation (`dojo sync` / `dojo sila`)**: Added `ReconcileHarnesses` and `ConvertFromCursorMDC` (`internal/harness/sync.go`) so skills created or edited directly inside `.gemini/skills` (Antigravity), `.antigravity/skills`, `.claude/skills`, `.cursor/rules/*.mdc`, `.windsurf/rules/*.md`, `.opencode/skills`, or `.sila/skills` are reconciled back into canonical `.agents/skills/<name>/SKILL.md` before projecting across all harnesses (with `--one-way` to opt out).
- [x] **Traveling `skills.json` & Git-Backed Profile (`dojo profile export|apply|sync`)**: Added `internal/profile` and `dojo profile` to export, apply (from local path, HTTP URL, or `github:owner/repo`), and sync declarative `skills.json` manifests across local laptops and cloud VMs (`agy-vm`).
- [x] **One-Command Workspace Onboarding (`dojo init --workspace` / `-w`)**: Auto-detects active harnesses, reconciles existing skills into `.agents/skills/`, applies or generates `skills.json`, and wires projections in one command.
- [x] **Dynamic Shell Completions (`dojo completion` & `dojo __complete`)**: Added tab-completion scripts for `bash`, `zsh`, `fish`, and `powershell` with live completion for skill names, `#<section>` slugs, subcommands, flags, and `--target` harnesses.
- [x] **Cross-OS Installer & Release Automation (`scripts/install.sh` & `.goreleaser.yaml`)**: Added one-line POSIX installer for Linux/macOS/`agy-vm` and multi-OS static build configuration (`linux`, `darwin`, `windows` across `amd64` and `arm64`).

## Completed (`v1.0.0` — The 7-Pillar Dojo Release)

- [x] **Section-Level Hypertext Paging (`dojo toc` & `dojo load <skill>#<section>`)**: Added Markdown section parser (`internal/catalog/sections.go`) that ignores `#` lines inside fenced code blocks, `dojo toc <skill>` for inspecting heading slugs/line ranges/byte budgets, and `#<section>` / `--section` extraction in `dojo load`.
- [x] **Skill Dependency Graph (`dependencies` & `--with-deps`)**: Added `dependencies` frontmatter parsing and topological DAG resolution with cycle detection (`internal/catalog/deps.go`), wired into `dojo load --with-deps` and `dojo learn --with-deps`.
- [x] **Multi-Harness Auto-Sync (`dojo sync` / `dojo sila` & `dojo learn --target`)**: Added cross-harness projection (`internal/harness/sync.go`) across `.agents/skills`, `.claude/skills`, `.cursor/rules/*.mdc` (auto-converted to `.mdc`), `.windsurf/rules/*.md`, `.gemini/skills`, `.opencode/skills`, and `.sila/skills`.
- [x] **Native MCP Server (`dojo mcp`)**: Added zero-dependency JSON-RPC 2.0 Model Context Protocol stdio server (`internal/mcp/server.go`) exposing `dojo_find`, `dojo_toc`, `dojo_load`, `dojo_run`, `dojo_learn`, and `dojo_doctor` (plus `--print-schema`).
- [x] **Skill Scaffolder & Security Doctor (`dojo init` & `dojo doctor`)**: Added `dojo init <name>` to scaffold compliant skill packages and `dojo doctor [--skills] [--security]` for YAML frontmatter linting, token budget analysis, and static prompt-injection / `curl | sh` security scanning.
- [x] **Self-Upgrade & Multi-Catalog Federation (`dojo upgrade`)**: Added `dojo upgrade [--check] [--force]` and pre-configured community catalogs (`default`, `dojo`, `gemini-skills`).
