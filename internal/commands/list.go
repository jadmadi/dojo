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
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/state"
)

func (a *App) RunList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var jsonFlag, rawFlag, globalFlag, workspaceFlag bool

	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw format")
	fs.BoolVar(&globalFlag, "global", false, "Filter skills installed in global scope (~/.agents/skills/)")
	fs.BoolVar(&globalFlag, "g", false, "Filter skills installed in global scope (shorthand)")
	fs.BoolVar(&workspaceFlag, "workspace", false, "Filter skills installed in workspace scope (.agents/skills/)")
	fs.BoolVar(&workspaceFlag, "w", false, "Filter skills installed in workspace scope (shorthand)")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return ExitUsage
	}

	mode := a.resolveMode(jsonFlag, rawFlag)

	items := a.Store.All()

	var filterQuery string
	if len(posArgs) > 0 {
		filterQuery = strings.ToLower(strings.TrimSpace(posArgs[0]))
	}

	if filterQuery != "" {
		var filtered []catalog.Item
		for _, it := range items {
			if strings.EqualFold(it.Category, filterQuery) ||
				strings.Contains(strings.ToLower(it.Name), filterQuery) ||
				strings.Contains(strings.ToLower(it.Description), filterQuery) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}

	if len(items) == 0 {
		if mode == OutputModeJSON {
			_, _ = fmt.Fprintln(a.Stdout, "[]")
			return ExitSuccess
		}
		if mode == OutputModeRaw {
			return ExitSuccess
		}
		if filterQuery != "" {
			_, _ = fmt.Fprintf(a.Stderr, "error: no skills found matching %q\n", filterQuery)
		} else {
			_, _ = fmt.Fprintln(a.Stderr, "error: no skills found in catalog")
		}
		return ExitNotFound
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	type SkillListing struct {
		Name         string   `json:"name"`
		Category     string   `json:"category"`
		Description  string   `json:"description"`
		Tags         []string `json:"tags,omitempty"`
		Version      string   `json:"version,omitempty"`
		Status       string   `json:"status"`
		AllowedTools string   `json:"allowed_tools,omitempty"`
	}

	var listings []SkillListing
	st := a.loadState()
	for _, it := range items {
		status := a.getSkillStatusWithState(&it, st)
		if globalFlag && status != state.ScopeGlobal {
			continue
		}
		if workspaceFlag && status != state.ScopeWorkspace {
			continue
		}
		listings = append(listings, SkillListing{
			Name:         it.Name,
			Category:     it.Category,
			Description:  it.Description,
			Tags:         it.Tags,
			Version:      it.Version,
			Status:       status,
			AllowedTools: it.AllowedTools,
		})
	}

	if len(listings) == 0 {
		if mode == OutputModeJSON {
			_, _ = fmt.Fprintln(a.Stdout, "[]")
			return ExitSuccess
		}
		if mode == OutputModeRaw {
			return ExitSuccess
		}
		if globalFlag {
			_, _ = fmt.Fprintln(a.Stderr, "No skills found installed in global scope (~/.agents/skills/).")
			return ExitSuccess
		}
		if workspaceFlag {
			_, _ = fmt.Fprintln(a.Stderr, "No skills found installed in workspace scope (.agents/skills/).")
			return ExitSuccess
		}
		if filterQuery != "" {
			_, _ = fmt.Fprintf(a.Stderr, "error: no skills found matching %q\n", filterQuery)
		} else {
			_, _ = fmt.Fprintln(a.Stderr, "error: no skills found in catalog")
		}
		return ExitNotFound
	}

	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(listings)
		return ExitSuccess
	case OutputModeRaw:
		for _, l := range listings {
			tagsStr := strings.Join(l.Tags, ",")
			_, _ = fmt.Fprintf(a.Stdout, "%s\t%s\t%s\t%s\t%s\n", l.Name, l.Category, l.Status, tagsStr, sanitizeTSVField(l.Description))
		}
		return ExitSuccess
	default: // OutputModeHuman
		if filterQuery != "" {
			_, _ = fmt.Fprintf(a.Stdout, "AVAILABLE SKILLS (matching %q, %d total):\n\n", filterQuery, len(listings))
		} else if globalFlag {
			_, _ = fmt.Fprintf(a.Stdout, "INSTALLED GLOBAL SKILLS (%d total):\n\n", len(listings))
		} else if workspaceFlag {
			_, _ = fmt.Fprintf(a.Stdout, "INSTALLED WORKSPACE SKILLS (%d total):\n\n", len(listings))
		} else {
			_, _ = fmt.Fprintf(a.Stdout, "AVAILABLE SKILLS (%d total):\n\n", len(listings))
		}
		_, _ = fmt.Fprintf(a.Stdout, "%-4s %-22s %-14s %-12s %s\n", "#", "NAME", "CATEGORY", "STATUS", "DESCRIPTION")
		_, _ = fmt.Fprintf(a.Stdout, "%-4s %-22s %-14s %-12s %s\n", "---", "----------------------", "--------------", "------------", "--------------------------------------------------")
		for idx, l := range listings {
			statusBadge := "[" + l.Status + "]"
			desc := truncateRunes(l.Description, 56)
			_, _ = fmt.Fprintf(a.Stdout, "%-4d %-22s %-14s %-12s %s\n", idx+1, l.Name, l.Category, statusBadge, desc)
		}

		_, _ = fmt.Fprintf(a.Stdout, "\nEXAMPLES:\n")
		_, _ = fmt.Fprintf(a.Stdout, "  Push skills to a remote profile repo or directory:\n")
		_, _ = fmt.Fprintf(a.Stdout, "    <target>                github:owner/repo or directory path\n")
		_, _ = fmt.Fprintf(a.Stdout, "                            Example: dojo profile push github:owner/repo 1,4,6 -g\n")
		_, _ = fmt.Fprintf(a.Stdout, "    <skills...>             List numbers (1,4,6) or skill names (retro,cli-reviewer)\n")
		_, _ = fmt.Fprintf(a.Stdout, "                            Example: dojo profile push github:owner/repo retro -g\n")
		_, _ = fmt.Fprintf(a.Stdout, "    --all                   Push all installed skills\n")
		_, _ = fmt.Fprintf(a.Stdout, "                            Example: dojo profile push github:owner/repo --all -g\n")
		_, _ = fmt.Fprintf(a.Stdout, "    -g, --global            Push from global store (~/.gemini/skills)\n")
		_, _ = fmt.Fprintf(a.Stdout, "                            Example: dojo profile push github:owner/repo 1,2 -g\n\n")

		_, _ = fmt.Fprintf(a.Stdout, "  Inspect skill metadata and table of contents:\n")
		_, _ = fmt.Fprintf(a.Stdout, "    <number>                Skill number from the list above\n")
		_, _ = fmt.Fprintf(a.Stdout, "                            Example: dojo show 1\n")
		_, _ = fmt.Fprintf(a.Stdout, "    <name>                  Skill name\n")
		_, _ = fmt.Fprintf(a.Stdout, "                            Example: dojo toc retro\n\n")

		_, _ = fmt.Fprintf(a.Stdout, "  Stream skill instructions into agent context:\n")
		_, _ = fmt.Fprintf(a.Stdout, "    <number>                Skill number\n")
		_, _ = fmt.Fprintf(a.Stdout, "                            Example: dojo load 1\n")
		_, _ = fmt.Fprintf(a.Stdout, "    <number#section>        Specific #section to minimize tokens\n")
		_, _ = fmt.Fprintf(a.Stdout, "                            Example: dojo load 1#architecture\n\n")

		_, _ = fmt.Fprintf(a.Stdout, "  Install / learn skills into local or global scope:\n")
		_, _ = fmt.Fprintf(a.Stdout, "    <number> [-g]           Install skill by list number\n")
		_, _ = fmt.Fprintf(a.Stdout, "                            Example: dojo learn 1 -g\n")
		_, _ = fmt.Fprintf(a.Stdout, "    <name> [-g]             Install skill by name\n")
		_, _ = fmt.Fprintf(a.Stdout, "                            Example: dojo learn retro -g\n")
		return ExitSuccess
	}
}
