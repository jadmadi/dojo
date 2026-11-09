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
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/runner"
	"github.com/jadmadi/dojo/internal/search"
	"github.com/jadmadi/dojo/internal/state"
)

type loadTarget struct {
	item    catalog.Item
	subpath string
}

const DefaultPageSize = 6000
const MaxPageSize = 7000

func paginateContent(content string, pageSize int) []string {
	if pageSize <= 0 || len(content) <= pageSize {
		return []string{content}
	}

	lines := strings.Split(content, "\n")
	var pages []string
	var cur strings.Builder

	for i, line := range lines {
		toAdd := len(line)
		if i > 0 && cur.Len() > 0 {
			toAdd += 2 // account for "\r\n" in PTY line disciplines
		}

		if cur.Len() > 0 && cur.Len()+toAdd > pageSize {
			pages = append(pages, cur.String())
			cur.Reset()
		}

		// Handle lines longer than pageSize without severing UTF-8 runes
		for len(line) > pageSize {
			if cur.Len() > 0 {
				pages = append(pages, cur.String())
				cur.Reset()
			}
			splitIdx := pageSize
			for splitIdx > 0 && !utf8.RuneStart(line[splitIdx]) {
				splitIdx--
			}
			if splitIdx == 0 {
				_, size := utf8.DecodeRuneInString(line)
				splitIdx = size
			}
			pages = append(pages, line[:splitIdx])
			line = line[splitIdx:]
		}

		if cur.Len() > 0 {
			cur.WriteString("\n")
		}
		cur.WriteString(line)
	}

	if cur.Len() > 0 {
		pages = append(pages, cur.String())
	}

	if len(pages) == 0 {
		return []string{content}
	}

	return pages
}

// loadOptions holds the parsed flags for `dojo load`.
type loadOptions struct {
	json     bool
	raw      bool
	local    bool
	update   bool
	force    bool
	full     bool
	withDeps bool
	page     int
	pageSize int
	section  string
}

// loadResultItem is the JSON payload for one load.
type loadResultItem struct {
	Name        string `json:"name"`
	Subpath     string `json:"subpath,omitempty"`
	Section     string `json:"section,omitempty"`
	Category    string `json:"category"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description"`
	Page        int    `json:"page,omitempty"`
	TotalPages  int    `json:"total_pages,omitempty"`
	Content     string `json:"content"`
	Digest      string `json:"digest,omitempty"`
	Source      string `json:"source,omitempty"`
}

func (a *App) RunLoad(args []string) int {
	opts, posArgs, ok := a.parseLoadArgs(args)
	if !ok {
		return ExitUsage
	}
	posArgs = a.resolveSkillSpecs(posArgs)
	mode := a.resolveMode(opts.json, opts.raw)

	if code := a.validateLoadRequest(posArgs, opts); code != ExitSuccess {
		return code
	}

	spec := posArgs[0]
	spec, opts.section = splitLoadSection(a, spec, opts.section)
	skillName, subpath := splitSkillSubpath(spec)

	item, subpath, code := a.resolveLoadItem(spec, skillName, subpath, opts.local)
	if code != ExitSuccess {
		return code
	}
	target := loadTarget{item: *item, subpath: subpath}

	content, matchedSection, code := a.loadTargetContent(target, spec, opts, mode)
	if code != ExitSuccess {
		return code
	}

	pages := loadPages(content, opts)
	if opts.page > len(pages) {
		_, _ = fmt.Fprintf(a.Stderr, "error: page %d out of bounds; %s has %d page(s) (valid pages: 1..%d)\n", opts.page, spec, len(pages), len(pages))
		return ExitUsage
	}

	currentContent := pages[opts.page-1]
	a.recordSkillLoad(target.item.Name, target.subpath, opts.page)

	if mode == OutputModeJSON {
		a.writeLoadJSON(target, matchedSection, currentContent, opts, len(pages))
		return ExitSuccess
	}
	a.writeLoadText(currentContent, spec, opts, len(pages), mode)
	return ExitSuccess
}

func (a *App) parseLoadArgs(args []string) (loadOptions, []string, bool) {
	var opts loadOptions
	fs := flag.NewFlagSet("load", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	fs.BoolVar(&opts.json, "json", false, "Output loaded content as JSON")
	fs.BoolVar(&opts.raw, "raw", false, "Output raw verbatim markdown content without extra formatting")
	fs.BoolVar(&opts.local, "local", false, "Use locally cached or installed copy without checking remote upstream")
	fs.BoolVar(&opts.update, "update", false, "Check remote upstream and update cached copy before loading")
	fs.BoolVar(&opts.update, "u", false, "Check remote upstream and update cached copy before loading (shorthand)")
	fs.BoolVar(&opts.force, "force", false, "Force loading/updating even if digest verification fails")
	fs.BoolVar(&opts.force, "f", false, "Force loading/updating even if digest verification fails (shorthand)")
	fs.IntVar(&opts.page, "page", 1, "Page number to display (1-based)")
	fs.IntVar(&opts.page, "p", 1, "Page number to display (1-based, shorthand)")
	fs.IntVar(&opts.pageSize, "page-size", DefaultPageSize, "Maximum page size in bytes")
	fs.BoolVar(&opts.full, "full", false, "Output complete unpaginated content")
	fs.StringVar(&opts.section, "section", "", "Extract a specific markdown section by heading slug or title")
	fs.StringVar(&opts.section, "s", "", "Extract a specific markdown section (shorthand)")
	fs.BoolVar(&opts.withDeps, "with-deps", false, "Load skill along with all transitive dependencies in topological order")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return opts, nil, false
	}
	return opts, posArgs, true
}

func (a *App) validateLoadRequest(posArgs []string, opts loadOptions) int {
	if len(posArgs) == 0 {
		_, _ = fmt.Fprintln(a.Stderr, "error: load requires a skill name or subpath (e.g. 'dojo load git/reference' or 'dojo load git#workflow')")
		return ExitUsage
	}
	if len(posArgs) > 1 {
		_, _ = fmt.Fprintln(a.Stderr, "error: loading multiple skills simultaneously is disabled to prevent truncation; load skills individually (e.g. 'dojo load <skill>')")
		return ExitUsage
	}
	if opts.page < 1 {
		_, _ = fmt.Fprintf(a.Stderr, "error: invalid page %d: page must be >= 1\n", opts.page)
		return ExitUsage
	}
	if opts.pageSize <= 0 || opts.pageSize > MaxPageSize {
		_, _ = fmt.Fprintf(a.Stderr, "error: invalid page-size %d: page-size must be between 1 and %d bytes (agent buffer safety ceiling)\n", opts.pageSize, MaxPageSize)
		return ExitUsage
	}
	if opts.full && opts.page > 1 {
		_, _ = fmt.Fprintln(a.Stderr, "error: cannot combine --full with -p/--page > 1")
		return ExitUsage
	}
	return ExitSuccess
}

// splitLoadSection separates `skill#section` when the left side is a skill.
func splitLoadSection(a *App, spec, section string) (string, string) {
	idx := strings.Index(spec, "#")
	if idx <= 0 || section != "" {
		return spec, section
	}

	candSpec := spec[:idx]
	candSection := spec[idx+1:]
	if !strings.Contains(candSpec, "/") {
		return candSpec, candSection
	}
	head := strings.SplitN(candSpec, "/", 2)[0]
	if _, err := a.Store.Get(head); err == nil {
		return candSpec, candSection
	}
	return spec, section
}

func splitSkillSubpath(spec string) (string, string) {
	parts := strings.SplitN(spec, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}

func (a *App) resolveLoadItem(spec, skillName, subpath string, localOnly bool) (*catalog.Item, string, int) {
	item, err := a.Store.Get(skillName)
	if err == nil {
		return item, subpath, ExitSuccess
	}
	if !localOnly && shouldProbeGitHubOnLoad(spec, subpath) {
		item, code := a.fetchLoadGitHubItem(spec)
		if code != ExitSuccess {
			return nil, "", code
		}
		if item != nil {
			return item, "", ExitSuccess
		}
	}

	suggestQuery := spec
	if isBundledSubpath(subpath) {
		suggestQuery = skillName
	}
	a.printNotFoundWithSuggestions("load", suggestQuery)
	return nil, "", ExitNotFound
}

// fetchLoadGitHubItem returns (nil, success) when the spec is not a GitHub
// target or the fetch failed in a way the catalog can still answer.
func (a *App) fetchLoadGitHubItem(spec string) (*catalog.Item, int) {
	ghTarget, ok := catalog.ParseGitHubTarget(spec)
	if !ok {
		return nil, ExitSuccess
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	ghItem, err := catalog.FetchGitHubSkill(ctx, ghTarget, a.getHTTPClient())
	cancel()
	if err == nil && ghItem != nil {
		return ghItem, ExitSuccess
	}
	if err != nil {
		if code := MapErrorToExitCode(err); code == ExitNetwork || code == ExitIntegrity {
			_, _ = fmt.Fprintf(a.Stderr, "error: fetching skill from github %q: %v\n", spec, err)
			return nil, code
		}
	}
	return nil, ExitSuccess
}

// loadTargetContent loads either a bundled subpath or the skill body, resolves
// dependencies, and extracts a requested section.
func (a *App) loadTargetContent(target loadTarget, spec string, opts loadOptions, mode OutputMode) (string, string, int) {
	var content string
	var code int
	if target.subpath != "" {
		content, code = a.loadSubpathContent(target, opts.local)
	} else {
		content, code = a.loadSkillBody(target, opts, mode)
	}
	if code != ExitSuccess {
		return "", "", code
	}
	if opts.section == "" {
		return content, "", ExitSuccess
	}
	return a.extractLoadSection(content, opts.section, spec)
}

func (a *App) loadSubpathContent(target loadTarget, localOnly bool) (string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	subpathItem := target.item
	if localOnly {
		subpathItem.URL = ""
		subpathItem.Metadata = nil
	}

	content, err := runner.ReadFileContext(ctx, subpathItem, target.subpath, a.getHTTPClient())
	if err == nil {
		return content, ExitSuccess
	}
	if errors.Is(err, runner.ErrInvalidPath) {
		_, _ = fmt.Fprintf(a.Stderr, "error: invalid path traversal in %q\n", target.subpath)
		return "", ExitUsage
	}
	if errors.Is(err, runner.ErrFileNotFound) {
		return "", a.reportMissingSubpath(target, localOnly)
	}
	_, _ = fmt.Fprintf(a.Stderr, "error: loading %s/%s: %v\n", target.item.Name, target.subpath, err)
	return "", MapErrorToExitCode(err)
}

func (a *App) reportMissingSubpath(target loadTarget, localOnly bool) int {
	if localOnly {
		_, _ = fmt.Fprintf(a.Stderr, "error: subpath %q not found in local cache for skill %q. Run 'dojo load %s/%s' to fetch from remote.\n", target.subpath, target.item.Name, target.item.Name, target.subpath)
		return ExitNotFound
	}

	_, _ = fmt.Fprintf(a.Stderr, "error: subpath %q not found in skill %q.\n", target.subpath, target.item.Name)
	loadSubpathSuggestions(a, target)
	return ExitNotFound
}

func loadSubpathSuggestions(a *App, target loadTarget) []string {
	var candidates []string
	for _, ref := range a.listReferences(target.item) {
		candidates = append(candidates, ref, "references/"+ref)
	}
	for _, refFile := range a.listReferenceFiles(target.item) {
		candidates = append(candidates, refFile, "references/"+refFile)
	}
	for _, scr := range a.listScripts(target.item) {
		candidates = append(candidates, scr, "scripts/"+scr)
	}

	suggs := search.Suggest(target.subpath, uniqueStrings(candidates), 3)
	if len(suggs) == 0 {
		return nil
	}
	_, _ = fmt.Fprintln(a.Stderr, "\nDid you mean:")
	for _, s := range suggs {
		_, _ = fmt.Fprintf(a.Stderr, "  - dojo load %s/%s\n", target.item.Name, s)
	}
	return suggs
}

func (a *App) loadSkillBody(target loadTarget, opts loadOptions, mode OutputMode) (string, int) {
	rawContent, err := a.loadItemContentWithOpts(&target.item, opts.local, opts.update, opts.force)
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: loading skill %q: %v\n", target.item.Name, err)
		if errors.Is(err, catalog.ErrDigestMismatch) {
			writeDigestRemediation(a.Stderr, target.item.Name)
		}
		return "", MapErrorToExitCode(err)
	}

	catalog.EnrichItemFromSkillMD(&target.item, []byte(rawContent))
	if cat, ok := a.Store.(*catalog.Catalog); ok {
		cat.AddItem(target.item)
	}

	if opts.withDeps {
		combined, code := a.loadDependencies(target, rawContent, opts, mode)
		if code != ExitSuccess {
			return "", code
		}
		if combined != "" {
			return combined, ExitSuccess
		}
	}

	if mode == OutputModeRaw {
		return rawContent, ExitSuccess
	}
	return stripFrontmatter(rawContent), ExitSuccess
}

func writeDigestRemediation(w interface{ Write([]byte) (int, error) }, name string) {
	_, _ = fmt.Fprintln(w, "\nDid you mean:")
	_, _ = fmt.Fprintf(w, "  - Load local cached copy:   dojo load %s --local\n", name)
	_, _ = fmt.Fprintf(w, "  - Check upstream update:    dojo load %s --update\n", name)
	_, _ = fmt.Fprintf(w, "  - Ignore digest check:      dojo load %s --force\n", name)
	_, _ = fmt.Fprintf(w, "  - Synchronize catalogs:     dojo update\n")
}

// loadDependencies concatenates the dependency graph when it has more than the
// root skill, returning an empty string otherwise.
func (a *App) loadDependencies(target loadTarget, rawContent string, opts loadOptions, mode OutputMode) (string, int) {
	depItems, err := catalog.ResolveDependencies(target.item.Name, a.Store)
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: resolving dependencies for %q: %v\n", target.item.Name, err)
		return "", MapErrorToExitCode(err)
	}
	if len(depItems) <= 1 {
		return "", ExitSuccess
	}

	var combined strings.Builder
	for i, dep := range depItems {
		depRaw := rawContent
		if !strings.EqualFold(dep.Name, target.item.Name) {
			loaded, loadErr := a.loadItemContentWithOpts(&dep, opts.local, opts.update, opts.force)
			if loadErr != nil {
				_, _ = fmt.Fprintf(a.Stderr, "error: loading dependency %q: %v\n", dep.Name, loadErr)
				return "", MapErrorToExitCode(loadErr)
			}
			depRaw = loaded
		}
		if i > 0 {
			combined.WriteString("\n\n---\n\n")
		}
		if mode == OutputModeRaw {
			combined.WriteString(depRaw)
		} else {
			combined.WriteString(stripFrontmatter(depRaw))
		}
	}
	return combined.String(), ExitSuccess
}

func (a *App) extractLoadSection(content, section, spec string) (string, string, int) {
	sec, err := catalog.ExtractSection(content, section)
	if err != nil || sec == nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: section %q not found in %q.\n", section, spec)
		sections := catalog.ParseSections(content)
		if len(sections) > 0 {
			_, _ = fmt.Fprintln(a.Stderr, "\nAvailable sections:")
			for _, s := range sections {
				_, _ = fmt.Fprintf(a.Stderr, "  - %s#%s (%s, %d bytes)\n", spec, s.Slug, s.Heading, s.Bytes)
			}
		}
		return "", "", ExitNotFound
	}
	return sec.Content, sec.Slug, ExitSuccess
}

func loadPages(content string, opts loadOptions) []string {
	if opts.full || opts.section != "" {
		return []string{content}
	}
	return paginateContent(content, opts.pageSize)
}

func (a *App) writeLoadJSON(target loadTarget, matchedSection, content string, opts loadOptions, totalPages int) {
	ver := target.item.Version
	if ver == "" && target.item.Metadata != nil {
		ver = target.item.Metadata["version"]
	}

	pageNum, totPages := 0, 0
	if totalPages > 1 && !opts.full {
		pageNum = opts.page
		totPages = totalPages
	}

	res := loadResultItem{
		Name:        target.item.Name,
		Subpath:     target.subpath,
		Section:     matchedSection,
		Category:    target.item.Category,
		Version:     ver,
		Description: target.item.Description,
		Page:        pageNum,
		TotalPages:  totPages,
		Content:     content,
		Digest:      target.item.Digest,
		Source:      target.item.Source,
	}

	enc := json.NewEncoder(a.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
}

func (a *App) writeLoadText(content, spec string, opts loadOptions, totalPages int, mode OutputMode) {
	var sb strings.Builder
	if totalPages > 1 && !opts.full && opts.page > 1 && mode != OutputModeRaw {
		_, _ = fmt.Fprintf(&sb, "[Page %d of %d (continued)]\n\n", opts.page, totalPages)
	}

	sb.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		sb.WriteString("\n")
	}

	if totalPages > 1 && !opts.full && mode != OutputModeRaw {
		sb.WriteString("\n---\n")
		if opts.page < totalPages {
			_, _ = fmt.Fprintf(&sb, "[Page %d of %d • Next: dojo load %s -p %d]\n", opts.page, totalPages, spec, opts.page+1)
		} else {
			_, _ = fmt.Fprintf(&sb, "[Page %d of %d • End of document]\n", opts.page, totalPages)
		}
	}

	_, _ = fmt.Fprint(a.Stdout, sb.String())
}

func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var res []string
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			res = append(res, s)
		}
	}
	return res
}

func (a *App) loadItemContentWithOpts(item *catalog.Item, localOnly, forceUpdate, forceDigest bool) (string, error) {
	// 1. If --local is specified, attempt to load from local cache or workspace immediately
	if localOnly {
		// Check workspace installation
		if item.Path != "" {
			skillMD := filepath.Join(item.Path, "SKILL.md")
			if data, err := os.ReadFile(skillMD); err == nil && len(data) > 0 {
				return string(data), nil
			}
			if fi, err := os.Stat(item.Path); err == nil && !fi.IsDir() {
				if data, err := os.ReadFile(item.Path); err == nil && len(data) > 0 {
					return string(data), nil
				}
			}
		}

		// Check local disk cache (<cacheRoot>/skills/<name>/SKILL.md)
		cacheRoot := a.cacheRootDir()
		if cacheRoot == "" {
			return "", fmt.Errorf("load --local: unable to determine cache directory")
		}
		cachedMD := filepath.Join(cacheRoot, "skills", item.Name, "SKILL.md")
		if data, err := os.ReadFile(cachedMD); err == nil && len(data) > 0 {
			return string(data), nil
		}
		return "", fmt.Errorf("skill %q not found in local cache. Run 'dojo load %s' to fetch from remote", item.Name, item.Name)
	}

	// 2. Normal load or update load
	autoUpdate := true
	if a.Config != nil {
		autoUpdate = a.Config.IsAutoUpdate()
	}

	if forceUpdate {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		freshItem := a.refreshCatalogForItem(ctx, item, a.resolveDefaultCatalogURL())
		cancel()
		if freshItem != nil {
			tempItem := *freshItem
			if tempItem.URL == "" && item.URL != "" {
				tempItem.URL = item.URL
			}
			*item = tempItem
		}
	}

	// Try standard loadContent
	content, err := a.loadItemContent(item)
	if err == nil {
		// Save to cache for offline/local access
		if item.Name != "" {
			runner.CacheSkillFile(item.Name, "SKILL.md", []byte(content))
		}
		return content, nil
	}

	// If error is not digest mismatch or remote is not reachable, check if forceDigest is set
	if errors.Is(err, catalog.ErrDigestMismatch) {
		// Case A: Upstream may have updated! Check remote catalog index to see if catalog has a new digest
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		freshItem := a.refreshCatalogForItem(ctx, item, a.resolveDefaultCatalogURL())
		if freshItem != nil {
			// Re-fetch content using the fresh digest
			tempItem := *freshItem
			if tempItem.URL == "" && item.URL != "" {
				tempItem.URL = item.URL
			}
			freshContent, freshContentErr := a.loadItemContent(&tempItem)
			if freshContentErr == nil {
				// The new catalog manifest acknowledges this content!
				if autoUpdate || forceUpdate {
					runner.CacheSkillFile(freshItem.Name, "SKILL.md", []byte(freshContent))
					*item = tempItem

					if !a.Options.NoColor && a.Options.Mode != OutputModeJSON && a.Options.Mode != OutputModeRaw {
						_, _ = fmt.Fprintf(a.Stderr, "✓ Detected upstream update for %q; cache updated.\n", item.Name)
					}
					return freshContent, nil
				}
				return "", fmt.Errorf("%w: upstream content for %q has changed (new digest %s); run 'dojo load %s --update' to update", catalog.ErrDigestMismatch, item.Name, freshItem.Digest, item.Name)
			}
		}

		// If forceDigest was passed, bypass verification without poisoning the local cache
		if forceDigest {
			_, _ = fmt.Fprintf(a.Stderr, "warning: bypassing integrity verification for %q (--force)\n", item.Name)
			// Fetch raw bytes without verify
			remoteURL := item.URL
			if remoteURL == "" && item.Metadata != nil {
				remoteURL = item.Metadata["canonical"]
			}
			if remoteURL != "" {
				reqCtx, reqCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer reqCancel()
				req, reqErr := http.NewRequestWithContext(reqCtx, http.MethodGet, remoteURL, nil)
				if reqErr != nil {
					return "", fmt.Errorf("creating force-refresh request for %s: %w", remoteURL, reqErr)
				}
				resp, rErr := a.getHTTPClient().Do(req)
				if rErr != nil {
					return "", fmt.Errorf("fetching %s (--force): %w", remoteURL, rErr)
				}
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusOK {
					return "", fmt.Errorf("fetching %s (--force): HTTP %d", remoteURL, resp.StatusCode)
				}
				b, readErr := catalog.ReadBounded(resp.Body, catalog.DefaultMaxUnpackedBytes)
				if readErr != nil {
					return "", fmt.Errorf("reading %s (--force): %w", remoteURL, readErr)
				}
				if len(b) > 0 {
					if catalog.IsArchiveItem(*item) || catalog.DetectArchiveFormat(b, remoteURL) != "" {
						md, exErr := catalog.ExtractSkillMDFromArchive(b)
						if exErr != nil {
							return "", fmt.Errorf("extracting SKILL.md from archive %s (--force): %w", remoteURL, exErr)
						}
						return md, nil
					}
					return string(b), nil
				}
			}
		}
	}

	return "", err
}

func (a *App) refreshCatalogForItem(ctx context.Context, item *catalog.Item, defaultCatURL string) *catalog.Item {
	if item == nil {
		return nil
	}

	findInManifest := func(m *catalog.Manifest) *catalog.Item {
		if m == nil {
			return nil
		}
		for _, mItem := range m.Items {
			if strings.EqualFold(mItem.Name, item.Name) || strings.EqualFold(mItem.ID, item.ID) {
				it := mItem
				return &it
			}
		}
		return nil
	}

	// 1. Check if item comes from a registered named catalog (e.g. "dojo" -> github:jadmadi/dojo)
	st := a.loadState()
	candidateCats := []string{
		item.Source,
		strings.TrimPrefix(item.Source, "github:"),
	}
	if os.Getenv("DOJO_URL") == "" && os.Getenv("DOJO_REMOTE_URL") == "" {
		candidateCats = append(candidateCats, item.Name)
	}
	seenCats := make(map[string]bool)
	for _, cand := range candidateCats {
		cand = strings.TrimSpace(cand)
		if cand == "" || cand == "default" || seenCats[cand] {
			continue
		}
		seenCats[cand] = true
		catRec, ok := st.GetCatalog(cand)
		if !ok || catRec.Type == state.CatalogTypeDefault {
			continue
		}
		switch catRec.Type {
		case state.CatalogTypeGitHub:
			if target, okTarget := catalog.ParseGitHubTarget(catRec.Source); okTarget {
				if catRec.Ref != "" {
					target.Ref = catRec.Ref
				}
				if freshManifest, err := catalog.FetchGitHubCatalog(ctx, target, a.getHTTPClient()); err == nil && freshManifest != nil {
					_ = catalog.SaveNamedCatalogCacheIn(a.cacheRootDir(), catRec.Name, catRec.URL, *freshManifest, "")
					if cat, ok := a.Store.(*catalog.Catalog); ok {
						cat.AddCatalogManifest(catRec.Name, freshManifest)
					}
					if found := findInManifest(freshManifest); found != nil {
						return found
					}
				}
			}
		case state.CatalogTypeHTTP:
			if freshManifest, freshETag, err := catalog.FetchHTTPCatalog(ctx, catRec.Source, a.getHTTPClient()); err == nil && freshManifest != nil {
				_ = catalog.SaveNamedCatalogCacheIn(a.cacheRootDir(), catRec.Name, catRec.URL, *freshManifest, freshETag)
				if cat, ok := a.Store.(*catalog.Catalog); ok {
					cat.AddCatalogManifest(catRec.Name, freshManifest)
				}
				if found := findInManifest(freshManifest); found != nil {
					return found
				}
			}
		}
	}

	// 2. Refresh default catalog
	if freshManifest, freshETag, fetchErr := catalog.FetchHTTPCatalog(ctx, defaultCatURL, a.getHTTPClient()); fetchErr == nil && freshManifest != nil {
		if cachePath := a.catalogIndexCachePath(); cachePath != "" {
			_ = catalog.SaveCachedCatalog(cachePath, defaultCatURL, *freshManifest, freshETag)
		}
		if found := findInManifest(freshManifest); found != nil {
			return found
		}
	}

	return nil
}
