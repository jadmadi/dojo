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
	"github.com/jadmadi/dojo/internal/state"
)

func (a *App) RunCatalog(args []string) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		_, _ = fmt.Fprintln(a.Stdout, `USAGE:
  dojo catalog <subcommand> [flags] [arguments...]

SUBCOMMANDS:
  add <source> [--name <name>] [--ref <ref>] [--subpath <path>] [--refresh] [--json] [--raw]
      Register a skill catalog from a GitHub repository, HTTP manifest, or local directory.
  list [--json] [--raw]
      List all registered skill catalogs and default sources.
  remove <name> [--json] [--raw]
      Remove a registered catalog source.

ALIASES:
  ls (list), rm / delete (remove)`)
		return ExitSuccess
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "add", "install":
		return a.runCatalogAdd(subArgs)
	case "list", "ls":
		return a.runCatalogList(subArgs)
	case "remove", "rm", "delete":
		return a.runCatalogRemove(subArgs)
	default:
		_, _ = fmt.Fprintf(a.Stderr, "error: unknown catalog subcommand %q. Run 'dojo catalog --help' for usage.\n", subcmd)
		return ExitUsage
	}
}

// catalogAddOptions holds the parsed flags for `dojo catalog add`.
type catalogAddOptions struct {
	name    string
	ref     string
	subpath string
	refresh bool
	json    bool
	raw     bool
}

// resolvedCatalog is a fetched catalog plus the metadata saved with it.
type resolvedCatalog struct {
	manifest *catalog.Manifest
	catType  string
	catName  string
	catURL   string
	etag     string
}

func (a *App) runCatalogAdd(args []string) int {
	opts, posArgs, ok := a.parseCatalogAddArgs(args)
	if !ok {
		return ExitUsage
	}
	if len(posArgs) == 0 {
		_, _ = fmt.Fprintln(a.Stderr, "error: catalog add requires a source (GitHub repository 'owner/repo', HTTP URL, or local directory)")
		return ExitUsage
	}

	source := posArgs[0]
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, code := a.resolveCatalogSource(ctx, source, opts)
	if code != ExitSuccess {
		return code
	}
	if res.manifest == nil || len(res.manifest.Items) == 0 {
		_, _ = fmt.Fprintf(a.Stderr, "error: no skills found in catalog %q\n", source)
		return ExitNotFound
	}

	a.persistAddedCatalog(res, source, opts)
	a.writeCatalogAddReport(res, source, opts)
	return ExitSuccess
}

func (a *App) parseCatalogAddArgs(args []string) (catalogAddOptions, []string, bool) {
	opts := catalogAddOptions{ref: "main"}
	fs := flag.NewFlagSet("catalog add", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	fs.StringVar(&opts.name, "name", "", "Custom alias/name for the catalog")
	fs.StringVar(&opts.name, "n", "", "Custom alias/name for the catalog (shorthand)")
	fs.StringVar(&opts.ref, "ref", "main", "Branch or tag ref for git repository")
	fs.StringVar(&opts.ref, "b", "main", "Branch or tag ref (shorthand)")
	fs.StringVar(&opts.ref, "branch", "main", "Branch or tag ref (alias)")
	fs.StringVar(&opts.subpath, "subpath", "", "Subpath directory inside repository")
	fs.StringVar(&opts.subpath, "p", "", "Subpath directory inside repository (shorthand)")
	fs.BoolVar(&opts.refresh, "refresh", false, "Force fresh download bypassing cache")
	fs.BoolVar(&opts.refresh, "r", false, "Force fresh download (shorthand)")
	fs.BoolVar(&opts.json, "json", false, "Output in JSON format")
	fs.BoolVar(&opts.raw, "raw", false, "Output in raw format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return opts, nil, false
	}
	return opts, posArgs, true
}

// resolveCatalogSource dispatches on local directory, HTTP manifest, or GitHub.
func (a *App) resolveCatalogSource(ctx context.Context, source string, opts catalogAddOptions) (resolvedCatalog, int) {
	isExplicitLocalPath := strings.HasPrefix(source, ".") ||
		strings.HasPrefix(source, "/") ||
		strings.HasPrefix(source, "~") ||
		strings.HasPrefix(source, `\`)

	info, statErr := os.Stat(source)
	switch {
	case statErr == nil:
		if !info.IsDir() {
			_, _ = fmt.Fprintf(a.Stderr, "error: local catalog source %q is a file, expected a directory\n", source)
			return resolvedCatalog{}, ExitUsage
		}
		return a.resolveLocalCatalog(source, opts.name)
	case isExplicitLocalPath:
		_, _ = fmt.Fprintf(a.Stderr, "error: local catalog directory %q not found: %v\n", source, statErr)
		return resolvedCatalog{}, MapErrorToExitCode(statErr)
	case isHTTPManifestSource(source):
		return a.resolveHTTPCatalog(ctx, source, opts)
	default:
		return a.resolveGitHubCatalog(ctx, source, opts)
	}
}

func isHTTPManifestSource(source string) bool {
	isHTTP := strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
	if !isHTTP {
		return false
	}
	return !strings.Contains(source, "github.com") || strings.HasSuffix(strings.ToLower(source), ".json")
}

func (a *App) resolveLocalCatalog(source, nameFlag string) (resolvedCatalog, int) {
	absPath, _ := filepath.Abs(source)
	manifest, err := catalog.DiscoverLocalCatalog(source)
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: discovering local catalog %s: %v\n", source, err)
		return resolvedCatalog{}, MapErrorToExitCode(err)
	}

	name := nameFlag
	if name == "" {
		name = filepath.Base(absPath)
	}
	return resolvedCatalog{
		manifest: manifest,
		catType:  state.CatalogTypeLocal,
		catName:  name,
		catURL:   absPath,
	}, ExitSuccess
}

func (a *App) resolveHTTPCatalog(ctx context.Context, source string, opts catalogAddOptions) (resolvedCatalog, int) {
	name := opts.name
	if name == "" {
		name = deriveHTTPCatalogName(source)
	}
	if opts.refresh {
		_ = catalog.DeleteNamedCatalogCacheIn(a.cacheRootDir(), name)
	}

	manifest, etag, err := catalog.FetchHTTPCatalog(ctx, source, a.getHTTPClient())
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: fetching http catalog %s: %v\n", source, err)
		return resolvedCatalog{}, MapErrorToExitCode(err)
	}
	return resolvedCatalog{
		manifest: manifest,
		catType:  state.CatalogTypeHTTP,
		catName:  name,
		catURL:   source,
		etag:     etag,
	}, ExitSuccess
}

func deriveHTTPCatalogName(source string) string {
	name := strings.TrimPrefix(strings.TrimPrefix(source, "https://"), "http://")
	name = strings.TrimSuffix(name, "/.well-known/agent-skills/index.json")
	name = strings.TrimSuffix(name, "/index.json")
	name = strings.TrimSuffix(name, "/catalog.json")
	return strings.TrimSuffix(name, "/")
}

func (a *App) resolveGitHubCatalog(ctx context.Context, source string, opts catalogAddOptions) (resolvedCatalog, int) {
	target, ok := catalog.ParseGitHubTarget(source)
	if !ok {
		_, _ = fmt.Fprintf(a.Stderr, "error: invalid catalog source %q. Expected GitHub repository (e.g. google/skills), HTTP URL, or local directory path.\n", source)
		return resolvedCatalog{}, ExitUsage
	}
	if opts.ref != "main" && opts.ref != "" {
		target.Ref = opts.ref
	}
	if opts.subpath != "" {
		target.Subpath = opts.subpath
	}

	name := opts.name
	if name == "" {
		name = target.RepoString()
		if target.Subpath != "" {
			name += "/" + target.Subpath
		}
	}
	if opts.refresh {
		_ = catalog.DeleteNamedCatalogCacheIn(a.cacheRootDir(), name)
	}

	manifest, err := catalog.FetchGitHubCatalog(ctx, target, a.getHTTPClient())
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: fetching github catalog %s: %v\n", source, err)
		return resolvedCatalog{}, MapErrorToExitCode(err)
	}
	return resolvedCatalog{
		manifest: manifest,
		catType:  state.CatalogTypeGitHub,
		catName:  name,
		catURL:   target.FullRepoURL(),
	}, ExitSuccess
}

// persistAddedCatalog writes the cache and the durable state record.
func (a *App) persistAddedCatalog(res resolvedCatalog, source string, opts catalogAddOptions) {
	if err := catalog.SaveNamedCatalogCacheIn(a.cacheRootDir(), res.catName, res.catURL, *res.manifest, res.etag); err != nil {
		a.logError("saving catalog cache: %v", err)
	}

	now := time.Now().UTC()
	rec := state.CatalogRecord{
		Name:       res.catName,
		Type:       res.catType,
		Source:     source,
		URL:        res.catURL,
		Ref:        opts.ref,
		Subpath:    opts.subpath,
		SkillCount: len(res.manifest.Items),
		AddedAt:    now,
		UpdatedAt:  now,
	}
	st := a.loadState()
	st.AddCatalog(rec)
	a.saveState(st)

	a.Store.AddCatalogManifest(res.catName, res.manifest)
}

func (a *App) writeCatalogAddReport(res resolvedCatalog, source string, opts catalogAddOptions) {
	switch a.resolveMode(opts.json, opts.raw) {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(struct {
			Name       string `json:"name"`
			Type       string `json:"type"`
			Source     string `json:"source"`
			URL        string `json:"url,omitempty"`
			SkillCount int    `json:"skill_count"`
			Status     string `json:"status"`
		}{
			Name:       res.catName,
			Type:       res.catType,
			Source:     source,
			URL:        res.catURL,
			SkillCount: len(res.manifest.Items),
			Status:     "added",
		})
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "added\t%s\t%s\t%d\n",
			sanitizeTSVField(res.catName),
			sanitizeTSVField(res.catType),
			len(res.manifest.Items),
		)
	default:
		_, _ = fmt.Fprintf(a.Stdout, "✓ Added catalog %q (%s, %d skills indexed)\n", res.catName, res.catType, len(res.manifest.Items))
		for _, it := range res.manifest.Items {
			_, _ = fmt.Fprintf(a.Stdout, "  • %-20s [%s] %s\n", it.Name, it.Category, it.Description)
		}
	}
}

func (a *App) runCatalogList(args []string) int {
	fs := flag.NewFlagSet("catalog list", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var jsonFlag, rawFlag bool
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw format")

	if _, err := parseFlagsWithArgs(fs, args); err != nil {
		return ExitUsage
	}

	mode := a.resolveMode(jsonFlag, rawFlag)
	st := a.loadState()
	registered := st.GetCatalogs()

	type CatalogView struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Source     string `json:"source"`
		URL        string `json:"url,omitempty"`
		SkillCount int    `json:"skill_count"`
		UpdatedAt  string `json:"updated_at,omitempty"`
	}

	var views []CatalogView

	defaultURL := a.resolveDefaultCatalogURL()

	for _, c := range registered {
		src := c.Source
		if src == "" {
			src = c.URL
		}
		if c.Name == "default" {
			src = defaultURL
			// Count skills in default catalog
			defSkills := 0
			if all := a.Store.All(); len(all) > 0 {
				for _, it := range all {
					if it.Source == "remote" || it.Source == "" || strings.Contains(it.Source, "jadmadi/dojo") || it.Source == "default" {
						defSkills++
					}
				}
			}
			if defSkills > 0 {
				c.SkillCount = defSkills
			}
		}

		upStr := ""
		if !c.UpdatedAt.IsZero() {
			upStr = c.UpdatedAt.Format("2006-01-02 15:04")
		}
		views = append(views, CatalogView{
			Name:       c.Name,
			Type:       c.Type,
			Source:     src,
			URL:        c.URL,
			SkillCount: c.SkillCount,
			UpdatedAt:  upStr,
		})
	}

	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(views)
		return ExitSuccess
	case OutputModeRaw:
		for _, v := range views {
			_, _ = fmt.Fprintf(a.Stdout, "%s\t%s\t%s\t%d\t%s\n",
				sanitizeTSVField(v.Name),
				sanitizeTSVField(v.Type),
				sanitizeTSVField(v.Source),
				v.SkillCount,
				sanitizeTSVField(v.UpdatedAt),
			)
		}
		return ExitSuccess
	default:
		nameW, typeW, srcW, skillsW, updatedW := 22, 10, 42, 8, 19
		for _, v := range views {
			nameW = max(nameW, len(v.Name))
			typeW = max(typeW, len(v.Type))
			dispSrc := formatDisplaySource(v.Source)
			srcW = max(srcW, len(dispSrc))
			skillsStr := fmt.Sprintf("%d", v.SkillCount)
			skillsW = max(skillsW, len(skillsStr))
			up := v.UpdatedAt
			if up == "" {
				up = "built-in"
			}
			updatedW = max(updatedW, len(up))
		}

		_, _ = fmt.Fprintf(a.Stdout, "CONFIGURED CATALOGS (%d total):\n\n", len(views))
		_, _ = fmt.Fprintf(a.Stdout, "%-*s %-*s %-*s %-*s %s\n", nameW, "NAME", typeW, "TYPE", srcW, "SOURCE", skillsW, "SKILLS", "UPDATED")
		_, _ = fmt.Fprintf(a.Stdout, "%s %s %s %s %s\n",
			strings.Repeat("-", nameW),
			strings.Repeat("-", typeW),
			strings.Repeat("-", srcW),
			strings.Repeat("-", skillsW),
			strings.Repeat("-", updatedW),
		)
		for _, v := range views {
			up := v.UpdatedAt
			if up == "" {
				up = "built-in"
			}
			dispSrc := formatDisplaySource(v.Source)
			_, _ = fmt.Fprintf(a.Stdout, "%-*s %-*s %-*s %-*d %s\n", nameW, v.Name, typeW, v.Type, srcW, dispSrc, skillsW, v.SkillCount, up)
		}
		return ExitSuccess
	}
}

func formatDisplaySource(src string) string {
	src = strings.TrimPrefix(src, "https://")
	src = strings.TrimPrefix(src, "http://")
	return src
}

func (a *App) runCatalogRemove(args []string) int {
	fs := flag.NewFlagSet("catalog remove", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var jsonFlag, rawFlag bool
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return ExitUsage
	}
	if len(posArgs) == 0 {
		_, _ = fmt.Fprintln(a.Stderr, "error: catalog remove requires catalog name(s)")
		return ExitUsage
	}

	mode := a.resolveMode(jsonFlag, rawFlag)
	st := a.loadState()
	var results []struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
	}

	hasNotFound := false
	hasError := false
	for _, name := range posArgs {
		if strings.EqualFold(name, "default") {
			hasError = true
			results = append(results, struct {
				Name    string `json:"name"`
				Status  string `json:"status"`
				Message string `json:"message,omitempty"`
			}{
				Name:    name,
				Status:  "error",
				Message: "cannot remove default catalog",
			})
			continue
		}

		if st.RemoveCatalog(name) {
			_ = catalog.DeleteNamedCatalogCacheIn(a.cacheRootDir(), name)
			a.Store.RemoveCatalogItems(name)
			results = append(results, struct {
				Name    string `json:"name"`
				Status  string `json:"status"`
				Message string `json:"message,omitempty"`
			}{
				Name:   name,
				Status: "removed",
			})
		} else {
			hasNotFound = true
			results = append(results, struct {
				Name    string `json:"name"`
				Status  string `json:"status"`
				Message string `json:"message,omitempty"`
			}{
				Name:    name,
				Status:  "not_found",
				Message: "catalog not found",
			})
		}
	}

	a.saveState(st)

	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
		if hasError {
			return ExitUsage
		}
		if hasNotFound {
			return ExitNotFound
		}
		return ExitSuccess
	case OutputModeRaw:
		for _, r := range results {
			_, _ = fmt.Fprintf(a.Stdout, "%s\t%s\t%s\n",
				sanitizeTSVField(r.Status),
				sanitizeTSVField(r.Name),
				sanitizeTSVField(r.Message),
			)
		}
		if hasError {
			return ExitUsage
		}
		if hasNotFound {
			return ExitNotFound
		}
		return ExitSuccess
	default:
		for _, r := range results {
			switch r.Status {
			case "removed":
				_, _ = fmt.Fprintf(a.Stdout, "✓ Removed catalog %q\n", r.Name)
			case "not_found":
				_, _ = fmt.Fprintf(a.Stderr, "⚠ Catalog %q not found.\n", r.Name)
			default:
				_, _ = fmt.Fprintf(a.Stderr, "✗ Failed to remove catalog %q: %s\n", r.Name, r.Message)
			}
		}
		if hasError {
			return ExitUsage
		}
		if hasNotFound {
			return ExitNotFound
		}
		return ExitSuccess
	}
}

func (a *App) syncRegisteredCatalogs(ctx context.Context, st *state.State, mode OutputMode) int {
	targets := st.GetCatalogs()

	type SyncResult struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		SkillCount int    `json:"skill_count"`
		Status     string `json:"status"`
		Message    string `json:"message,omitempty"`
	}

	var results []SyncResult
	errExitCode := ExitSuccess
	for _, cat := range targets {
		var m *catalog.Manifest
		var etag string
		var err error

		switch cat.Type {
		case state.CatalogTypeDefault:
			defaultURL := a.resolveDefaultCatalogURL()
			cat.Source = defaultURL
			cat.URL = defaultURL
			m, etag, err = catalog.FetchHTTPCatalog(ctx, defaultURL, a.getHTTPClient())
			if err == nil && m != nil {
				// Save to index.json cache
				cachePath := a.catalogIndexCachePath()
				_ = catalog.SaveCachedCatalog(cachePath, defaultURL, *m, etag)
				if syncer, ok := a.Store.(interface {
					Sync(context.Context, bool) error
				}); ok {
					_ = syncer.Sync(ctx, true)
				}
			}
		case state.CatalogTypeGitHub:
			target, ok := catalog.ParseGitHubTarget(cat.Source)
			if ok {
				if cat.Ref != "" {
					target.Ref = cat.Ref
				}
				if cat.Subpath != "" {
					target.Subpath = cat.Subpath
				}
				m, err = catalog.FetchGitHubCatalog(ctx, target, a.getHTTPClient())
			}
		case state.CatalogTypeHTTP:
			m, etag, err = catalog.FetchHTTPCatalog(ctx, cat.Source, a.getHTTPClient())
		case state.CatalogTypeLocal:
			m, err = catalog.DiscoverLocalCatalog(cat.Source)
		}

		if err != nil || m == nil {
			if err != nil {
				if code := MapErrorToExitCode(err); errExitCode == ExitSuccess || errExitCode == ExitGeneral {
					errExitCode = code
				}
			}
			results = append(results, SyncResult{
				Name:    cat.Name,
				Type:    cat.Type,
				Status:  "error",
				Message: fmt.Sprintf("%v", err),
			})
			continue
		}

		if cat.Type != state.CatalogTypeDefault {
			_ = catalog.SaveNamedCatalogCacheIn(a.cacheRootDir(), cat.Name, cat.URL, *m, etag)
			a.Store.AddCatalogManifest(cat.Name, m)
		}
		cat.SkillCount = len(m.Items)
		cat.UpdatedAt = time.Now().UTC()
		st.AddCatalog(cat)

		results = append(results, SyncResult{
			Name:       cat.Name,
			Type:       cat.Type,
			SkillCount: len(m.Items),
			Status:     "synced",
		})
	}

	a.saveState(st)

	hasError := false
	for _, r := range results {
		if r.Status == "error" {
			hasError = true
			break
		}
	}

	exitCode := ExitSuccess
	if hasError {
		if errExitCode != ExitSuccess {
			exitCode = errExitCode
		} else {
			exitCode = ExitGeneral
		}
	}

	a.pruneSkillCache(!hasError)

	if mode == OutputModeHuman && len(results) > 0 {
		_, _ = fmt.Fprintf(a.Stdout, "Synchronized %d catalogs:\n", len(results))
		for _, r := range results {
			if r.Status == "synced" {
				_, _ = fmt.Fprintf(a.Stdout, "  ✓ %-22s (%s, %d skills indexed)\n", r.Name, r.Type, r.SkillCount)
			} else {
				_, _ = fmt.Fprintf(a.Stderr, "  ✗ %-22s (%s: %s)\n", r.Name, r.Type, r.Message)
			}
		}
		_, _ = fmt.Fprintln(a.Stdout)
	}

	if err := a.syncTelemetryIfEnabled(ctx, mode); err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: %v\n", err)
		if exitCode == ExitSuccess {
			exitCode = MapErrorToExitCode(err)
		}
	}

	return exitCode
}

// pruneSkillCache cleans stale or orphaned entries under <cacheRoot>/skills/.
// If removeOrphans is true (all registered catalogs synced cleanly), directories
// not present in the active catalog store are removed.
func (a *App) pruneSkillCache(removeOrphans bool) {
	cacheRoot := a.cacheRootDir()
	if cacheRoot == "" || a.Store == nil {
		return
	}
	skillsDir := filepath.Join(cacheRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return
	}

	activeByName := make(map[string]catalog.Item)
	for _, it := range a.Store.All() {
		if it.Name != "" {
			// Prefer archive or default catalog entry when duplicates exist across catalogs
			if prev, exists := activeByName[it.Name]; !exists || catalog.IsArchiveItem(it) || prev.Type == "" {
				activeByName[it.Name] = it
			}
		}
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		skillCachePath := filepath.Join(skillsDir, name)
		it, exists := activeByName[name]
		if !exists {
			if removeOrphans {
				_ = os.RemoveAll(skillCachePath)
			}
			continue
		}

		if catalog.IsArchiveItem(it) {
			expected := it.EffectiveDigest()
			if expected != "" && !catalog.IsArchiveCachedWithDigest(cacheRoot, name, expected) {
				_ = os.RemoveAll(skillCachePath)
			}
			continue
		}

		if it.Type == catalog.ItemTypeSkillMD {
			subEntries, sErr := os.ReadDir(skillCachePath)
			if sErr == nil {
				for _, sub := range subEntries {
					if sub.Name() != "SKILL.md" {
						_ = os.RemoveAll(filepath.Join(skillCachePath, sub.Name()))
					}
				}
			}
		}
	}
}
