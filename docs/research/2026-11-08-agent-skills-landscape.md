# Agent Skills tooling landscape, 8 November 2026

Scope: competitive map for Dojo v1.3.0 plus evidence-backed recommendations.
All URLs below were accessed on 2026-11-08. Claims are labelled as
observed fact, inference, or recommendation. Anything I could not verify is
listed in the last section instead of being asserted.

Repository metrics come from the GitHub REST API at
`https://api.github.com/repos/<owner>/<repo>`, read on 2026-11-08.
npm download counts come from `https://api.npmjs.org/downloads/point/...`,
read on 2026-11-08.

## 1. Executive summary

- The format is settled and not owned by Dojo. The Agent Skills spec lives at
  agentskills.io, is published by the `agentskills` org, and the three-tier
  progressive disclosure model is what every harness implements. Dojo wins
  nothing by re-stating the format.
  (https://agentskills.io/specification, 2026-11-08)
- Distribution, not features, is the binding constraint. The `skills` CLI from
  Vercel Labs has 18,515,925 npm downloads in the last month and its
  leaderboard is the de facto discovery surface. Dojo has 0 stars, 0 GitHub
  topics, no homepage, and does not appear in skills.sh search.
  (https://www.npmjs.com/package/skills, https://skills.sh, 2026-11-08)
- The one documented, reproducible pain Dojo already solves is context cost.
  Anthropic's issue tracker has an open report that a single skill injected
  ~156k tokens in one tool call. The spec says the whole body loads on
  activation. No competitor documents heading-level loading.
  (https://github.com/anthropics/skills/issues/1487, 2026-11-08)
- Dojo's cross-harness sync is ahead of every CLI rival, but two of its target
  paths are drifting: Cursor now reads `.agents/skills/` natively, and
  Windsurf has become Devin with `.windsurf/*` kept only for compatibility.
  (https://cursor.com/docs/skills,
  https://docs.devin.ai/desktop/devin-local/customizations, 2026-11-08)
- The cheapest high-leverage move is to publish the Dojo skill where agents
  already look: a skills.sh installable repo, a Claude Code plugin
  `marketplace.json`, and an `llms.txt` with a real origin serving
  `/.well-known/agent-skills/index.json`. Dojo currently does none of these.

## 2. Landscape table

Cell text is deliberately terse. Detail and links are in section 3.

| Tool         | Does        | Install  | Licence | Signal    | Gap       |
|--------------|-------------|----------|---------|-----------|-----------|
| skills CLI   | find/add    | npx      | MIT     | 18.5M/mo  | no paging |
| anthropic    | ref skills  | /plugin  | none    | 189k      | Claude    |
| superpowers  | method      | plugin   | MIT     | 314k      | opinion   |
| kungfu       | catalog CLI | go get   | Apache  | 7 stars   | no MCP    |
| agentskills  | spec        | web      | Apache  | 27k       | no tool   |
| Claude Code  | harness     | bundled  | prop.   | 155k      | no sect.  |
| Cursor       | harness     | IDE/CLI  | prop.   | n/a       | no sect.  |
| Devin        | harness     | IDE/CLI  | prop.   | n/a       | paths     |
| Antigravity  | harness     | IDE/CLI  | prop.   | n/a       | no pkg    |
| Gemini CLI   | harness+ext | npm      | Apache  | n/a       | no paging |
| Smithery     | MCP reg.    | CLI      | n/a     | n/a       | MCP       |
| Glama        | MCP index   | web      | n/a     | 117k srv  | MCP       |
| MCP registry | MCP index   | CLI      | open    | preview   | manifest  |
| mcp.so       | MCP dir.    | web      | n/a     | n/a       | MCP       |
| PulseMCP     | MCP dir.    | web      | n/a     | n/a       | MCP       |
| Dojo         | sections    | curl/go  | Apache  | 0 stars   | no reach  |

Legend: anthropic is `anthropics/skills`; superpowers is `obra/superpowers`;
agentskills is agentskills.io; prop. is proprietary; n/a means not applicable
or not published; no sect. means no section-level loading; paths means harness
paths are in transition; n/a in Signal means the project publishes no star
count I could read, either because it is a website or a commercial product.

## 3. Per-tool notes

### skills CLI (Vercel Labs) and skills.sh

Observed: `skills@1.7.6`, MIT, repository `vercel-labs/skills`, 38,580 stars,
810 open issues, last push 2026-11-06. Last-month npm downloads 18,515,925;
last week 5,144,259. (https://www.npmjs.com/package/skills,
https://api.github.com/repos/vercel-labs/skills, 2026-11-08)

Commands: `add`, `use`, `find`, `list`, `check`, `update`, `remove`, `init`,
plus experimental `sync` and `install` with a lock file. Installs to
`.agents/skills/` and links or copies into agent directories, 79 agents
claimed. `skills use` prints a generated prompt for one skill without
installing. Sources: https://github.com/vercel-labs/skills and
https://github.com/vercel-labs/skills/blob/main/docs/commands.md
(2026-11-08).

skills.sh ranks by anonymous install telemetry and states it runs "routine
security audits" but "cannot guarantee the quality or security of every skill
listed". (https://skills.sh/docs, 2026-11-08)

Context handling observed: whole skill. No heading-level paging is documented
in the CLI reference.

Gap left open: no section-level loading, no dependency graph, no reverse
reconciliation from harness directories back to a canonical store, no local
secret scan before you push a skill repo.

### Anthropic: anthropics/skills and Claude Code

Observed: `anthropics/skills` has 189,310 stars, 22,312 forks, 1,495 open
issues, no licence field on the repo, last push 2026-11-06. The README says
many skills are Apache-2.0 and points the standard at agentskills.io.
(https://github.com/anthropics/skills, 2026-11-08)

Install path is a Claude Code plugin marketplace:
`/plugin marketplace add anthropics/skills`, then `/plugin install
document-skills@anthropic-agent-skills`.
(https://github.com/anthropics/skills, 2026-11-08)

Context handling observed: "a skill's body loads only when it's used" and
"once a skill loads, its content stays in context across turns". Description
plus `when_to_use` is truncated at 1,536 characters. No section-level loading.
(https://code.claude.com/docs/en/skills, 2026-11-08)

Distribution: your own `.claude-plugin/marketplace.json` needs no review.
Anthropic's directory needs a paid claude.ai plan and a review. The official
marketplace `claude-plugins-official` does not take portal submissions.
(https://code.claude.com/docs/en/plugins/publish, 2026-11-08)

Gap left open: Anthropic's tooling is Claude-first. There is no CLI that pages,
audits, or reconciles across vendors.

### obra/superpowers

Observed: 314,446 stars, 27,975 forks, MIT, 343 open issues, last push
2026-11-03. It is a skills framework plus a curated plugin marketplace
(`obra/superpowers-marketplace`, 1,325 stars).
(https://api.github.com/repos/obra/superpowers, 2026-11-08)

Install paths it documents: Claude Code plugin search, Codex plugin
marketplace, Cursor `/add-plugin superpowers`, `npx skills add
obra/superpowers`, and a list of a dozen other harnesses.
(https://github.com/obra/superpowers, 2026-11-08)

Context handling observed: no paging. It sells discipline, not token economy.

Gap left open: it is one opinionated methodology. It does not manage third
party skills, audit them, or move them between harnesses.

### kungfu (danicat/kungfu)

Observed: 7 stars, 1 fork, Apache-2.0, 0 open issues, last push 2026-11-07.
Go CLI, `go install`, implements the Cloudflare RFC v0.2.0 with SHA-256
checks, pages output at ~6,000 bytes into `~/.cache/kungfu/skills/`, and runs
bundled scripts. (https://github.com/danicat/kungfu, 2026-11-08)

This is Dojo's nearest ancestor and `CREDITS.md` in the Dojo repo says so.

Gap left open: byte paging, not heading paging. No MCP server, no harness
sync, no personal registry, no security scan. A README grep for `mcp`,
`section`, `harness`, `claude`, `cursor` returns no matches
(curl of the raw README, 2026-11-08).

### agentskills.io (the standard)

Observed: `agentskills/agentskills` has 27,065 stars, Apache-2.0, last push
2026-11-04. The site publishes an `llms.txt` index of its own docs and a
Client Showcase page for products that support the format.
(https://agentskills.io/llms.txt, https://agentskills.io/clients.md,
2026-11-08)

Spec facts that matter to positioning: the body has "no format restrictions"
but "the agent will load this entire file once it's decided to activate a
skill", with a recommendation to split long content into referenced files and
keep `SKILL.md` under 500 lines. Progressive disclosure is three tiers:
metadata ~100 tokens, body under 5,000 tokens, resources on demand.
(https://agentskills.io/specification, 2026-11-08)

The client guide names `.agents/skills/` as "a widely-adopted convention",
says project skills override user skills, warns about untrusted repositories
injecting instructions, and recommends lenient parsing for malformed YAML
such as unquoted colons in descriptions. Source: the client implementation
guide linked from https://agentskills.io/llms.txt (2026-11-08).

Gap left open: the standard defines files, not delivery. It says nothing about
paging inside a body, syncing between harnesses, or verifying a publisher.

### Claude Code

Observed: 154,972 stars, 16,878 open issues. Skills live in
`.claude/skills/<name>/SKILL.md` at enterprise, personal, project, nested and
plugin scope, plus `.claude/commands/*.md` as the older format.
Symlinked skill folders are followed.
(https://api.github.com/repos/anthropics/claude-code,
https://code.claude.com/docs/en/skills, 2026-11-08)

Gap left open: no heading-level loading, and skill content persists in context
for the rest of the session, so a long body is a recurring cost.

### Cursor

Observed: Cursor ships Skills natively and auto-loads from `.agents/skills/`,
`.cursor/skills/`, `~/.agents/skills/`, `~/.cursor/skills/`, and for
compatibility `.claude/skills/`, `.codex/skills/` and their home equivalents.
Nested skill directories are scoped to their subtree. `paths` frontmatter
scopes a skill to file globs. `/migrate-to-skills` converts eligible
`.mdc` rules and slash commands into skills.
(https://cursor.com/docs/skills, 2026-11-08)

Rules are still documented as `.cursor/rules/*.mdc`, and a plain `.md` in that
folder is ignored. Importing skills from a GitHub repo requires packaging them
in a plugin with `.cursor-plugin/marketplace.json`.
(https://cursor.com/docs/context/rules, https://cursor.com/docs/skills,
2026-11-08)

Relevant to Dojo: Dojo projects Cursor skills as `.cursor/rules/<name>.mdc`.
If the same skill also exists in `.agents/skills/`, Cursor can see it twice,
once as a skill and once as a rule. That is an inference from the two docs, not
something I tested.

Gap left open: Cursor does not page a skill body, and it does not manage
skills for other harnesses.

### Devin (formerly Windsurf)

Observed: `https://docs.windsurf.com/windsurf/cascade/memories` resolves to
`https://docs.devin.ai/desktop/devin-local/customizations`. Skills load from
`.devin/skills/`, and `.agents/skills/` and legacy `.windsurf/skills/` are also
read. Global is `~/.config/devin/skills/` plus `~/.agents/skills/`. Rules live
in `.devin/rules/*.md`; legacy `.windsurf/rules/*.md` and `.windsurfrules` are
"imported by default".
(https://docs.devin.ai/desktop/devin-local/customizations, 2026-11-08)

Relevant to Dojo: the `windsurf` target writes `.windsurf/rules/<name>.md`,
which is now a compatibility import rather than the canonical path.

Gap left open: same as Cursor, no paging, no cross-vendor management.

### Google Antigravity and Gemini CLI

Antigravity observed: workspace skills at `<root>/.agents/skills/`, global at
`~/.gemini/config/skills/`, with legacy `~/.gemini/antigravity/skills/` still
supported. The CLI additionally reads `~/.gemini/antigravity-cli/skills/` and
`~/.gemini/skills/`. (https://antigravity.google/docs/skills, 2026-11-08)

Gemini CLI observed: user skills in `~/.gemini/skills/` or `~/.agents/skills/`,
workspace skills in `.gemini/skills/` or `.agents/skills/`, and
`.agents/skills/` takes precedence within a tier. It has `gemini skills
install|link|uninstall` and an extensions gallery that bundles skills.
(https://geminicli.com/docs/cli/skills/,
https://geminicli.com/docs/extensions/, 2026-11-08)

Relevant to Dojo: the `gemini` target writing `.gemini/skills/` is valid.
The `antigravity` target writing `.antigravity/skills/` has no support in
either doc I read. Whether Antigravity reads it is unverified.

Gap left open: both are harnesses, not managers. Neither pages a body.

### Registries: Smithery, Glama, mcp.so, PulseMCP, official MCP registry

Observed: Smithery publishes MCP servers by URL or `.mcpb` bundle via
`smithery mcp publish`, uses namespaces, and also runs a skills registry.
(https://raw.githubusercontent.com/smithery-ai/cli/main/README.md, 2026-11-08)

Glama auto-indexes open-source MCP servers from GitHub and claims 117,443
servers plus quality grades and an `isVerified` flag.
(https://glama.ai/mcp/servers, 2026-11-08)

The official registry publishes through the `mcp-publisher` CLI, not pull
requests, and validates namespace ownership by GitHub OAuth, GitHub OIDC or
DNS. It is still in preview. (https://github.com/modelcontextprotocol/registry,
2026-11-08)

mcp.so exposes a submit page (HTTP 200 on `/submit`). PulseMCP returned 403 to
my fetch, so its listing rules are unverified.
(https://mcp.so/submit, https://www.pulsemcp.com, 2026-11-08)

Relevant gap: all of these index MCP servers. Dojo's differentiator is a CLI
plus skills, which none of them model.

## 4. What Dojo already does better

Evidence for Dojo comes from `README.md` and `BACKLOG.md` in this repo.

Section-level paging. Dojo parses Markdown headings, ignores `#` inside fenced
code, and exposes `dojo toc` plus `dojo load <skill>#<section>` with slugs,
line ranges and byte sizes (`internal/catalog/sections.go`, BACKLOG v1.0.0).
None of the rival docs I read describe heading-level loading: not the
`skills` CLI, Claude Code, Cursor, Devin, Antigravity, Gemini CLI, or the
spec. kungfu pages by fixed 6,000-byte chunks, which can cut a section in
half. The closest competitor capability is Cursor's `paths`
scoping, which filters which skills are visible, not which part of one skill
loads. (https://cursor.com/docs/skills, 2026-11-08)

Token-cost urgency is documented by Anthropic's own users, not by Dojo
marketing: issue 1487 reports a single skill injecting ~156k tokens in one
call, and issue 1220 asks for multi-file preload control.
(https://github.com/anthropics/skills/issues/1487,
https://github.com/anthropics/skills/issues/1220, 2026-11-08)

Bi-directional reconciliation. `dojo sync` imports edits made directly in
`.claude/skills`, `.cursor/rules/*.mdc`, `.windsurf/rules`, `.gemini/skills`,
`.opencode/skills` and `.sila/skills` back into `.agents/skills`, including
reverse transpiling Cursor `.mdc` frontmatter. The `skills` CLI links outward
from `.agents/skills`; it does not document reverse import. Cursor's
`/migrate-to-skills` is a one-time conversion inside Cursor only.
(https://raw.githubusercontent.com/vercel-labs/skills/main/docs/commands.md,
https://cursor.com/docs/skills, 2026-11-08)

Personal registry across machines. `dojo profile link|sync|pull|push|apply`
keeps a `skills.json` in a Git repo and moves a skill library between a laptop
and a cloud VM. The `skills` CLI has `skills-lock.json` for reproducibility and
packs for sharing, but no documented "my own repo is my registry, two-way
sync" loop. Cursor's sync uploads only `~/.cursor/skills/` for Cloud Agents.
(https://raw.githubusercontent.com/vercel-labs/skills/main/docs/commands.md,
https://cursor.com/docs/skills, 2026-11-08)

Pre-push security gate. `dojo profile push` and `sync` run `ScanDirSecurity`
and block `ghp_`, `github_pat_`, `sk-`, `AIza`, `AKIA`, private keys, and
prompt-injection patterns (BACKLOG v1.3.0, G-03 and G-07). skills.sh audits
skills after they are published and disclaims guarantee. No competitor CLI I
read documents a local pre-push secret gate.

Digest-verified federated catalogs with dependency ordering. Dojo verifies
SHA-256 per the Cloudflare RFC and resolves `dependencies` topologically with
cycle detection (BACKLOG v1.0.0). kungfu verifies digests but has no dependency
graph. The `skills` CLI resolves packs, not per-skill declared dependencies.

Single static binary, no Node runtime. `CGO_ENABLED=0`, six OS/arch targets,
one-line installer and `dojo upgrade` (README, BACKLOG G-01). The `skills` CLI
needs Node.js 22.20 or later.

## 5. Where Dojo is behind

Distribution. Observed: 0 stars, 0 topics, no homepage, no GitHub Pages. All
12 releases landed in 48 hours, from 2026-11-06 21:11 to 2026-11-07 19:58
UTC.
(https://api.github.com/repos/jadmadi/dojo,
https://api.github.com/repos/jadmadi/dojo/releases, 2026-11-08)

Absent from the main discovery surface. A skills.sh search for `dojo` returns
only `dojoengine` game-engine skills. A search for the author returns six
skills under `jadmadi/skills` with 1 to 3 installs each, and the Dojo skill is
not among them. So the author already publishes to skills.sh, but not the
product's own skill. (https://skills.sh/api/search?q=dojo,
https://skills.sh/api/search?q=jadmadi, 2026-11-08)

No package manager presence. No npm package, no Homebrew formula, no
`.mcpb` bundle, no `server.json`. Compare `skills@1.7.6` at 18.5M monthly
downloads.

No agent-readable docs surface. No `llms.txt`, no docs site, no `agents.md`,
no OpenAPI, nothing the standard's own site does not already model better.
agentskills.io ships `llms.txt`; Claude Code ships
`https://code.claude.com/docs/llms.txt`; Gemini CLI pages offer "Copy as
Markdown". (https://agentskills.io/llms.txt,
https://code.claude.com/docs/llms.txt, 2026-11-08)

RFC self-hosting gap. Dojo's default catalog URL is a raw.githubusercontent
path under `main/.well-known/agent-skills/index.json` in its own repo (see
`DOJO_URL` in the README). The RFC requires publishers to serve the index at
`/.well-known/agent-skills/index.json` on an origin root, and resolves
path-absolute URLs against that origin. A raw file URL is not an origin
root, so an RFC-compliant crawler cannot discover Dojo by hostname.
(https://github.com/cloudflare/agent-skills-discovery-rfc,
https://github.com/jadmadi/dojo, 2026-11-08)

Naming collision. "Dojo" is a common word and already belongs to a game engine
ecosystem with skills on the same registry. Every search surface will bury
Dojo under `dojoengine`. This is an inference from the search results, not a
measured ranking.

Trust signals. skills.sh runs audits and Glama assigns quality grades. Dojo's
`doctor --security` is a self-attested local gate with no third-party review,
no signed releases, no SBOM and no reproducible build claim that I could find
in the repo.

Integration drift. Cursor reads `.agents/skills` natively, so `.mdc`
projection is now redundant and possibly duplicative. Windsurf paths are
legacy Devin imports. The `antigravity` target writes `.antigravity/skills`,
which neither Google doc mentions.

No marketplace listing anywhere. No `.claude-plugin/marketplace.json`, no
`.cursor-plugin/marketplace.json`, no Gemini extension, no Smithery listing,
no MCP registry entry.

## 6. Recommendations

Ranked by impact against effort. Each states the evidence, the expected
effect, and the cost.

1. Publish the Dojo skill to skills.sh, and keep publishing each release.
   Evidence: `skills` CLI at 18.5M monthly downloads, skills.sh ranks by
   install telemetry, and the author's other skills already appear there.
   Effect: `npx skills add jadmadi/dojo` becomes a real install path, and the
   skill shows up in the surface agents and developers already search.
   Cost: under a day. Restructure the existing `skills/dojo` folder to the
   layout `skills add` expects, then ask for a re-index.
   (https://skills.sh/docs,
   https://github.com/vercel-labs/skills/blob/main/docs/commands.md,
   2026-11-08)

2. Fix the stale and duplicated harness paths before adding more targets.
   Evidence: Cursor auto-loads `.agents/skills`; Devin treats `.windsurf/*` as
   legacy; Antigravity documents `.agents/skills` and `~/.gemini/config/skills`
   only. Effect: removes double-loading risk and stops shipping targets that
   point at compatibility paths. Cost: two to three days in
   `internal/harness`. Make `.mdc` projection opt-in, rename the windsurf
   target to devin with a windsurf alias, and verify the antigravity path by
   running the real app.
   (https://cursor.com/docs/skills,
   https://docs.devin.ai/desktop/devin-local/customizations,
   https://antigravity.google/docs/skills, 2026-11-08)

3. Ship a Claude Code plugin marketplace and a Cursor plugin manifest.
   Evidence: Anthropic's own docs say a marketplace is just a
   `.claude-plugin/marketplace.json` in a repo with no review, and Cursor needs
   `.cursor-plugin/marketplace.json` to import skills from GitHub.
   Effect: one-command install for the two largest harness user bases, and a
   path into Anthropic's reviewed directory later. Cost: one to two days plus
   a submission.
   (https://code.claude.com/docs/en/plugins/publish,
   https://cursor.com/docs/skills, 2026-11-08)

4. Publish a measured token benchmark, not a percentage claim.
   Evidence: the README claims 70 to 90 percent context savings with no
   method. Anthropic's issue 1487 gives a real, citable failure mode. Dojo has
   `toc` byte counts and can measure tokens per section.
   Effect: converts the main differentiator from assertion into evidence, and
   gives you something concrete to post in issue 1487 and 1220. Cost: two days,
   mostly measurement and a reproducible script.
   (https://github.com/anthropics/skills/issues/1487, 2026-11-08)

5. Stand up a real origin with `llms.txt` and a root `.well-known` index.
   Evidence: the RFC resolves index URLs against an origin root, and Dojo's
   default is a raw GitHub file. Every comparable project ships an `llms.txt`.
   Effect: RFC-compliant discovery works, agents can read the docs in one
   fetch, and Dojo gets a URL to put in the README and the repo homepage.
   Cost: one day for a static site, plus DNS. Serve
   `/.well-known/agent-skills/index.json` at the root and keep the current
   file as a mirror.
   (https://github.com/cloudflare/agent-skills-discovery-rfc,
   https://agentskills.io/llms.txt, 2026-11-08)

6. Add an interop import command so switching costs are zero in both
   directions. Evidence: the ecosystem standard is `SKILL.md`, but real users
   hold `.cursor/rules/*.mdc`, `.devin/rules/*.md`, `.claude/commands/*.md`,
   `skills-lock.json` and `.skills.json`. Dojo already transpiles `.mdc` one
   way. Effect: "bring your existing rules and skills" becomes a one-liner,
   and Dojo becomes the migration tool rather than a new silo. Cost: three to
   five days.
   (https://github.com/vercel-labs/skills/blob/main/docs/commands.md,
   https://cursor.com/docs/context/rules, 2026-11-08)

7. Buy trust with artifacts, then ask for listings. Evidence: skills.sh runs
   audits, Glama grades quality, and the official MCP registry verifies
   namespace ownership. Effect: a security-conscious team can justify trying
   Dojo. Cost: low for SBOM and signed releases, medium for a third-party
   review. Publish SBOMs, sign release archives, add a SECURITY.md with the
   scan rules and their limits, then submit to awesome lists and the
   agentskills.io client showcase.
   (https://skills.sh/docs, https://glama.ai/mcp/servers,
   https://github.com/modelcontextprotocol/registry,
   https://agentskills.io/clients.md, 2026-11-08)

## 7. Open questions and unverified claims

- Whether the official MCP registry accepts a Go binary at all. The docs list
  npm, PyPI, OCI and remote URLs in the examples I read, and I did not
  confirm a Go or generic-binary package type. Dojo may only qualify
  through a `.mcpb` bundle or a hosted endpoint. Unverified.
- Whether Smithery's `.mcpb` bundle format supports an arbitrary static binary.
  Unverified.
- Whether skills.sh requires any submission or review. The docs describe
  telemetry-based ranking and no submission step, but I did not find an
  explicit statement that indexing is automatic. Unverified.
- Whether Antigravity reads `.antigravity/skills` or `.gemini/skills`. Neither
  Google doc mentions them, but absence in docs is not proof of absence in the
  binary. Needs a runtime test.
- Whether projecting a skill to both `.agents/skills` and `.cursor/rules`
  causes Cursor to load it twice. Inference from the two docs, not tested.
- Cursor's rule directory at `https://cursor.directory` returned HTTP 429 on
  every attempt, so its listing rules and traffic are unverified.
- PulseMCP returned HTTP 403, so its listing rules are unverified.
- Dojo's real user count is unknown. Stars are 0 and no download or telemetry
  figures are published.
- The README's "70 to 90 percent" token saving has no published measurement in
  the repo that I could find.
- Anthropic's directory review criteria and turnaround time are documented as
  a process, not as published acceptance rules, so I make no claim about Dojo's
  odds of acceptance.
