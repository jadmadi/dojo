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
	"os"
	"sort"
	"strings"

	"github.com/jadmadi/dojo/internal/state"
)

// statusOptions holds the parsed flags for `dojo status`.
type statusOptions struct {
	global bool
	json   bool
	raw    bool
}

// statusData is the filtered snapshot the report renders.
type statusData struct {
	wsDir           string
	wsInstalled     map[string]state.SkillRecord
	wsForgotten     map[string]state.SkillRecord
	globalInstalled map[string]state.SkillRecord
	globalForgotten map[string]state.SkillRecord
	history         map[string]*state.HistoryRecord
	missingTargets  []string
}

// statusOutput is the JSON payload.
type statusOutput struct {
	Workspace       string                          `json:"workspace,omitempty"`
	WorkspaceSkills map[string]state.SkillRecord    `json:"workspace_skills,omitempty"`
	GlobalSkills    map[string]state.SkillRecord    `json:"global_skills"`
	ForgottenSkills map[string]state.SkillRecord    `json:"forgotten_skills,omitempty"`
	History         map[string]*state.HistoryRecord `json:"history,omitempty"`
}

func (a *App) RunStatus(args []string) int {
	opts, posArgs, ok := a.parseStatusArgs(args)
	if !ok {
		return ExitUsage
	}
	posArgs = a.resolveSkillSpecs(posArgs)

	data := a.collectStatus(posArgs, opts.global)
	switch a.resolveMode(opts.json, opts.raw) {
	case OutputModeJSON:
		a.writeStatusJSON(data, opts.global)
	case OutputModeRaw:
		writeStatusRaw(a.Stdout, data, opts.global)
	default:
		a.writeStatusHuman(data, opts.global)
	}

	if len(data.missingTargets) > 0 {
		if a.resolveMode(opts.json, opts.raw) == OutputModeHuman {
			for _, name := range data.missingTargets {
				_, _ = fmt.Fprintf(a.Stderr, "⚠ Skill %q has no installation or usage history.\n", name)
			}
		}
		return ExitNotFound
	}
	return ExitSuccess
}

func (a *App) parseStatusArgs(args []string) (statusOptions, []string, bool) {
	var opts statusOptions
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	fs.BoolVar(&opts.global, "global", false, "Show global skills status only")
	fs.BoolVar(&opts.global, "g", false, "Show global skills status only (shorthand)")
	fs.BoolVar(&opts.json, "json", false, "Output in JSON format")
	fs.BoolVar(&opts.raw, "raw", false, "Output in raw format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return opts, nil, false
	}
	return opts, posArgs, true
}

func (a *App) collectStatus(targetNames []string, globalOnly bool) statusData {
	st := a.loadState()
	wsDir, _ := os.Getwd()

	data := statusData{
		wsDir:           wsDir,
		wsInstalled:     st.GetInstalledSkills(state.ScopeWorkspace, wsDir),
		wsForgotten:     st.GetForgottenSkills(state.ScopeWorkspace, wsDir),
		globalInstalled: st.GetInstalledSkills(state.ScopeGlobal, ""),
		globalForgotten: st.GetForgottenSkills(state.ScopeGlobal, ""),
		history:         st.History,
	}

	targets := make(map[string]bool, len(targetNames))
	for _, name := range targetNames {
		targets[name] = true
	}
	if len(targets) > 0 {
		data.wsInstalled = filterSkillRecords(data.wsInstalled, targets)
		data.wsForgotten = filterSkillRecords(data.wsForgotten, targets)
		data.globalInstalled = filterSkillRecords(data.globalInstalled, targets)
		data.globalForgotten = filterSkillRecords(data.globalForgotten, targets)
		data.history = filterHistory(data.history, targets)
	}

	data.missingTargets = findMissingTargets(targetNames, data, globalOnly)
	return data
}

func filterSkillRecords(skills map[string]state.SkillRecord, targets map[string]bool) map[string]state.SkillRecord {
	if len(targets) == 0 {
		return skills
	}
	res := make(map[string]state.SkillRecord)
	for name, rec := range skills {
		if targets[name] {
			res[name] = rec
		}
	}
	return res
}

func filterHistory(history map[string]*state.HistoryRecord, targets map[string]bool) map[string]*state.HistoryRecord {
	if len(targets) == 0 || history == nil {
		return history
	}
	res := make(map[string]*state.HistoryRecord)
	for name, rec := range history {
		if targets[name] {
			res[name] = rec
		}
	}
	return res
}

func findMissingTargets(posArgs []string, data statusData, globalOnly bool) []string {
	var missing []string
	for _, name := range posArgs {
		_, inGlobal := data.globalInstalled[name]
		_, inGlobalForgotten := data.globalForgotten[name]
		_, inHistory := data.history[name]
		inWorkspace, inWorkspaceForgotten := false, false
		if !globalOnly {
			_, inWorkspace = data.wsInstalled[name]
			_, inWorkspaceForgotten = data.wsForgotten[name]
		}
		if !inGlobal && !inGlobalForgotten && !inHistory && !inWorkspace && !inWorkspaceForgotten {
			missing = append(missing, name)
		}
	}
	return missing
}

func (a *App) writeStatusJSON(data statusData, globalOnly bool) {
	out := statusOutput{
		GlobalSkills:    data.globalInstalled,
		ForgottenSkills: combinedForgotten(data, globalOnly),
		History:         data.history,
	}
	if !globalOnly {
		out.Workspace = data.wsDir
		out.WorkspaceSkills = data.wsInstalled
	}

	enc := json.NewEncoder(a.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func combinedForgotten(data statusData, globalOnly bool) map[string]state.SkillRecord {
	combined := make(map[string]state.SkillRecord)
	for k, v := range data.globalForgotten {
		combined[k] = v
	}
	if !globalOnly {
		for k, v := range data.wsForgotten {
			combined[k] = v
		}
	}
	return combined
}

func writeStatusRaw(w interface{ Write([]byte) (int, error) }, data statusData, globalOnly bool) {
	if !globalOnly {
		writeRawRecords(w, "workspace", "installed", data.wsInstalled)
		writeRawRecords(w, "workspace", "forgotten", data.wsForgotten)
	}
	writeRawRecords(w, "global", "installed", data.globalInstalled)
	writeRawRecords(w, "global", "forgotten", data.globalForgotten)
}

func writeRawRecords(w interface{ Write([]byte) (int, error) }, scope, status string, records map[string]state.SkillRecord) {
	names := make([]string, 0, len(records))
	for n := range records {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		rec := records[n]
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			sanitizeTSVField(scope),
			sanitizeTSVField(rec.Name),
			sanitizeTSVField(rec.Version),
			sanitizeTSVField(status),
			sanitizeTSVField(rec.Origin),
		)
	}
}

func (a *App) writeStatusHuman(data statusData, globalOnly bool) {
	_, _ = fmt.Fprintf(a.Stdout, "Dojo Status\n")
	if !globalOnly {
		_, _ = fmt.Fprintf(a.Stdout, "Workspace: %s\n\n", data.wsDir)
		writeHumanSkillGroup(a.Stdout, "Workspace Skills", data.wsInstalled)
	}
	_, _ = fmt.Fprintf(a.Stdout, "\nGlobal Skills (%d):\n", len(data.globalInstalled))
	writeHumanSkillRows(a.Stdout, data.globalInstalled)

	writeHumanForgotten(a.Stdout, data, globalOnly)
	writeHumanHistory(a.Stdout, data.history)
}

func writeHumanSkillGroup(w interface{ Write([]byte) (int, error) }, title string, skills map[string]state.SkillRecord) {
	_, _ = fmt.Fprintf(w, "%s (%d):\n", title, len(skills))
	writeHumanSkillRows(w, skills)
}

func writeHumanSkillRows(w interface{ Write([]byte) (int, error) }, skills map[string]state.SkillRecord) {
	if len(skills) == 0 {
		_, _ = fmt.Fprintf(w, "  (none)\n")
		return
	}
	names := make([]string, 0, len(skills))
	for n := range skills {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		rec := skills[n]
		ver := rec.Version
		if ver == "" {
			ver = "unversioned"
		}
		if rec.Origin == state.OriginCustom {
			_, _ = fmt.Fprintf(w, "  • %-20s %-12s (%d files, custom - read-only to dojo)\n", rec.Name, ver, len(rec.Files))
			continue
		}
		_, _ = fmt.Fprintf(w, "  ✓ %-20s %-12s (%d files, installed %s)\n", rec.Name, ver, len(rec.Files), rec.InstalledAt.Format("2006-01-02"))
	}
}

func writeHumanForgotten(w interface{ Write([]byte) (int, error) }, data statusData, globalOnly bool) {
	count := len(data.globalForgotten)
	if !globalOnly {
		count += len(data.wsForgotten)
	}
	if count == 0 {
		return
	}

	_, _ = fmt.Fprintf(w, "\nForgotten Skills:\n")
	if !globalOnly {
		writeForgottenRows(w, data.wsForgotten, "workspace")
	}
	writeForgottenRows(w, data.globalForgotten, "global")
}

func writeForgottenRows(w interface{ Write([]byte) (int, error) }, skills map[string]state.SkillRecord, scope string) {
	names := make([]string, 0, len(skills))
	for n := range skills {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		rec := skills[n]
		uninstalled := ""
		if rec.UninstalledAt != nil {
			uninstalled = rec.UninstalledAt.Format("2006-01-02")
		}
		_, _ = fmt.Fprintf(w, "  ✗ %-20s (%s, uninstalled %s)\n", rec.Name, scope, uninstalled)
	}
}

func writeHumanHistory(w interface{ Write([]byte) (int, error) }, history map[string]*state.HistoryRecord) {
	if len(history) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "\nSkill Usage History:\n")

	names := make([]string, 0, len(history))
	for n := range history {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		_, _ = fmt.Fprintf(w, "  • %-20s (%s, %s)\n", n, history[n].LastSeenAt.Format("2006-01-02 15:04"), historyDetails(history[n]))
	}
}

func historyDetails(h *state.HistoryRecord) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%d loads", h.LoadCount))
	if h.PageLoadCount > 0 {
		parts = append(parts, fmt.Sprintf("+%d pages", h.PageLoadCount))
	}
	if h.SubpathLoadCount > 0 {
		parts = append(parts, fmt.Sprintf("+%d subpaths", h.SubpathLoadCount))
	}
	if h.RunCount > 0 {
		parts = append(parts, fmt.Sprintf("%d runs", h.RunCount))
	}
	if h.LearnCount > 0 {
		parts = append(parts, fmt.Sprintf("%d learns", h.LearnCount))
	}
	return strings.Join(parts, ", ")
}
