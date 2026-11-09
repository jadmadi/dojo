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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/runner"
	"github.com/jadmadi/dojo/internal/search"
	"github.com/jadmadi/dojo/internal/state"
)

// updateOptions holds the parsed flags for `dojo update`.
type updateOptions struct {
	global  bool
	yes     bool
	force   bool
	all     bool
	refresh bool
	json    bool
	raw     bool

	// explicitTargets is true when the user named skills on the command line.
	explicitTargets bool
}

// skillUpdateCheck is one row of the update report.
type skillUpdateCheck struct {
	Name            string   `json:"name"`
	InstalledVer    string   `json:"installed_version,omitempty"`
	RemoteVer       string   `json:"remote_version,omitempty"`
	InstalledDigest string   `json:"installed_digest,omitempty"`
	RemoteDigest    string   `json:"remote_digest,omitempty"`
	HasUpdate       bool     `json:"has_update"`
	RequiresYes     bool     `json:"requires_yes"`
	IsDirty         bool     `json:"is_dirty"`
	Reason          string   `json:"reason"`
	Status          string   `json:"status"`
	Files           []string `json:"files,omitempty"`
	Message         string   `json:"message,omitempty"`
}

func (a *App) RunUpdate(args []string) int {
	opts, posArgs, ok := a.parseUpdateArgs(args)
	if !ok {
		return ExitUsage
	}
	posArgs = a.resolveSkillSpecs(posArgs)
	opts.explicitTargets = len(posArgs) > 0

	scope, wsDir := resolveScopeAndDir(opts.global)
	st := a.loadState()
	mode := a.resolveMode(opts.json, opts.raw)

	syncExitCode := a.syncUpdateCatalogs(st, opts.refresh, mode)

	installed := st.GetInstalledSkills(scope, wsDir)
	if len(installed) == 0 && len(posArgs) == 0 {
		writeEmptyUpdateReport(a.Stdout, mode, scope)
		return syncExitCode
	}

	dryRun := len(posArgs) == 0 && !opts.all
	installedNames, targetMap, checks := planUpdateTargets(installed, posArgs, scope)

	errExitCode := ExitSuccess
	syncedNamedCatalogs := make(map[string]bool)
	for _, name := range installedNames {
		if !targetMap[name] {
			continue
		}

		check, code, abort := a.checkOneSkillUpdate(name, installed[name], scope, wsDir, st, opts, dryRun, syncedNamedCatalogs)
		if abort {
			return ExitUsage
		}
		checks = append(checks, check)
		if code != ExitSuccess {
			errExitCode = promoteExitCode(errExitCode, code)
		}
	}

	if !dryRun {
		a.saveState(st)
	}

	a.writeUpdateReport(checks, mode, dryRun, scope, opts.global, installedNames)
	return updateExitCode(checks, errExitCode, syncExitCode)
}

func (a *App) parseUpdateArgs(args []string) (updateOptions, []string, bool) {
	var opts updateOptions
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	fs.BoolVar(&opts.global, "global", false, "Update global skills (~/.agents/skills/)")
	fs.BoolVar(&opts.global, "g", false, "Update global skills (shorthand)")
	fs.BoolVar(&opts.yes, "yes", false, "Accept unversioned or drifted upstream updates")
	fs.BoolVar(&opts.yes, "y", false, "Accept unversioned or drifted upstream updates (shorthand)")
	fs.BoolVar(&opts.force, "force", false, "Force update regardless of version/digest status")
	fs.BoolVar(&opts.force, "f", false, "Force update regardless of status (shorthand)")
	fs.BoolVar(&opts.all, "all", false, "Apply updates to all installed skills")
	fs.BoolVar(&opts.all, "a", false, "Apply updates to all installed skills (shorthand)")
	fs.BoolVar(&opts.refresh, "refresh", false, "Bypass cache and force refresh remote catalog")
	fs.BoolVar(&opts.refresh, "r", false, "Bypass cache and force refresh remote catalog (shorthand)")
	fs.BoolVar(&opts.json, "json", false, "Output in JSON format")
	fs.BoolVar(&opts.raw, "raw", false, "Output in raw format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return opts, nil, false
	}
	return opts, posArgs, true
}

func (a *App) syncUpdateCatalogs(st *state.State, refresh bool, mode OutputMode) int {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if refresh {
		if syncer, ok := a.Store.(interface {
			Sync(context.Context, bool) error
		}); ok {
			_ = syncer.Sync(ctx, true)
		}
	}
	return a.syncRegisteredCatalogs(ctx, st, mode)
}

func writeEmptyUpdateReport(w interface{ Write([]byte) (int, error) }, mode OutputMode, scope string) {
	switch mode {
	case OutputModeJSON:
		_ = json.NewEncoder(w).Encode([]any{})
	case OutputModeRaw:
		// empty output
	default:
		_, _ = fmt.Fprintf(w, "No installed skills found in %s scope.\n", scope)
	}
}

// planUpdateTargets resolves the requested skills, records missing ones, and
// returns the sorted installed names plus the target set.
func planUpdateTargets(installed map[string]state.SkillRecord, posArgs []string, scope string) ([]string, map[string]bool, []skillUpdateCheck) {
	installedNames := make([]string, 0, len(installed))
	for name := range installed {
		installedNames = append(installedNames, name)
	}
	sort.Strings(installedNames)

	targetMap := make(map[string]bool)
	var checks []skillUpdateCheck

	if len(posArgs) > 0 {
		for _, name := range posArgs {
			targetMap[name] = true
			if _, ok := installed[name]; !ok {
				checks = append(checks, skillUpdateCheck{
					Name:   name,
					Status: "not_found",
					Reason: fmt.Sprintf("skill %q is not installed in %s scope", name, scope),
				})
			}
		}
		return installedNames, targetMap, checks
	}

	for _, name := range installedNames {
		targetMap[name] = true
	}
	return installedNames, targetMap, checks
}

// checkOneSkillUpdate resolves one skill's upstream state and applies the
// update when allowed. The bool result aborts the whole command.
func (a *App) checkOneSkillUpdate(name string, rec state.SkillRecord, scope, wsDir string, st *state.State, opts updateOptions, dryRun bool, syncedNamedCatalogs map[string]bool) (skillUpdateCheck, int, bool) {
	if rec.Origin == state.OriginCustom {
		if !dryRun && opts.explicitTargets {
			_, _ = fmt.Fprintf(a.Stderr, "error: skill %q is a custom/external skill (not learned from catalog). Dojo cannot update custom skills.\n", name)
			return skillUpdateCheck{}, ExitUsage, true
		}
		return skillUpdateCheck{
			Name:         name,
			InstalledVer: rec.Version,
			Status:       "custom",
			Reason:       "custom skill (not in catalog, skipped)",
		}, ExitSuccess, false
	}

	item, err := a.fetchUpdateItem(name, &rec, st, syncedNamedCatalogs)
	if err != nil || item == nil {
		code := ExitSuccess
		if err != nil {
			if mapped := MapErrorToExitCode(err); mapped != ExitNotFound {
				code = mapped
			}
		}
		return skillUpdateCheck{
			Name:         name,
			InstalledVer: rec.Version,
			Status:       "error",
			Reason:       fmt.Sprintf("not found in upstream source: %v", err),
		}, code, false
	}

	remoteVer := a.resolveRemoteVersion(item)
	hasUpdate, requiresYes, reason, isDirty := evaluateUpdateAvailability(rec, item, remoteVer, opts.force)

	check := skillUpdateCheck{
		Name:            name,
		InstalledVer:    rec.Version,
		RemoteVer:       remoteVer,
		InstalledDigest: rec.Digest,
		HasUpdate:       hasUpdate,
		RequiresYes:     requiresYes,
		IsDirty:         isDirty,
		Reason:          reason,
		Status:          "up_to_date",
	}
	if hasUpdate {
		check.Status = "update_available"
	}

	if dryRun || (!hasUpdate && !opts.force) {
		return check, ExitSuccess, false
	}
	return a.applySkillUpdate(check, &rec, item, scope, wsDir, st, opts)
}

// fetchUpdateItem resolves the upstream catalog item for an installed skill,
// following GitHub, raw URL, and named catalog sources.
func (a *App) fetchUpdateItem(name string, rec *state.SkillRecord, st *state.State, syncedNamedCatalogs map[string]bool) (*catalog.Item, error) {
	if isGitHubBacked(rec) {
		return a.fetchGitHubUpdateItem(rec)
	}

	item, err := a.Store.Get(name)
	if item != nil && item.Source == "local" {
		item = nil
		err = catalog.ErrItemNotFound
	}

	if catalogName := catalogCandidate(rec, item); catalogName != "" && !syncedNamedCatalogs[catalogName] {
		syncedNamedCatalogs[catalogName] = true
		if catRec, ok := st.GetCatalog(catalogName); ok && catRec.Type != state.CatalogTypeDefault {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			if freshItem := a.refreshCatalogForItem(ctx, &catalog.Item{ID: name, Name: name, Source: catRec.Name}, ""); freshItem != nil {
				item = freshItem
				err = nil
			}
			cancel()
		}
	}

	if err != nil || item == nil || (item.URL == "" && item.Body == "") {
		if ghItem := a.fetchFromNamedGitHubCatalogs(name, rec, st); ghItem != nil {
			item = ghItem
			err = nil
		}
	}
	return item, err
}

func isGitHubBacked(rec *state.SkillRecord) bool {
	if rec.Origin == state.OriginGitHub {
		return true
	}
	return rec.RawURL != "" && (strings.Contains(rec.RawURL, "github.com") || strings.Contains(rec.RawURL, "raw.githubusercontent.com"))
}

func (a *App) fetchGitHubUpdateItem(rec *state.SkillRecord) (*catalog.Item, error) {
	var target *catalog.GitHubTarget
	var ok bool
	if rec.RawURL != "" {
		target, ok = catalog.ParseGitHubTarget(rec.RawURL)
	}
	if !ok && rec.Repo != "" {
		target, ok = catalog.ParseGitHubTarget(rec.Repo)
	}
	if !ok {
		return nil, fmt.Errorf("invalid github repository: %s", rec.Repo)
	}
	if rec.Ref != "" {
		target.Ref = rec.Ref
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return catalog.FetchGitHubSkill(ctx, target, a.getHTTPClient())
}

func catalogCandidate(rec *state.SkillRecord, item *catalog.Item) string {
	if rec.Catalog != "" {
		return rec.Catalog
	}
	if item == nil || item.Source == "" || item.Source == "default" || strings.HasPrefix(item.Source, "http") {
		return ""
	}
	return strings.TrimPrefix(item.Source, "github:")
}

func (a *App) fetchFromNamedGitHubCatalogs(name string, rec *state.SkillRecord, st *state.State) *catalog.Item {
	lookupCatalogs := []string{rec.Catalog, name, "dojo"}
	for _, catName := range lookupCatalogs {
		if catName == "" {
			continue
		}
		catRec, ok := st.GetCatalog(catName)
		if !ok || catRec.Type != state.CatalogTypeGitHub {
			continue
		}
		target, okTarget := catalog.ParseGitHubTarget(catRec.Source)
		if !okTarget {
			continue
		}
		if catRec.Ref != "" {
			target.Ref = catRec.Ref
		}
		if target.Subpath == "" {
			target.Subpath = "skills/" + name
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		ghItem, ghErr := catalog.FetchGitHubSkill(ctx, target, a.getHTTPClient())
		cancel()
		if ghErr != nil || ghItem == nil {
			continue
		}
		rec.Origin = state.OriginGitHub
		rec.Repo = catRec.Source
		rec.Ref = catRec.Ref
		if ghItem.URL != "" {
			rec.RawURL = ghItem.URL
		}
		return ghItem
	}
	return nil
}

// resolveRemoteVersion reads the version from the item, its metadata, or the
// lazily fetched SKILL.md frontmatter.
func (a *App) resolveRemoteVersion(item *catalog.Item) string {
	if item.Version != "" {
		return item.Version
	}
	if item.Metadata != nil && item.Metadata["version"] != "" {
		return item.Metadata["version"]
	}
	if item.URL == "" || item.Body != "" {
		return ""
	}

	content, err := a.Store.LoadContent(item)
	if err != nil {
		return ""
	}
	fm, _, err := catalog.ParseFrontmatter([]byte(content))
	if err != nil || fm == nil || fm.Metadata == nil || fm.Metadata["version"] == "" {
		return ""
	}
	item.Version = fm.Metadata["version"]
	if item.Metadata == nil {
		item.Metadata = fm.Metadata
	}
	return item.Version
}

func evaluateUpdateAvailability(rec state.SkillRecord, item *catalog.Item, remoteVer string, force bool) (hasUpdate, requiresYes bool, reason string, isDirty bool) {
	switch {
	case remoteVer != "" && rec.Version != "":
		if catalog.CompareSemver(remoteVer, rec.Version) > 0 {
			hasUpdate = true
			reason = fmt.Sprintf("new version available: %s -> %s", rec.Version, remoteVer)
		} else if remoteVer == rec.Version {
			reason = fmt.Sprintf("up to date (%s)", rec.Version)
		} else {
			reason = fmt.Sprintf("installed version (%s) is newer than catalog (%s)", rec.Version, remoteVer)
		}
	case item.Digest != "" && rec.Digest != "":
		if item.Digest == rec.Digest {
			reason = "up to date (digest match)"
		} else {
			hasUpdate = true
			reason = fmt.Sprintf("digest updated: %s -> %s", rec.Digest, item.Digest)
		}
	default:
		requiresYes = true
		hasUpdate = true
		reason = "unversioned skill in catalog (cannot verify if newer or different)"
	}

	isDirty = rec.InstalledDigest != "" && rec.Digest != "" && rec.Digest != rec.InstalledDigest
	if isDirty && hasUpdate {
		requiresYes = true
		reason = fmt.Sprintf("local files modified since install; %s", reason)
	}
	if force && !hasUpdate {
		reason = fmt.Sprintf("force update requested (%s)", rec.Version)
	}
	return hasUpdate, requiresYes, reason, isDirty
}

// applySkillUpdate installs the new upstream content unless the local edits or
// confirmation rules block it.
func (a *App) applySkillUpdate(check skillUpdateCheck, rec *state.SkillRecord, item *catalog.Item, scope, wsDir string, st *state.State, opts updateOptions) (skillUpdateCheck, int, bool) {
	switch {
	case check.IsDirty && !opts.force:
		check.Status = "conflict"
		check.Message = "Local skill has been modified since install; pass -f/--force to overwrite local edits"
		return check, ExitSuccess, false
	case check.RequiresYes && !opts.yes && !opts.force:
		check.Status = "blocked"
		check.Message = "Pass -y/--yes or -f/--force to accept unversioned/drifted update"
		return check, ExitSuccess, false
	}

	content, err := a.loadItemContentWithOpts(item, false, true, opts.force)
	if err != nil {
		check.Status = "error"
		check.Message = err.Error()
		return check, MapErrorToExitCode(err), false
	}

	extraCandidates := itemResourceCandidates(a, item)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	files, err := runner.InstallSkillContext(ctx, *item, rec.Path, content, extraCandidates, a.getHTTPClient())
	cancel()
	if err != nil {
		check.Status = "error"
		check.Message = err.Error()
		return check, MapErrorToExitCode(err), false
	}

	newDigest, _ := state.ComputePackageDigest(rec.Path, files)
	rec.Version = check.RemoteVer
	rec.Digest = newDigest
	rec.Files = files
	rec.UpdatedAt = time.Now().UTC()
	st.RecordInstall(scope, wsDir, *rec)

	check.Status = "updated"
	check.Files = files
	return check, ExitSuccess, false
}

func itemResourceCandidates(a *App, item *catalog.Item) []string {
	var extra []string
	for _, refFile := range a.listReferenceFiles(*item) {
		extra = append(extra, "references/"+refFile)
	}
	for _, scr := range a.listScripts(*item) {
		extra = append(extra, "scripts/"+scr)
	}
	return extra
}

func promoteExitCode(current, code int) int {
	if code == ExitSuccess {
		return current
	}
	if current == ExitSuccess || current == ExitGeneral {
		return code
	}
	return current
}

func (a *App) writeUpdateReport(checks []skillUpdateCheck, mode OutputMode, dryRun bool, scope string, global bool, installedNames []string) {
	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(checks)
	case OutputModeRaw:
		writeUpdateRaw(a.Stdout, checks, dryRun)
	default:
		if dryRun {
			a.writeUpdateDryRun(checks, scope, global, installedNames)
			return
		}
		writeUpdateApplied(a.Stdout, a.Stderr, checks, scope, installedNames)
	}
}

func writeUpdateRaw(w interface{ Write([]byte) (int, error) }, checks []skillUpdateCheck, dryRun bool) {
	for _, c := range checks {
		if dryRun {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				sanitizeTSVField(c.Name),
				sanitizeTSVField(c.InstalledVer),
				sanitizeTSVField(c.RemoteVer),
				sanitizeTSVField(c.Status),
				sanitizeTSVField(c.Reason),
			)
			continue
		}
		if c.Status == "updated" {
			_, _ = fmt.Fprintf(w, "updated\t%s\t%s\n",
				sanitizeTSVField(c.Name),
				sanitizeTSVField(c.Reason),
			)
			continue
		}
		msg := c.Reason
		if c.Message != "" {
			msg = c.Message
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
			sanitizeTSVField(c.Status),
			sanitizeTSVField(c.Name),
			sanitizeTSVField(msg),
		)
	}
}

func (a *App) writeUpdateDryRun(checks []skillUpdateCheck, scope string, global bool, installedNames []string) {
	hasAnyUpdates := false
	hasAnyErrors := false
	_, _ = fmt.Fprintf(a.Stdout, "Checking for skill updates (%s scope)...\n\n", scope)

	for _, c := range checks {
		switch {
		case c.Status == "custom":
			_, _ = fmt.Fprintf(a.Stdout, "  • %-20s [Custom skill] %s\n", c.Name, c.Reason)
		case c.Status == "not_found":
			hasAnyErrors = true
			_, _ = fmt.Fprintf(a.Stderr, "  ⚠ %-20s %s\n", c.Name, c.Reason)
			writeUpdateSuggestions(a.Stderr, c.Name, installedNames, "    Did you mean:")
		case c.Status == "error":
			hasAnyErrors = true
			msg := c.Reason
			if c.Message != "" {
				msg = c.Message
			}
			_, _ = fmt.Fprintf(a.Stderr, "  ✗ %-20s [Error] %s\n", c.Name, msg)
		case c.HasUpdate:
			hasAnyUpdates = true
			switch {
			case c.IsDirty:
				_, _ = fmt.Fprintf(a.Stdout, "  ⚠ %-20s [Update available with -f] %s\n", c.Name, c.Reason)
			case c.RequiresYes:
				_, _ = fmt.Fprintf(a.Stdout, "  ⚠ %-20s [Update available with -y] %s\n", c.Name, c.Reason)
			default:
				_, _ = fmt.Fprintf(a.Stdout, "  ↑ %-20s [Update available] %s\n", c.Name, c.Reason)
			}
		default:
			_, _ = fmt.Fprintf(a.Stdout, "  ✓ %-20s %s\n", c.Name, c.Reason)
		}
	}

	if hasAnyUpdates {
		if global {
			_, _ = fmt.Fprintln(a.Stdout, "\nRun 'dojo update -g <skill>' or 'dojo update -g --all' to apply updates.")
		} else {
			_, _ = fmt.Fprintln(a.Stdout, "\nRun 'dojo update <skill>' or 'dojo update --all' to apply updates.")
		}
	} else if !hasAnyErrors {
		_, _ = fmt.Fprintln(a.Stdout, "\nAll installed skills are up to date.")
	}
}

func writeUpdateSuggestions(w interface{ Write([]byte) (int, error) }, name string, installedNames []string, header string) {
	if len(installedNames) == 0 {
		return
	}
	suggs := search.Suggest(name, installedNames, 3)
	if len(suggs) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, header)
	for _, s := range suggs {
		_, _ = fmt.Fprintf(w, "      - %s\n", s)
	}
}

func writeUpdateApplied(stdout, stderr interface{ Write([]byte) (int, error) }, checks []skillUpdateCheck, scope string, installedNames []string) {
	for _, c := range checks {
		switch c.Status {
		case "updated":
			_, _ = fmt.Fprintf(stdout, "✓ Updated skill %q (%s, %d files)\n", c.Name, c.Reason, len(c.Files))
		case "up_to_date":
			_, _ = fmt.Fprintf(stdout, "✓ Skill %q is up to date (%s)\n", c.Name, c.Reason)
		case "custom":
			_, _ = fmt.Fprintf(stdout, "• Skipped custom skill %q (%s)\n", c.Name, c.Reason)
		case "not_found":
			_, _ = fmt.Fprintf(stderr, "⚠ Skill %q is not installed in %s scope.\n", c.Name, scope)
			if len(installedNames) == 0 {
				continue
			}
			suggs := search.Suggest(c.Name, installedNames, 3)
			if len(suggs) == 0 {
				continue
			}
			_, _ = fmt.Fprintln(stderr, "\nDid you mean:")
			for _, s := range suggs {
				_, _ = fmt.Fprintf(stderr, "  - dojo update %s\n", s)
			}
		case "conflict", "blocked":
			_, _ = fmt.Fprintf(stderr, "⚠ Skill %q was not updated: %s\n", c.Name, c.Message)
		case "error":
			msg := c.Message
			if msg == "" {
				msg = c.Reason
			}
			_, _ = fmt.Fprintf(stderr, "✗ Failed to update skill %q: %s\n", c.Name, msg)
		}
	}
}

func updateExitCode(checks []skillUpdateCheck, errExitCode, syncExitCode int) int {
	hasNotFound := false
	hasGeneralError := false
	for _, c := range checks {
		switch c.Status {
		case "not_found":
			hasNotFound = true
		case "error", "conflict", "blocked":
			hasGeneralError = true
		}
	}

	if hasNotFound {
		return ExitNotFound
	}
	if errExitCode != ExitSuccess {
		return errExitCode
	}
	if hasGeneralError {
		return ExitGeneral
	}
	return syncExitCode
}
