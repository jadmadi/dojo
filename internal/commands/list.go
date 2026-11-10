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

// listOptions holds the parsed flags for `dojo list`.
type listOptions struct {
	json      bool
	raw       bool
	global    bool
	workspace bool
}

// skillListing is one row of the list report.
type skillListing struct {
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags,omitempty"`
	Version      string   `json:"version,omitempty"`
	Status       string   `json:"status"`
	AllowedTools string   `json:"allowed_tools,omitempty"`
}

func (a *App) RunList(args []string) int {
	opts, posArgs, ok := a.parseListArgs(args)
	if !ok {
		return ExitUsage
	}
	mode := a.resolveMode(opts.json, opts.raw)

	items := filterCatalogItems(a.Store.All(), posArgs)
	if len(items) == 0 {
		return a.writeEmptyCatalogList(mode, posArgs)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	listings := a.buildSkillListings(items, opts)
	if len(listings) == 0 {
		return a.writeEmptyScopedList(mode, opts, posArgs)
	}

	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(listings)
	case OutputModeRaw:
		writeListRaw(a.Stdout, listings)
	default:
		writeListHuman(a.Stdout, listings, opts, firstArg(posArgs))
	}
	return ExitSuccess
}

func (a *App) parseListArgs(args []string) (listOptions, []string, bool) {
	var opts listOptions
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	fs.BoolVar(&opts.json, "json", false, "Output in JSON format")
	fs.BoolVar(&opts.raw, "raw", false, "Output in raw format")
	fs.BoolVar(&opts.global, "global", false, "Filter skills installed in global scope (~/.agents/skills/)")
	fs.BoolVar(&opts.global, "g", false, "Filter skills installed in global scope (shorthand)")
	fs.BoolVar(&opts.workspace, "workspace", false, "Filter skills installed in workspace scope (.agents/skills/)")
	fs.BoolVar(&opts.workspace, "w", false, "Filter skills installed in workspace scope (shorthand)")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return opts, nil, false
	}
	return opts, posArgs, true
}

func firstArg(posArgs []string) string {
	if len(posArgs) == 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(posArgs[0]))
}

// filterCatalogItems applies the optional category/name/description query.
func filterCatalogItems(items []catalog.Item, posArgs []string) []catalog.Item {
	query := firstArg(posArgs)
	if query == "" {
		return items
	}

	filtered := make([]catalog.Item, 0, len(items))
	for _, it := range items {
		if strings.EqualFold(it.Category, query) ||
			strings.Contains(strings.ToLower(it.Name), query) ||
			strings.Contains(strings.ToLower(it.Description), query) {
			filtered = append(filtered, it)
		}
	}
	return filtered
}

// buildSkillListings resolves install status and applies scope filters.
func (a *App) buildSkillListings(items []catalog.Item, opts listOptions) []skillListing {
	st := a.loadState()
	listings := make([]skillListing, 0, len(items))
	for _, it := range items {
		status := a.getSkillStatusWithState(&it, st)
		if opts.global && status != state.ScopeGlobal {
			continue
		}
		if opts.workspace && status != state.ScopeWorkspace {
			continue
		}
		listings = append(listings, skillListing{
			Name:         it.Name,
			Category:     it.Category,
			Description:  it.Description,
			Tags:         it.Tags,
			Version:      it.Version,
			Status:       status,
			AllowedTools: it.AllowedTools,
		})
	}
	return listings
}

// writeEmptyCatalogList handles an empty catalog before any query.
func (a *App) writeEmptyCatalogList(mode OutputMode, posArgs []string) int {
	if mode == OutputModeJSON {
		_, _ = fmt.Fprintln(a.Stdout, "[]")
		return ExitSuccess
	}
	if mode == OutputModeRaw {
		return ExitSuccess
	}
	if query := firstArg(posArgs); query != "" {
		_, _ = fmt.Fprintf(a.Stderr, "error: no skills found matching %q\n", query)
	} else {
		_, _ = fmt.Fprintln(a.Stderr, "error: no skills found in catalog")
	}
	return ExitNotFound
}

// writeEmptyScopedList handles an empty result after scope filtering.
func (a *App) writeEmptyScopedList(mode OutputMode, opts listOptions, posArgs []string) int {
	if mode == OutputModeJSON {
		_, _ = fmt.Fprintln(a.Stdout, "[]")
		return ExitSuccess
	}
	if mode == OutputModeRaw {
		return ExitSuccess
	}
	if opts.global {
		_, _ = fmt.Fprintln(a.Stderr, "No skills found installed in global scope (~/.agents/skills/).")
		return ExitSuccess
	}
	if opts.workspace {
		_, _ = fmt.Fprintln(a.Stderr, "No skills found installed in workspace scope (.agents/skills/).")
		return ExitSuccess
	}
	if query := firstArg(posArgs); query != "" {
		_, _ = fmt.Fprintf(a.Stderr, "error: no skills found matching %q\n", query)
	} else {
		_, _ = fmt.Fprintln(a.Stderr, "error: no skills found in catalog")
	}
	return ExitNotFound
}

func writeListRaw(w interface{ Write([]byte) (int, error) }, listings []skillListing) {
	for _, l := range listings {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			l.Name, l.Category, l.Status, strings.Join(l.Tags, ","), sanitizeTSVField(l.Description))
	}
}

func writeListHuman(w interface{ Write([]byte) (int, error) }, listings []skillListing, opts listOptions, query string) {
	switch {
	case query != "":
		_, _ = fmt.Fprintf(w, "AVAILABLE SKILLS (matching %q, %d total):\n\n", query, len(listings))
	case opts.global:
		_, _ = fmt.Fprintf(w, "INSTALLED GLOBAL SKILLS (%d total):\n\n", len(listings))
	case opts.workspace:
		_, _ = fmt.Fprintf(w, "INSTALLED WORKSPACE SKILLS (%d total):\n\n", len(listings))
	default:
		_, _ = fmt.Fprintf(w, "AVAILABLE SKILLS (%d total):\n\n", len(listings))
	}

	_, _ = fmt.Fprintf(w, "%-4s %-22s %-14s %-12s %s\n", "#", "NAME", "CATEGORY", "STATUS", "DESCRIPTION")
	_, _ = fmt.Fprintf(w, "%-4s %-22s %-14s %-12s %s\n", "---", "----------------------", "--------------", "------------", "--------------------------------------------------")
	for idx, l := range listings {
		_, _ = fmt.Fprintf(w, "%-4d %-22s %-14s %-12s %s\n", idx+1, l.Name, l.Category, "["+l.Status+"]", truncateRunes(l.Description, 56))
	}
	writeListExamples(w)
}

func writeListExamples(w interface{ Write([]byte) (int, error) }) {
	_, _ = fmt.Fprintf(w, "\nEXAMPLES:\n")
	_, _ = fmt.Fprintf(w, "  Push skills to a remote profile repo or directory:\n")
	_, _ = fmt.Fprintf(w, "    <target>                github:owner/repo or directory path\n")
	_, _ = fmt.Fprintf(w, "                            Example: dojo profile push github:owner/repo 1,4,6 -g\n")
	_, _ = fmt.Fprintf(w, "    <skills...>             List numbers (1,4,6) or skill names (retro,cli-reviewer)\n")
	_, _ = fmt.Fprintf(w, "                            Example: dojo profile push github:owner/repo retro -g\n")
	_, _ = fmt.Fprintf(w, "    --all                   Push all installed skills\n")
	_, _ = fmt.Fprintf(w, "                            Example: dojo profile push github:owner/repo --all -g\n")
	_, _ = fmt.Fprintf(w, "    -g, --global            Push from global store (~/.gemini/skills)\n")
	_, _ = fmt.Fprintf(w, "                            Example: dojo profile push github:owner/repo 1,2 -g\n\n")

	_, _ = fmt.Fprintf(w, "  Inspect skill metadata and table of contents:\n")
	_, _ = fmt.Fprintf(w, "    <number>                Skill number from the list above\n")
	_, _ = fmt.Fprintf(w, "                            Example: dojo show 1\n")
	_, _ = fmt.Fprintf(w, "    <name>                  Skill name\n")
	_, _ = fmt.Fprintf(w, "                            Example: dojo toc retro\n\n")

	_, _ = fmt.Fprintf(w, "  Stream skill instructions into agent context:\n")
	_, _ = fmt.Fprintf(w, "    <number>                Skill number\n")
	_, _ = fmt.Fprintf(w, "                            Example: dojo load 1\n")
	_, _ = fmt.Fprintf(w, "    <number#section>        Specific #section to minimize tokens\n")
	_, _ = fmt.Fprintf(w, "                            Example: dojo load 1#architecture\n\n")

	_, _ = fmt.Fprintf(w, "  Install / learn skills into local or global scope:\n")
	_, _ = fmt.Fprintf(w, "    <number> [-g]           Install skill by list number\n")
	_, _ = fmt.Fprintf(w, "                            Example: dojo learn 1 -g\n")
	_, _ = fmt.Fprintf(w, "    <name> [-g]             Install skill by name\n")
	_, _ = fmt.Fprintf(w, "                            Example: dojo learn retro -g\n")
}
