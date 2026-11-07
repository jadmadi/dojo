# CLI UX & Ergonomics Review: `dojo`

Conducted by **`cli-reviewer`** following the persona-based CLI evaluation framework.

---

## 1. Executive Summary & Scorecard

`dojo` is a fast, responsive, and robust CLI tool. It scores very high on terminal ergonomics, with smart Levenshtein-distance typo correction, clean ANSI escape suppression when piped to files/streams, strict stdout/stderr stream separation, and reliable JSON contracts across discovery and audit subcommands.

However, a few branding artifacts from its sister project `kungfu`, an incomplete root help summary for the `profile` subcommand, and flag ambiguity around `-v` warrant quick polish.

| Evaluation Area | Rating | Verdict |
| :--- | :---: | :--- |
| **Out-of-Tree Build** | **A+** | Clean static build with `CGO_ENABLED=0`, zero external runtime dependencies. |
| **Impatient Beginner** | **B+** | Clear command list and help flag behavior, but `-v` conflicts with `--verbose`, and root help omits `push`. |
| **Clumsy User** | **A** | Outstanding typo suggestions and strict exit code `2` on syntax errors. |
| **Script Automator** | **A+** | Flawless stdout/stderr separation, valid JSON contracts, and clean non-TTY output. |

---

## 2. Target Discovery & Out-of-Tree Build

- **Source Entry Point**: `cmd/dojo/main.go`
- **Build Invocation**:
  ```bash
  CGO_ENABLED=0 go build -o /tmp/dojo-review ./cmd/dojo
  ```
- **Observed Result**: Built cleanly in 1.4s, zero compiler warnings, static binary size ~18MB.

---

## 3. Persona Evaluations & Receipts

### 3.1 Impatient Beginner (Help, Flags, First Impressions)

#### Observed Behaviors
1. **Help & Root Invocation**:
   - `dojo --help`, `dojo -h`, and `dojo help` all exit `0` with structured command categorization.
   - Bare `dojo` without arguments outputs the help screen with exit code `2` (standard POSIX syntax error convention).
2. **Version Flag Friction (`-v` vs `-V`)**:
   - `dojo --version`, `dojo version`, and `dojo -V`: correctly print `dojo version 1.1.2` (exit `0`).
   - `dojo -v`: prints:
     ```
     error: unknown command "".
     Run 'dojo --help' for available commands and usage.
     ```
     *(Exit: 2)*.
   - *Issue*: `-v` is mapped to `--verbose` instead of `--version`. When users pass `dojo -v` with no subcommand, it fails with an awkward empty command error (`unknown command ""`).

---

### 3.2 Clumsy User (Typos, Missing Args & Exit Codes)

#### Observed Behaviors
1. **Typo Tolerance**:
   ```bash
   $ dojo fnd
   error: unknown command "fnd".

   Did you mean:
     - dojo find
   (Exit: 2)
   ```
   ```bash
   $ dojo profiel
   error: unknown command "profiel".

   Did you mean:
     - dojo profile
   (Exit: 2)
   ```
   *Receipt*: Fuzzy suggestion works smoothly and exits `2` on invalid command invocation.
2. **Missing Required Arguments**:
   - `dojo load` -> `error: load requires a skill name or subpath (e.g. 'dojo load git/reference' or 'dojo load git#workflow')` *(Exit 2)*
   - `dojo toc` -> `error: toc requires a skill name or subpath (e.g. 'dojo toc godoctor')` *(Exit 2)*
   - `dojo show` -> `error: show requires at least one skill name` *(Exit 2)*
   - `dojo profile push` -> `error: profile push requires a target repository/directory and at least one skill/command name (or --all)` *(Exit 2)*
3. **Exit Code Disciplines**:
   - Syntax error / missing argument: **`2`**
   - Resource not found (`dojo load nonexistent-xyz`): **`3`**
   - Clean differentiation between bad usage (`EX_USAGE`) and missing data (`ExitNotFound`).

---

### 3.3 Script Automator & Unix Citizen (Streams, JSON & Pipes)

#### Observed Behaviors
1. **Stdout vs Stderr Separation**:
   ```bash
   $ dojo load nonexistent > /tmp/out 2> /tmp/err
   ```
   - `/tmp/out`: `0 bytes` (clean)
   - `/tmp/err`: Contains the full error and suggestions.
2. **JSON Contracts**:
   - `dojo find <term> --json | jq .` -> **VALID JSON**
   - `dojo toc <skill> --json | jq .` -> **VALID JSON**
   - `dojo profile export --stdout | jq .` -> **VALID JSON**
   - `dojo doctor --json | jq .` -> **VALID JSON**
3. **TTY / Color Detection**:
   - When output is piped (e.g. `dojo find go | head -n 3 | cat -v`), Dojo detects non-TTY and strips all ANSI color escape sequences automatically.

---

## 4. Key Findings & Recommended Fixes

### Finding 1: Copy-Paste Branding in `dojo run` (Medium Friction)
- **Location**: `internal/commands/run.go:35` and `internal/commands/run.go:65`
- **Receipt**:
  ```bash
  $ dojo run
  USAGE:
    kungfu run <skill> <script> [args...]
  ```
- **Fix**: Change `kungfu run` to `dojo run`.

### Finding 2: Root Help Omits `push` for `profile` (Low Friction)
- **Location**: `internal/commands/app.go:998`
- **Receipt**:
  ```
  profile    <export|apply|sync> [source] [-f <file>] [-g] [--target <harnesses>]
  ```
  While `dojo profile --help` correctly lists `<export|apply|sync|push>`, the root overview table omits `push`.
- **Fix**: Update the summary in `app.go` to `<export|apply|sync|push>`.

### Finding 3: Awkward Error for `dojo -v` Bare Flag (Low Friction)
- **Location**: `internal/commands/app.go`
- **Receipt**:
  Running `dojo -v` produces `error: unknown command "". Run 'dojo --help' for available commands and usage.`
- **Fix**: If `-v` is provided without a subcommand or additional arguments, print the version (or a hint that `-V` is `--version` and `-v` is `--verbose`).

---

## 5. Resolution & Implementation Status

All findings have been resolved in the codebase:
- [x] **Finding 1 (Branding)**: Fixed in `internal/commands/run.go`, `internal/commands/load.go`, `internal/commands/update.go`, `internal/commands/telemetry.go`, `internal/commands/status.go`, and `internal/runner/runner.go`. Companion script regex now supports both `dojo run` and legacy `kungfu run`.
- [x] **Finding 2 (Profile Help)**: Updated root `PrintGlobalHelp` in `internal/commands/app.go` to explicitly list `<export|apply|sync|push>`.
- [x] **Finding 3 (Bare `-v` Version Handling)**: Updated `internal/commands/app.go` command dispatch to print `dojo version <version>` and exit 0 when bare `-v` or `--verbose` is provided without a command, eliminating the `unknown command ""` error.
- [x] **Test Coverage**: Added test cases in `main_test.go` and `internal/commands/load_run_test.go` verifying bare `-v`, `--verbose`, `dojo run` usage branding, and root help text. 100% test pass rate across all packages.

