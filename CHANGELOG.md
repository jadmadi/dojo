# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/jadmadi/dojo/compare/v1.1.7...HEAD
[1.1.7]: https://github.com/jadmadi/dojo/compare/v1.1.6...v1.1.7
[1.1.6]: https://github.com/jadmadi/dojo/compare/v1.1.5...v1.1.6
[1.1.5]: https://github.com/jadmadi/dojo/compare/v1.1.4...v1.1.5
[1.1.4]: https://github.com/jadmadi/dojo/compare/v1.1.3...v1.1.4
[1.1.3]: https://github.com/jadmadi/dojo/compare/v1.1.2...v1.1.3
[1.1.2]: https://github.com/jadmadi/dojo/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/jadmadi/dojo/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/jadmadi/dojo/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/jadmadi/dojo/releases/tag/v1.0.0
