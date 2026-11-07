// Copyright 2026 The Dojo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/harness"
)

var allSubcommands = []string{
	"find",
	"toc",
	"load",
	"run",
	"learn",
	"unlearn",
	"list",
	"show",
	"status",
	"update",
	"sync",
	"sila",
	"profile",
	"init",
	"doctor",
	"catalog",
	"upgrade",
	"mcp",
	"completion",
	"telemetry",
	"version",
	"help",
}

// RunCompletion generates native shell completion scripts for bash, zsh, fish, and powershell.
func (a *App) RunCompletion(args []string) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(a.Stderr, "error: completion requires a shell name (bash, zsh, fish, or powershell)")
		return ExitUsage
	}
	shell := strings.ToLower(strings.TrimSpace(args[0]))
	switch shell {
	case "bash":
		_, _ = fmt.Fprint(a.Stdout, bashCompletionScript)
		return ExitSuccess
	case "zsh":
		_, _ = fmt.Fprint(a.Stdout, zshCompletionScript)
		return ExitSuccess
	case "fish":
		_, _ = fmt.Fprint(a.Stdout, fishCompletionScript)
		return ExitSuccess
	case "powershell", "pwsh":
		_, _ = fmt.Fprint(a.Stdout, powershellCompletionScript)
		return ExitSuccess
	default:
		_, _ = fmt.Fprintf(a.Stderr, "error: unsupported shell %q (valid: bash, zsh, fish, powershell)\n", shell)
		return ExitUsage
	}
}

// RunComplete executes dynamic tab-completion lookups for shell completion scripts (`dojo __complete ...`).
func (a *App) RunComplete(args []string) int {
	if len(args) == 0 {
		for _, cmd := range allSubcommands {
			_, _ = fmt.Fprintln(a.Stdout, cmd)
		}
		return ExitSuccess
	}

	sub := args[0]
	cur := ""
	prev := ""
	if len(args) >= 2 {
		cur = args[len(args)-1]
		prev = args[len(args)-2]
	} else {
		// Completing the subcommand itself
		for _, cmd := range allSubcommands {
			if strings.HasPrefix(cmd, sub) {
				_, _ = fmt.Fprintln(a.Stdout, cmd)
			}
		}
		return ExitSuccess
	}

	if prev == "--target" || prev == "-t" {
		for _, t := range []string{"all", "antigravity", "claude", "cursor", "windsurf", "opencode", "sila"} {
			if strings.HasPrefix(t, cur) {
				_, _ = fmt.Fprintln(a.Stdout, t)
			}
		}
		return ExitSuccess
	}

	switch sub {
	case "completion":
		for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
			if strings.HasPrefix(sh, cur) {
				_, _ = fmt.Fprintln(a.Stdout, sh)
			}
		}
		return ExitSuccess
	case "profile":
		if len(args) == 2 {
			for _, p := range []string{"link", "unlink", "export", "apply", "pull", "sync", "push"} {
				if strings.HasPrefix(p, cur) {
					_, _ = fmt.Fprintln(a.Stdout, p)
				}
			}
			return ExitSuccess
		}
	case "catalog":
		if len(args) == 2 {
			for _, c := range []string{"list", "add", "remove", "update"} {
				if strings.HasPrefix(c, cur) {
					_, _ = fmt.Fprintln(a.Stdout, c)
				}
			}
			return ExitSuccess
		}
	case "load", "toc", "show", "run", "learn", "unlearn", "sync", "sila":
		if strings.HasPrefix(cur, "-") {
			for _, fl := range flagsForSubcommand(sub) {
				if strings.HasPrefix(fl, cur) {
					_, _ = fmt.Fprintln(a.Stdout, fl)
				}
			}
			return ExitSuccess
		}
		// Check if completing a #section slug (e.g. "dojo load godoctor#qu")
		if hashIdx := strings.IndexByte(cur, '#'); hashIdx > 0 && (sub == "load" || sub == "toc") {
			skillName := cur[:hashIdx]
			secPrefix := strings.ToLower(cur[hashIdx+1:])
			for _, slug := range a.completeSkillSections(skillName) {
				if strings.HasPrefix(slug, secPrefix) {
					_, _ = fmt.Fprintf(a.Stdout, "%s#%s\n", skillName, slug)
				}
			}
			return ExitSuccess
		}
		for _, name := range a.completeSkillNames() {
			if strings.HasPrefix(name, cur) {
				_, _ = fmt.Fprintln(a.Stdout, name)
			}
		}
		return ExitSuccess
	}

	return ExitSuccess
}

func flagsForSubcommand(sub string) []string {
	switch sub {
	case "load":
		return []string{"--section", "--with-deps", "--page", "--page-size", "--full", "--local", "--update", "--force", "--json", "--raw"}
	case "toc":
		return []string{"--local", "--update", "--force", "--json", "--raw"}
	case "learn":
		return []string{"--global", "--with-deps", "--target", "--force", "--load", "--json", "--raw"}
	case "sync", "sila":
		return []string{"--target", "--global", "--one-way", "--json", "--raw"}
	default:
		return []string{"--json", "--raw"}
	}
}

func (a *App) completeSkillNames() []string {
	seen := make(map[string]bool)
	if a.Store != nil {
		if items, err := a.Store.List(""); err == nil {
			for _, item := range items {
				if item.Name != "" {
					seen[item.Name] = true
				}
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		for name := range harness.DiscoverInstalledSkillDirs(wd) {
			seen[name] = true
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		for name := range harness.DiscoverInstalledSkillDirs(home) {
			seen[name] = true
		}
	}
	var out []string
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (a *App) completeSkillSections(skillName string) []string {
	var raw string
	if a.Store != nil {
		if item, err := a.Store.Get(skillName); err == nil && item != nil {
			if content, lErr := a.loadItemContentWithOpts(item, true, false, false); lErr == nil {
				raw = content
			} else if content, lErr2 := a.loadItemContent(item); lErr2 == nil {
				raw = content
			}
		}
	}
	if raw == "" {
		if wd, err := os.Getwd(); err == nil {
			if data, rErr := os.ReadFile(filepath.Join(wd, ".agents", "skills", skillName, "SKILL.md")); rErr == nil {
				raw = string(data)
			} else if data, rErr2 := os.ReadFile(filepath.Join(wd, "skills", skillName, "SKILL.md")); rErr2 == nil {
				raw = string(data)
			}
		}
	}
	if raw == "" {
		return nil
	}
	sections := catalog.ParseSections(raw)
	var slugs []string
	for _, s := range sections {
		slugs = append(slugs, s.Slug)
	}
	return slugs
}

const bashCompletionScript = `# bash completion for dojo
_dojo_completions() {
    local cur prev words cword
    if declare -F _init_completion >/dev/null 2>&1; then
        _init_completion -n : || return
    else
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    fi

    local completions
    completions=$(dojo __complete "${words[@]:1}" 2>/dev/null)
    COMPREPLY=( $(compgen -W "${completions}" -- "${cur}") )
}
complete -F _dojo_completions dojo
`

const zshCompletionScript = `#compdef dojo
# zsh completion for dojo
_dojo() {
    local -a completions
    completions=("${(@f)$(dojo __complete "${words[@]:1}" 2>/dev/null)}")
    if [[ -n "$completions" ]]; then
        compadd -a completions
    fi
}
compdef _dojo dojo
`

const fishCompletionScript = `# fish completion for dojo
function __fish_dojo_complete
    set -l cmd (commandline -opc)
    set -l cur (commandline -ct)
    dojo __complete $cmd[2..-1] "$cur" 2>/dev/null
end
complete -c dojo -f -a "(__fish_dojo_complete)"
`

const powershellCompletionScript = `# PowerShell completion for dojo
Register-ArgumentCompleter -Native -CommandName dojo -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $elements = $commandAst.CommandElements | Select-Object -Skip 1 | ForEach-Object { $_.Extent.Text }
    if (-not $elements) { $elements = @("") }
    $completions = & dojo __complete @elements 2>$null
    foreach ($c in $completions) {
        [System.Management.Automation.CompletionResult]::new($c, $c, 'ParameterValue', $c)
    }
}
`
