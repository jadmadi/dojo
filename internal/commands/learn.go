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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/harness"
	"github.com/jadmadi/dojo/internal/runner"
	"github.com/jadmadi/dojo/internal/state"
)

func resolveInstallDirectory(scope, skillName string) (string, error) {
	if scope == state.ScopeGlobal {
		homeDir, err := os.UserHomeDir()
		if err != nil || homeDir == "" {
			return "", fmt.Errorf("unable to determine user home directory: %w", err)
		}
		return filepath.Join(homeDir, ".agents", "skills", skillName), nil
	}
	wsDir, err := os.Getwd()
	if err != nil || wsDir == "" {
		return "", fmt.Errorf("unable to determine working directory: %w", err)
	}
	return filepath.Join(wsDir, ".agents", "skills", skillName), nil
}

// learnOptions holds the parsed flags for `dojo learn`.
type learnOptions struct {
	global   bool
	force    bool
	load     bool
	withDeps bool
	target   string
	json     bool
	raw      bool
}

// installResult is one row of the learn report.
type installResult struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Path    string   `json:"path,omitempty"`
	Version string   `json:"version,omitempty"`
	Digest  string   `json:"digest,omitempty"`
	Files   []string `json:"files,omitempty"`
	Targets []string `json:"targets,omitempty"`
	Content string   `json:"content,omitempty"`
	Message string   `json:"message,omitempty"`
}

func (a *App) RunLearn(args []string) int {
	opts, posArgs, ok := a.parseLearnArgs(args)
	if !ok {
		return ExitUsage
	}
	posArgs = a.resolveSkillSpecs(posArgs)
	if len(posArgs) == 0 {
		_, _ = fmt.Fprintln(a.Stderr, "error: learn requires at least one skill name or repository (e.g. 'github:owner/repo')")
		return ExitUsage
	}

	scope, wsDir := resolveScopeAndDir(opts.global)
	syncBaseDir := wsDir
	if opts.global {
		syncBaseDir, _ = os.UserHomeDir()
	}

	syncTargets, code := a.resolveLearnTargets(opts.target, syncBaseDir)
	if code != ExitSuccess {
		return code
	}

	items, code := a.resolveLearnItems(posArgs)
	if code != ExitSuccess {
		return code
	}
	if opts.withDeps {
		items, code = a.expandLearnDependencies(items)
		if code != ExitSuccess {
			return code
		}
	}

	st := a.loadState()
	installed := st.GetInstalledSkills(scope, wsDir)

	var results []installResult
	errExitCode := ExitSuccess
	for _, item := range items {
		res, itemCode := a.installOneItem(item, opts, scope, wsDir, syncBaseDir, syncTargets, installed, st)
		if itemCode != ExitSuccess {
			errExitCode = promoteExitCode(errExitCode, itemCode)
		}
		results = append(results, res)
	}

	a.saveState(st)
	a.writeLearnReport(results, opts, scope)
	return learnExitCode(errExitCode, results)
}

func (a *App) parseLearnArgs(args []string) (learnOptions, []string, bool) {
	var opts learnOptions
	fs := flag.NewFlagSet("learn", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	fs.BoolVar(&opts.global, "global", false, "Install skill to global store (~/.agents/skills/)")
	fs.BoolVar(&opts.global, "g", false, "Install skill to global store (shorthand)")
	fs.BoolVar(&opts.force, "force", false, "Overwrite existing skill installation")
	fs.BoolVar(&opts.force, "f", false, "Overwrite existing skill installation (shorthand)")
	fs.BoolVar(&opts.load, "load", false, "Output loaded skill markdown to stdout upon successful installation")
	fs.BoolVar(&opts.load, "l", false, "Output loaded skill markdown (shorthand)")
	fs.BoolVar(&opts.withDeps, "with-deps", false, "Install skill and all transitive dependencies in topological order")
	fs.StringVar(&opts.target, "target", "", "Also sync learned skill(s) to target harnesses (e.g. all, claude,cursor,gemini,windsurf,opencode,sila)")
	fs.StringVar(&opts.target, "t", "", "Also sync learned skill(s) to target harnesses (shorthand)")
	fs.BoolVar(&opts.json, "json", false, "Output in JSON format")
	fs.BoolVar(&opts.raw, "raw", false, "Output in raw format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return opts, nil, false
	}
	return opts, posArgs, true
}

func (a *App) resolveLearnTargets(targetSpec, syncBaseDir string) ([]harness.Target, int) {
	if targetSpec == "" {
		return nil, ExitSuccess
	}
	targets, err := harness.ParseTargets(targetSpec, syncBaseDir)
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: %v\n", err)
		return nil, ExitUsage
	}
	return targets, ExitSuccess
}

// resolveLearnItems turns specs into catalog items, fetching GitHub targets and
// reporting missing names.
func (a *App) resolveLearnItems(specs []string) ([]catalog.Item, int) {
	var items []catalog.Item
	var notFounds []string

	for _, spec := range specs {
		if isGitHubSkillSpec(spec) {
			target, ok := catalog.ParseGitHubTarget(spec)
			if !ok {
				_, _ = fmt.Fprintf(a.Stderr, "error: invalid github skill target %q\n", spec)
				return nil, ExitUsage
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			item, err := catalog.FetchGitHubSkill(ctx, target, a.getHTTPClient())
			cancel()
			if err != nil {
				_, _ = fmt.Fprintf(a.Stderr, "error: fetching skill from github %q: %v\n", spec, err)
				return nil, MapErrorToExitCode(err)
			}
			if cat, ok := a.Store.(*catalog.Catalog); ok {
				cat.AddItem(*item)
			}
			items = append(items, *item)
			continue
		}

		item, err := a.Store.Get(spec)
		if err != nil {
			notFounds = append(notFounds, spec)
			continue
		}
		items = append(items, *item)
	}

	if len(notFounds) > 0 {
		for _, name := range notFounds {
			a.printNotFoundWithSuggestions("learn", name)
		}
		return nil, ExitNotFound
	}
	return items, ExitSuccess
}

// expandLearnDependencies resolves the transitive dependency graph in
// topological order and de-duplicates by name.
func (a *App) expandLearnDependencies(items []catalog.Item) ([]catalog.Item, int) {
	var expanded []catalog.Item
	seen := make(map[string]bool)

	for _, rootItem := range items {
		if len(rootItem.Dependencies) == 0 {
			if rawContent, err := a.loadItemContent(&rootItem); err == nil {
				catalog.EnrichItemFromSkillMD(&rootItem, []byte(rawContent))
				if cat, ok := a.Store.(*catalog.Catalog); ok {
					cat.AddItem(rootItem)
				}
			}
		}

		resolved, err := catalog.ResolveDependencies(rootItem.Name, a.Store)
		if err != nil {
			_, _ = fmt.Fprintf(a.Stderr, "error: resolving dependencies for %q: %v\n", rootItem.Name, err)
			return nil, MapErrorToExitCode(err)
		}
		for _, dep := range resolved {
			low := strings.ToLower(dep.Name)
			if !seen[low] {
				seen[low] = true
				expanded = append(expanded, dep)
			}
		}
	}
	return expanded, ExitSuccess
}

// installOneItem installs a single catalog item and returns its report row.
func (a *App) installOneItem(item catalog.Item, opts learnOptions, scope, wsDir, syncBaseDir string, syncTargets []harness.Target, installed map[string]state.SkillRecord, st *state.State) (installResult, int) {
	skillDir, dirErr := resolveInstallDirectory(scope, item.Name)
	if dirErr != nil {
		return installResult{
			Name:    item.Name,
			Status:  "error",
			Message: dirErr.Error(),
		}, ExitGeneral
	}

	if rec, exists := installed[item.Name]; exists && !opts.force {
		return installResult{
			Name:    item.Name,
			Status:  "already_installed",
			Path:    rec.Path,
			Version: rec.Version,
			Digest:  rec.Digest,
			Files:   rec.Files,
			Targets: syncedTargetPaths(item.Name, rec.Path, syncBaseDir, syncTargets),
			Message: fmt.Sprintf("skill %q is already installed in %s scope; pass -f/--force to overwrite", item.Name, scope),
		}, ExitSuccess
	}

	content, err := a.loadItemContent(&item)
	if err != nil {
		return installResult{
			Name:    item.Name,
			Status:  "error",
			Message: fmt.Sprintf("failed to load skill content: %v", err),
		}, MapErrorToExitCode(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	files, err := runner.InstallSkillContext(ctx, item, skillDir, content, itemResourceCandidates(a, &item), a.getHTTPClient())
	cancel()
	if err != nil {
		return installResult{
			Name:    item.Name,
			Status:  "error",
			Message: fmt.Sprintf("installation failed: %v", err),
		}, MapErrorToExitCode(err)
	}

	pkgDigest, _ := state.ComputePackageDigest(skillDir, files)
	ver := learnItemVersion(item)
	rec := buildLearnedRecord(item, scope, skillDir, ver, pkgDigest, files)
	st.RecordInstall(scope, wsDir, rec)
	st.RecordLearn(item.Name)

	res := installResult{
		Name:    item.Name,
		Status:  "installed",
		Path:    skillDir,
		Version: ver,
		Digest:  pkgDigest,
		Files:   files,
		Targets: syncedTargetPaths(item.Name, skillDir, syncBaseDir, syncTargets),
	}
	if opts.load {
		res.Content = stripFrontmatter(content)
		st.RecordLoadDetailed(item.Name, "", 1)
	}
	return res, ExitSuccess
}

func learnItemVersion(item catalog.Item) string {
	if item.Version != "" {
		return item.Version
	}
	if item.Metadata != nil {
		return item.Metadata["version"]
	}
	return ""
}

// buildLearnedRecord derives install provenance from the catalog item.
func buildLearnedRecord(item catalog.Item, scope, skillDir, version, digest string, files []string) state.SkillRecord {
	origin := state.OriginCatalog
	repo, ref, rawURL := "", "", ""

	switch {
	case strings.HasPrefix(item.Source, "github:"):
		origin = state.OriginGitHub
		if item.Metadata != nil {
			repo = item.Metadata["repo"]
			ref = item.Metadata["ref"]
			rawURL = item.Metadata["canonical"]
		}
	case item.Metadata != nil && (item.Metadata["repo"] != "" || item.Metadata["canonical"] != ""):
		if item.Metadata["repo"] != "" {
			repo = item.Metadata["repo"]
			ref = item.Metadata["ref"]
			origin = state.OriginGitHub
		}
		if item.Metadata["canonical"] != "" {
			rawURL = item.Metadata["canonical"]
		}
	case strings.Contains(item.URL, "raw.githubusercontent.com"):
		origin = state.OriginGitHub
		rawURL = item.URL
	}
	if rawURL == "" && item.URL != "" {
		rawURL = item.URL
	}

	now := time.Now().UTC()
	return state.SkillRecord{
		Name:            item.Name,
		Scope:           scope,
		Path:            skillDir,
		Version:         version,
		Digest:          digest,
		InstalledDigest: digest,
		Status:          state.StatusInstalled,
		InstalledAt:     now,
		UpdatedAt:       now,
		Origin:          origin,
		Repo:            repo,
		Ref:             ref,
		RawURL:          rawURL,
		Catalog:         item.Source,
		Files:           files,
	}
}

// syncedTargetPaths projects a skill and reports the harnesses that accepted it.
func syncedTargetPaths(name, skillDir, syncBaseDir string, targets []harness.Target) []string {
	if len(targets) == 0 || syncBaseDir == "" {
		return nil
	}
	var paths []string
	for _, res := range harness.SyncSkill(name, skillDir, syncBaseDir, targets) {
		if res.Status == "synced" {
			paths = append(paths, fmt.Sprintf("%s:%s", res.Harness, res.Path))
		}
	}
	return paths
}

func (a *App) writeLearnReport(results []installResult, opts learnOptions, scope string) {
	switch a.resolveMode(opts.json, opts.raw) {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
	case OutputModeRaw:
		writeLearnRaw(a.Stdout, results, opts.load)
	default:
		writeLearnHuman(a.Stdout, a.Stderr, results, opts.load, scope)
	}
}

func writeLearnRaw(w interface{ Write([]byte) (int, error) }, results []installResult, load bool) {
	for _, r := range results {
		if r.Status != "installed" {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
				sanitizeTSVField(r.Status),
				sanitizeTSVField(r.Name),
				sanitizeTSVField(r.Message),
			)
			continue
		}
		_, _ = fmt.Fprintf(w, "installed\t%s\t%s\t%s\t%d\n",
			sanitizeTSVField(r.Name),
			sanitizeTSVField(r.Version),
			sanitizeTSVField(r.Path),
			len(r.Files),
		)
		if load && r.Content != "" {
			_, _ = fmt.Fprint(w, r.Content)
			if !strings.HasSuffix(r.Content, "\n") {
				_, _ = fmt.Fprintln(w)
			}
		}
	}
}

func writeLearnHuman(stdout, stderr interface{ Write([]byte) (int, error) }, results []installResult, load bool, scope string) {
	for _, r := range results {
		switch r.Status {
		case "installed":
			_, _ = fmt.Fprintf(stdout, "✓ Learned skill %q into %s scope (%d files in %s)\n", r.Name, scope, len(r.Files), r.Path)
			for _, tp := range r.Targets {
				_, _ = fmt.Fprintf(stdout, "  ↪ Synced to %s\n", tp)
			}
			if load && r.Content != "" {
				_, _ = fmt.Fprintln(stdout, "\n--- Skill Content ---")
				_, _ = fmt.Fprint(stdout, r.Content)
				if !strings.HasSuffix(r.Content, "\n") {
					_, _ = fmt.Fprintln(stdout)
				}
			}
		case "already_installed":
			_, _ = fmt.Fprintf(stderr, "• %s\n", r.Message)
			for _, tp := range r.Targets {
				_, _ = fmt.Fprintf(stdout, "  ↪ Synced to %s\n", tp)
			}
		default:
			_, _ = fmt.Fprintf(stderr, "✗ Failed to learn skill %q: %s\n", r.Name, r.Message)
		}
	}
}

func learnExitCode(errExitCode int, results []installResult) int {
	if errExitCode != ExitSuccess {
		return errExitCode
	}
	for _, r := range results {
		if r.Status == "error" {
			return ExitGeneral
		}
	}
	return ExitSuccess
}
