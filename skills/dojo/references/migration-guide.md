# Agent Playbook: Moving Skills, Rules, and Commands Between Local and Cloud (`agy-vm`)

Use this playbook whenever the user asks an agent to move skills, rules, or slash commands between a local computer and a cloud VM (`agy-vm` / `github:<owner>/<repo>`).

## Scenario A: "I need `X` and `Y` in my `agy-vm`" (Local -> Cloud)

### Fast Path (Single Command)

Run `dojo profile push` targeting the user's GitHub profile repository (e.g., `github:jadmadi/agy-vm`):

```bash
dojo profile push github:jadmadi/agy-vm <x> <y>
```

`dojo profile push` automatically:
1. Searches both the current workspace and `~/` home directory across:
   - Skill directories: `.agents/skills/`, `.gemini/skills/`, `.antigravity/skills/`, `.claude/skills/`, `.opencode/skills/`, `.sila/skills/`
   - Cursor & Windsurf rules: `.cursor/rules/<name>.mdc`, `.windsurf/rules/<name>.md`
   - Agentic slash commands & workflows: `.claude/commands/<name>.md`, `.gemini/commands/<name>.md`, `.gemini/antigravity/global_workflows/<name>.md`, `.agents/workflows/<name>.md`, `.windsurf/workflows/<name>.md`, `.cursor/commands/<name>.md`
2. Converts single-file rules (`.mdc`) or slash commands (`.md`) into compliant `SKILL.md` packages with YAML frontmatter (`name` and `description`), while copying full skill directories (`SKILL.md`, `scripts/`, `references/`, `assets/`) intact.
3. Shallow-clones the target repository (supporting private repos via `gh` CLI or `git`), writes `skills/<name>/`, upserts the entries into `skills.json` without removing existing skills, commits, and pushes.

### Custom Command with External Scripts

If a local slash command (e.g., `~/.claude/commands/release.md`) invokes a local script outside its directory (e.g., `~/bin/release-helper.sh`), bundle them into a self-contained skill before pushing:

```bash
# 1. Scaffold or create the canonical skill directory
dojo init release --dir ~/.agents/skills --description "Release automation workflow"

# 2. Copy the command instructions into SKILL.md (preserving YAML frontmatter) and place scripts in scripts/
cp ~/bin/release-helper.sh ~/.agents/skills/release/scripts/release-helper.sh
chmod +x ~/.agents/skills/release/scripts/release-helper.sh

# 3. Validate frontmatter, token budget, and security
dojo doctor --skills --security -g

# 4. Push the complete skill package to agy-vm
dojo profile push github:jadmadi/agy-vm release -g
```

## Scenario B: Pulling Skills into `agy-vm` or Another Workstation (Cloud -> Local)

On `agy-vm` or a second laptop:

```bash
# Apply the entire traveling profile (private repos authenticate via GITHUB_TOKEN, GH_TOKEN, or `gh` CLI)
dojo profile apply github:jadmadi/agy-vm -g

# Or install a single skill directly from the private/public profile repository
dojo learn github:jadmadi/agy-vm/skills/<name> -g --target all
```
