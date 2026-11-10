# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.5.1] - 2026-11-10

### Added
- **Context Savings Benchmark**: A four-skill corpus under `internal/catalog/testdata/bench-skills`, a published `docs/benchmarks/context-savings.json`, and `TestContextSavings`, which enforces a 45 percent ceiling on the largest leaf section and fails when the published numbers drift.

### Changed
- **README Claim**: Section paging is now described as an enforced ceiling with published evidence instead of an unmeasured percentage.

## [1.5.0] - 2026-11-10

### Added
- **Agent-First MCP Surface**: every tool now returns JSON text plus `structuredContent` matching a published `outputSchema`, carries MCP annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`), and the server sends usage instructions at initialization. Protocol version is negotiated from `2025-06-18`, `2025-03-26`, or `2024-11-05` instead of being hard-coded.
- **Dry-Run Previews**: `dojo learn --dry-run`, `dojo sync --dry-run`, and `dojo profile push --dry-run` report planned work without writing files, and the MCP tools expose the same `dry_run` argument.

### Changed
- **MCP Tool Output**: tool results are JSON for `find`, `toc`, `load`, `doctor`, `learn`, `sync`, and `profile`, so agents no longer parse padded terminal tables. `dojo_run` keeps raw script output.

## [1.4.1] - 2026-11-10

### Added
- **Toc Refresh Flags**: `dojo toc -u/--update` refreshes the cached skill before listing sections, and `-f/--force` overrides digest verification. Both flags were documented but missing from the command.

### Changed
- **Remaining Handler Refactor**: Split `InstallSkillContext`, `RunDoctor`, `runCatalogAdd`, `probeGitHubCatalogRef`, `RunList`, and `RunInit` into named helpers with unchanged flags, output, and exit codes.
- **Deterministic Scaffolding**: `dojo init` now writes and reports generated files in a fixed order instead of map iteration order.
- **Claim Audit**: README and `skills/dojo/SKILL.md` state measured behaviour, scope `--json`/`--raw` to reporting commands, separate `status` from `update`, and describe the published discovery index accurately.

### Fixed
- **Profile Pull Manifest**: `skills.json` no longer records a catalog URL where a catalog name belongs, so `dojo profile pull` installs listed skills instead of silently installing zero. Failed manifest installs now warn on stderr.

## [1.4.0] - 2026-11-10

### Changed
- **Command Structure Refactor**: Split the largest command handlers into named helpers with unchanged behavior: `update`, `profile push`, `learn`, `load`, `status`, and the top-level `Run` dispatcher. This lowers cognitive complexity without changing flags, output, or exit codes.
- **Reader and Reconciler Refactor**: Split `runner.readBytesInternal` into path validation, local lookup, archive download, and remote lookup steps. Split `harness.ReconcileHarnesses` into one helper per harness family.
- **Search Deduplication**: `searchBM25` and `searchTFIDF` now share candidate scoring, mask combination, and ranking.
- **State History Consolidation**: `RecordRun`, `RecordLearn`, and `RecordLoadDetailed` share one locked history upsert.

### Added
- **Catalog Metadata (`project.jsonc`)**: Authoritative project metadata for the jadmadi.net project catalog, synchronized with the site on each release.
- **README Restructure**: Answer-first README that states current capabilities, documents the `skills` CLI install path, and drops the unmeasured token-savings percentage.

### Fixed
- **Release Workflow Toolchain**: The release workflow now uses Go 1.26, matching `go.mod`.

## [1.3.0] - 2026-11-07

### Added
- **Pre-Push Secret & Security Gate (`dojo profile push` / `sync`)**: Automatically scans packaged skill directories for hardcoded secrets (`ghp_`, `github_pat_`, `sk-`, `AIza`, `AKIA`, `xox`, private keys), prompt-injection overrides, and unsafe remote execution pipes before committing or pushing to GitHub (override with `--force`).
- **Complete MCP Server Surface (`dojo_sync` & `dojo_profile`)**: Added `dojo_sync` and `dojo_profile` tools (plus `--skills`, `--security`, and `-g` parameters on `dojo_doctor`) to `dojo mcp`.
- **Archive Extraction in `dojo upgrade`**: Self-upgrade now extracts `.tar.gz` (Linux/macOS) and `.zip` (Windows) release archives published by GoReleaser in addition to raw binaries.
- **GitHub Actions CI & GoReleaser Workflows**: Added `.github/workflows/ci.yml` and `.github/workflows/release.yml` for automated multi-OS binary distribution.
- **Automated `.well-known` Digest Verification Test**: Added `TestWellKnownIndexDigestsMatchSkills` ensuring `.well-known/agent-skills/index.json` SHA-256 digests always match `skills/*/SKILL.md`.

### Changed
- **Blocking Security Audit (`dojo doctor --security`)**: Security and secret findings now emit `"error"` severity and exit with code `1` (`ExitGeneral`) so CI pipelines fail on security violations.
- **Standalone Dojo Catalog & Config Defaults**: Default catalog (`DOJO_URL`) now points to `https://raw.githubusercontent.com/jadmadi/dojo/main/.well-known/agent-skills/index.json` with zero legacy external catalog coupling.

## [1.2.0] - 2026-11-07

### Added
- **Personal Skills Registry (`dojo profile link` / `unlink`)**: Link a persistent GitHub repository (`github:<owner>/<repo>`) or directory as your personal skills registry in `~/.config/dojo/config.yaml` (or via `DOJO_PROFILE_REPO`).
- **Two-Way Registry Continuity (`dojo profile sync`)**: When linked to a personal registry, `dojo profile sync` automatically reconciles local harness skills (`.gemini/skills`, `.claude/skills`, `.cursor/rules`, etc.), pulls any missing skills from the remote registry, updates `skills.json`, projects across harnesses, and pushes local skills back to the registry.
- **Zero-Argument `dojo profile pull` & `dojo profile push`**: `dojo profile pull` (alias for `apply`) and `dojo profile push` now automatically use the linked personal registry when invoked without a target repository argument, defaulting `dojo profile push` to `--all`.
- **Fast Local Startup**: Skipped remote catalog network initialization for purely local commands (`init`, `sync`, `doctor`, `telemetry`, `upgrade`, `profile link/unlink/export`).

## [1.1.8] - 2026-11-07

### Added
- Compound numeric spec resolution: `dojo load <number>#<section>` and `dojo load <number>/<subpath>` now resolve numeric list indices into canonical skill names while preserving section anchors and subpaths.
- Universal numeric index resolution across secondary commands: `dojo toc`, `dojo run`, `dojo forget`, `dojo update`, `dojo status`, and `dojo sync` now support 1-based list numbers and comma-separated specs (matching `show`, `load`, and `learn`).
- Scope filtering in `dojo list`: added `-g` / `--global` and `-w` / `--workspace` flags to list only skills installed in global or workspace scope.
- Skill name kebab-case validation in `dojo init`: rejects invalid names containing spaces or illegal characters with an informative error.

## [1.1.7] - 2026-11-07

### Added
- Catalog fallback for `dojo profile push`: pushing skills by name or list number to a remote repository or directory now automatically fetches and packages skills from the Dojo catalog if they are not yet installed in local workspace or home harness directories.

## [1.1.6] - 2026-11-07

### Added
- Actionable `EXAMPLES` block appended to `dojo list` output detailing `dojo profile push`, `dojo show`, `dojo toc`, `dojo load`, and `dojo learn`.
- Concrete usage examples documented after every argument and flag in `dojo list` output and CLI help.
- Category and search keyword filtering support (`dojo list [category|filter]`).

## [1.1.5] - 2026-11-07

### Fixed
- Fixed self-upgrade executable discovery (`findInstalledGoBin`) under version and toolchain managers such as `mise` and `asdf` by checking the active Go directory and build timestamps.
- Fixed false-positive `✓ Upgraded dojo ...` status report when the active binary could not be replaced due to insufficient permissions; now emits a distinct `sync_required` status with exact manual copy instructions.
- Used PID-isolated temporary files (`.dojo-upgrade.<pid>.tmp`) during binary overwrite to prevent file collision.

## [1.1.4] - 2026-11-07

### Added
- Numbered index column (`#`) in human-readable `dojo list` output for fast visual indexing.
- Numeric and comma-separated skill references across `dojo show`, `dojo load`, and `dojo learn` (e.g. `dojo show 1,3`).
- Numeric selection support for `dojo profile push` (e.g. `dojo profile push github:owner/repo 1,4,6 -g`).
- Automatic executable synchronization in `dojo upgrade` when compiled via the Go toolchain.

## [1.1.3] - 2026-11-07

### Added
- Automatic bootstrap of the `dojo` skill into global harnesses upon installation.

### Fixed
- CLI UX polish addressing audit findings for branding consistency, root help display, and flag parsing ergonomics.

## [1.1.2] - 2026-11-07

### Added
- `dojo profile push` command to export and sync local skills and commands directly to GitHub repositories or local directories.
- Agent migration guide for bidirectional synchronization between local workstations and remote cloud environments (`agy-vm`).

## [1.1.1] - 2026-11-07

### Added
- Support for private GitHub repository authentication via `GITHUB_TOKEN`, `GH_TOKEN`, or GitHub CLI (`gh api`).

### Fixed
- Symlink-safe harness projection to prevent accidental recursion and cyclic symlink traversals.

## [1.1.0] - 2026-11-07

### Added
- Bi-directional multi-harness continuity and projection engine.
- Portable `skills.json` profile support for traveling configurations across machines.
- `dojo init` command for workspace initialization.
- Shell auto-completions generation for bash, zsh, and fish (`dojo completion`).

## [1.0.0] - 2026-11-07

### Added
- Initial release of Dojo: Agent Skills Training Ground, Native MCP Server, and Multi-Harness Continuity Engine.
- 7-Pillar agent skills discovery, search, and JIT loading.
- Native support for Antigravity (Desktop & CLI) across sync detection, projection, and CLI.

[Unreleased]: https://github.com/jadmadi/dojo/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/jadmadi/dojo/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/jadmadi/dojo/compare/v1.1.8...v1.2.0
[1.1.8]: https://github.com/jadmadi/dojo/compare/v1.1.7...v1.1.8
[1.1.7]: https://github.com/jadmadi/dojo/compare/v1.1.6...v1.1.7
[1.1.6]: https://github.com/jadmadi/dojo/compare/v1.1.5...v1.1.6
[1.1.5]: https://github.com/jadmadi/dojo/compare/v1.1.4...v1.1.5
[1.1.4]: https://github.com/jadmadi/dojo/compare/v1.1.3...v1.1.4
[1.1.3]: https://github.com/jadmadi/dojo/compare/v1.1.2...v1.1.3
[1.1.2]: https://github.com/jadmadi/dojo/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/jadmadi/dojo/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/jadmadi/dojo/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/jadmadi/dojo/releases/tag/v1.0.0
