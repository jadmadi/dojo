# Dojo Project Backlog

This backlog tracks roadmap items and completed releases for `dojo` (`github.com/jadmadi/dojo`, evolved from `kungfu`).

## Completed (`v1.1.0` — Bi-Directional Continuity, Traveling `skills.json`, Workspace Init & Shell Completions)

- [x] **Bi-Directional Cross-Harness Reconciliation (`dojo sync` / `dojo sila`)**: Added `ReconcileHarnesses` and `ConvertFromCursorMDC` (`internal/harness/sync.go`) so skills created or edited directly inside `.gemini/skills` (Antigravity), `.antigravity/skills`, `.claude/skills`, `.cursor/rules/*.mdc`, `.windsurf/rules/*.md`, `.opencode/skills`, or `.sila/skills` are reconciled back into canonical `.agents/skills/<name>/SKILL.md` before projecting across all harnesses (with `--one-way` to opt out).
- [x] **Traveling `skills.json` & Git-Backed Profile (`dojo profile export|apply|sync`)**: Added `internal/profile` and `dojo profile` to export, apply (from local path, HTTP URL, or `github:owner/repo`), and sync declarative `skills.json` manifests across local laptops and cloud VMs (`agy-vm`).
- [x] **One-Command Workspace Onboarding (`dojo init --workspace` / `-w`)**: Auto-detects active harnesses, reconciles existing skills into `.agents/skills/`, applies or generates `skills.json`, and wires projections in one command.
- [x] **Dynamic Shell Completions (`dojo completion` & `dojo __complete`)**: Added tab-completion scripts for `bash`, `zsh`, `fish`, and `powershell` with live completion for skill names, `#<section>` slugs, subcommands, flags, and `--target` harnesses.
- [x] **Cross-OS Installer & Release Automation (`scripts/install.sh` & `.goreleaser.yaml`)**: Added one-line POSIX installer for Linux/macOS/`agy-vm` and multi-OS static build configuration (`linux`, `darwin`, `windows` across `amd64` and `arm64`).

## Completed (`v1.0.0` — The 7-Pillar Dojo Release)

- [x] **Rebrand & Backward Compatibility**: Renamed module to `github.com/jadmadi/dojo` and binary to `dojo` (`cmd/dojo`), with automatic fallback to `~/.config/kungfu`, `~/.cache/kungfu`, and `KUNGFU_*` environment variables.
- [x] **Section-Level Hypertext Paging (`dojo toc` & `dojo load <skill>#<section>`)**: Added Markdown section parser (`internal/catalog/sections.go`) that ignores `#` lines inside fenced code blocks, `dojo toc <skill>` for inspecting heading slugs/line ranges/byte budgets, and `#<section>` / `--section` extraction in `dojo load`.
- [x] **Skill Dependency Graph (`dependencies` & `--with-deps`)**: Added `dependencies` frontmatter parsing and topological DAG resolution with cycle detection (`internal/catalog/deps.go`), wired into `dojo load --with-deps` and `dojo learn --with-deps`.
- [x] **Multi-Harness Auto-Sync (`dojo sync` / `dojo sila` & `dojo learn --target`)**: Added cross-harness projection (`internal/harness/sync.go`) across `.agents/skills`, `.claude/skills`, `.cursor/rules/*.mdc` (auto-converted to `.mdc`), `.windsurf/rules/*.md`, `.gemini/skills`, `.opencode/skills`, and `.sila/skills`.
- [x] **Native MCP Server (`dojo mcp`)**: Added zero-dependency JSON-RPC 2.0 Model Context Protocol stdio server (`internal/mcp/server.go`) exposing `dojo_find`, `dojo_toc`, `dojo_load`, `dojo_run`, `dojo_learn`, and `dojo_doctor` (plus `--print-schema`).
- [x] **Skill Scaffolder & Security Doctor (`dojo init` & `dojo doctor`)**: Added `dojo init <name>` to scaffold compliant skill packages and `dojo doctor [--skills] [--security]` for YAML frontmatter linting, token budget analysis, and static prompt-injection / `curl | sh` security scanning.
- [x] **Self-Upgrade & Multi-Catalog Federation (`dojo upgrade`)**: Added `dojo upgrade [--check] [--force]` and pre-configured community catalogs (`default`, `dojo`, `kungfu`, `gemini-skills`).

## Completed (`v0.6.1`)

- [x] Centralized updates (`kungfu update`): Removed `kungfu catalog sync` so `kungfu update` synchronizes all registered catalogs (`default`, `github`, `http`, `local`), flushes opt-in telemetry, and checks or updates installed skills.
- [x] Top-5 Hybrid "Did You Mean" (`kungfu load`, `show`, `run`, `learn`): Expanded `Did you mean` suggestions to combine `search.Suggest` with hybrid BM25 + TF-IDF search (`Searcher.Search(..., "hybrid", 5, 0.0)`) and 1-line `# <description>` summaries.
- [x] Simplified Shortcut Language (`skills/kungfu/SKILL.md`): Consolidated Section 5 around the four organic operators (`::skill`, `::?keyword`, `::skill(params...)`, `A -> B`) and three core language rules (First-Mention Hint Rule, Non-Rigid Specialization, Catalog & Reference Scoping).

## Completed (`v0.6.0`)

- [x] RFC v0.2.0 Discovery Index (`.well-known/agent-skills/index.json`): Switched default catalog URL and discovery precedence to `.well-known/agent-skills/index.json` -> `index.json` -> `catalog.json` (kept for 2 versions through `v0.7.0`) -> directory scan.
- [x] Relative URL & freshness support: Added relative URL resolution for `.well-known` manifests and preserved `freshnessScore`, `freshnessTier`, `freshnessEmoji`, and `lastCommitAt` metadata.
- [x] Version skew and update sync fixes: Fixed `kungfu update` to always refresh remote catalog manifests, updated `ReconcileDisk` to sync version changes on disk, resolved named-catalog semver shadowing in `loadNamedCatalogsLocked`, and made `kungfu load -u` refresh the upstream index before loading.

## Completed (`v0.5.2`)

- [x] Opt-in community telemetry (`kungfu telemetry`): Added `telemetry.enabled: false` (`DO_NOT_TRACK=1` aware), watermark tracking in `state.json`, `kungfu telemetry [status|enable|disable|sync]`, and bounded (`<= 100`) counter flushes to Cloud Firestore during `kungfu catalog sync`.
- [x] Config and lock hardening: Removed Firebase project and key fields from `config.yaml`, enforced fail-fast errors on missing `--config` files, added `EINTR` retry on POSIX `flock`, and isolated all test packages from user config and state files.
- [x] CLI and docs cleanup (`less-is-more`): Simplified help, status, and README copy, and removed stale flags and duplicate sections.

## Completed (`v0.5.1`)

- [x] Local usage history (`state.json`): Restored load tracking in `kungfu load` and added tracking for `kungfu learn` and `kungfu run`.
- [x] Separate page and subpath counters: Added `page_load_count`, `subpath_load_count`, `run_count`, and `learn_count` so root `SKILL.md` loads (`load_count`) stay separate from page-2+ reads and reference loads.
- [x] Cross-process file locking (`flock`): Added `WithLockedState` (`state.json.lock` via `flock` on POSIX and `LockFileEx` on Windows) so concurrent `::parallel` subagents never overwrite each other's updates.
- [x] Test state isolation: Isolated `TestMain` with temporary `KUNGFU_STATE_PATH` folders so `go test ./...` never mutates user state.

## Completed (`v0.5.0`)

- [x] Fail-fast error handling: Removed silent fallbacks, stale cache fallbacks, and legacy shims across `catalog`, `config`, `state`, and `runner`.
- [x] 8 KiB buffer ceiling and pagination: Added pagination to `kungfu load` (`--full`, `-p`/`--page`, `--page-size`) with UTF-8 rune slicing to prevent truncation in 8 KiB agent command buffers.
- [x] Single-skill loading: Disabled multi-skill batch loading (`kungfu load a b`) so paged output never interleaves.
- [x] Decoupled skill catalog: Removed embedded binaries and exposed the repo itself as a GitHub skill catalog.
- [x] Security hardening: Added symlink boundary checks (`isSafeSkillDirectory`) and path containment checks (`isContained`).
- [x] Google Open Source compliance: Added `CONTRIBUTING.md`, `NOTICE`, and Apache 2.0 headers across source files.

## Completed (`v0.4.0`)

- [x] User config (`~/.config/kungfu/config.yaml`): Added `internal/config` for custom catalog URLs and `load.auto_update` settings.
- [x] Unified catalog sync: Added the default catalog to `state.json` so `kungfu catalog sync` refreshes all catalogs the same way.
- [x] Refresh modes for `kungfu load`: Added `--local`, `-u`/`--update`, and `-f`/`--force` flags with upstream digest drift checks.
- [x] CLI cleanup: Removed category filter flags, flattened `kungfu list`, and dropped boolean tag query flags in favor of keyword search.
- [x] Inverted pyramid `SKILL.md`: Rebuilt `skills/kungfu/SKILL.md` around core workflows (`find`, `load`, `run`, `::notation`, `learn`).

## Completed (`v0.3.0`)

- [x] Multi-catalog support (`kungfu catalog`): Added `add`, `list`, `remove`, and `sync` for GitHub repos, HTTP manifests, and local folders.
- [x] Multi-format discovery: Ordered checks as `catalog.json` -> `index.json` -> folder tree scan.
- [x] Modular command package: Split command handlers into focused files under `internal/commands/`.
- [x] Disk sync: Updated `ReconcileDisk` to detect skills added or removed outside `kungfu`.

## Completed (`v0.2.1`)

- [x] Build info version lookup: Added `internal/version` using `runtime/debug.ReadBuildInfo()`.
- [x] Schema cleanup: Aligned `catalog.Item` with the catalog spec.

## Completed (`v0.2.0`)

- [x] Direct GitHub loading: Added support for loading and learning skills from GitHub (`owner/repo`, `@tag`, `#branch`) over raw HTTP without `git` or `npm`.
- [x] `kungfu add` alias: Added `add` as an alias for `learn`.
- [x] Provenance and drift checks: Saved origin repo, ref, and package digests in `state.json` for `kungfu update`.

## Completed (`v0.1.1`)

- [x] Remote script runner (`kungfu run`): Cached and ran companion scripts on demand without `kungfu learn`.
- [x] Companion file install: Installed referenced `scripts/`, `references/`, and `assets/` alongside `SKILL.md`.

## Completed (`v0.1.0`)

- [x] Initial release: Built catalog lookup, SHA-256 digest checks, ETag caching, BM25 + TF-IDF search (`kungfu find`), and state commands (`status`, `update`, `forget`).



