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

package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jadmadi/dojo/internal/version"
)

// GitHubTarget captures parsed repository and subpath components.
type GitHubTarget struct {
	Owner       string // e.g. "anthropics"
	Repo        string // e.g. "anthropic-quickstarts"
	Ref         string // e.g. "main", "v1.0.0", "master"
	Subpath     string // e.g. "skills/evals", "evals", ""
	OriginalURL string // e.g. "https://github.com/anthropics/anthropic-quickstarts"
}

// RepoString returns "owner/repo".
func (t *GitHubTarget) RepoString() string {
	return fmt.Sprintf("%s/%s", t.Owner, t.Repo)
}

// FullRepoURL returns "https://github.com/owner/repo".
func (t *GitHubTarget) FullRepoURL() string {
	return fmt.Sprintf("https://github.com/%s/%s", t.Owner, t.Repo)
}

// ParseGitHubTarget parses raw GitHub URLs or shorthand owner/repo notation.
func ParseGitHubTarget(input string) (*GitHubTarget, bool) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return nil, false
	}

	// Remove protocols
	raw = strings.TrimPrefix(raw, "github:")
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "git@github.com:")

	// Handle raw.githubusercontent.com URLs
	if after, ok := strings.CutPrefix(raw, "raw.githubusercontent.com/"); ok {
		raw = after
		raw = strings.Trim(raw, "/")
		parts := strings.Split(raw, "/")
		if len(parts) < 3 {
			return nil, false
		}
		owner := parts[0]
		repo := strings.TrimSuffix(parts[1], ".git")
		ref := parts[2]
		subpath := ""
		if len(parts) > 3 {
			subpath = strings.Join(parts[3:], "/")
		}
		if strings.HasSuffix(strings.ToLower(subpath), "/skill.md") {
			subpath = subpath[:len(subpath)-len("/skill.md")]
		} else if strings.EqualFold(subpath, "skill.md") {
			subpath = ""
		}
		return &GitHubTarget{
			Owner:       owner,
			Repo:        repo,
			Ref:         ref,
			Subpath:     subpath,
			OriginalURL: fmt.Sprintf("https://github.com/%s/%s", owner, repo),
		}, true
	}

	raw = strings.TrimPrefix(raw, "github.com/")

	// Extract ref if passed via @ref or #ref
	ref := "main"
	if idx := strings.IndexAny(raw, "@#"); idx != -1 {
		ref = strings.Trim(raw[idx+1:], "/")
		raw = raw[:idx]
	}

	raw = strings.Trim(raw, "/")
	parts := strings.Split(raw, "/")
	if len(parts) < 2 {
		return nil, false
	}

	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")

	// Validate owner/repo naming (basic sanity check)
	if owner == "" || repo == "" || strings.HasPrefix(owner, "-") || strings.HasPrefix(repo, "-") {
		return nil, false
	}

	subpath := ""
	if len(parts) > 2 {
		remainder := parts[2:]
		// Check for /tree/{ref}/subpath or /blob/{ref}/subpath
		if len(remainder) >= 2 && (remainder[0] == "tree" || remainder[0] == "blob") {
			ref = remainder[1]
			if len(remainder) > 2 {
				subpath = strings.Join(remainder[2:], "/")
			}
		} else {
			subpath = strings.Join(remainder, "/")
		}
	}

	// If subpath points directly to SKILL.md, trim it to directory
	if strings.HasSuffix(strings.ToLower(subpath), "/skill.md") {
		subpath = subpath[:len(subpath)-len("/skill.md")]
	} else if strings.EqualFold(subpath, "skill.md") {
		subpath = ""
	}

	return &GitHubTarget{
		Owner:       owner,
		Repo:        repo,
		Ref:         ref,
		Subpath:     subpath,
		OriginalURL: fmt.Sprintf("https://github.com/%s/%s", owner, repo),
	}, true
}

// FetchGitHubRaw fetches a raw file from a public or private GitHub repository.
// It tries in order:
//  1. Unauthenticated raw.githubusercontent.com (fast path for public repos)
//  2. Authenticated api.github.com/repos/{owner}/{repo}/contents/{path} using GITHUB_TOKEN or GH_TOKEN
//  3. Authenticated `gh api` CLI fallback (supports private repos via ~/.config/gh / system keychain even when GITHUB_TOKEN is unset or stale)
func FetchGitHubRaw(ctx context.Context, owner, repo, ref, relPath string, maxBytes int64, client *http.Client) ([]byte, string, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if maxBytes <= 0 {
		maxBytes = 4 << 20
	}
	cleanRel := strings.TrimLeft(relPath, "/")
	if ref == "" {
		ref = "main"
	}
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, cleanRel)

	// 1. Fast path: unauthenticated raw.githubusercontent.com for public repos
	var lastStatus int
	var lastNetErr error
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil); err == nil {
		req.Header.Set("User-Agent", version.UserAgent())
		if resp, doErr := client.Do(req); doErr == nil {
			lastStatus = resp.StatusCode
			if resp.StatusCode == http.StatusOK {
				data, rErr := ReadBounded(resp.Body, maxBytes)
				_ = resp.Body.Close()
				if rErr == nil && len(data) > 0 {
					return data, rawURL, nil
				}
			} else {
				_ = resp.Body.Close()
			}
		} else {
			lastNetErr = doErr
		}
	}

	// 2. Authenticated GitHub Contents API via GITHUB_TOKEN or GH_TOKEN
	envToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if envToken == "" {
		envToken = strings.TrimSpace(os.Getenv("GH_TOKEN"))
	}
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, cleanRel, ref)
	if envToken != "" {
		if req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil); err == nil {
			req.Header.Set("User-Agent", version.UserAgent())
			req.Header.Set("Accept", "application/vnd.github.raw+json")
			req.Header.Set("Authorization", "Bearer "+envToken)
			if resp, doErr := client.Do(req); doErr == nil {
				if resp.StatusCode == http.StatusOK {
					data, rErr := ReadBounded(resp.Body, maxBytes)
					_ = resp.Body.Close()
					if rErr == nil && len(data) > 0 {
						return data, rawURL, nil
					}
				} else {
					lastStatus = resp.StatusCode
					_ = resp.Body.Close()
				}
			}
		}
	}

	// 3. Fallback to `gh api` CLI (works with private repos when user is logged into gh CLI)
	ghBin, ghErr := exec.LookPath("gh")
	if ghErr != nil {
		if _, statErr := os.Stat("/opt/antigravity/bin/gh"); statErr == nil {
			ghBin = "/opt/antigravity/bin/gh"
			ghErr = nil
		}
	}
	if ghErr == nil && ghBin != "" {
		apiEndpoint := fmt.Sprintf("repos/%s/%s/contents/%s?ref=%s", owner, repo, cleanRel, ref)
		runGH := func(stripEnvToken bool) ([]byte, error) {
			cmd := exec.CommandContext(ctx, ghBin, "api", apiEndpoint, "-H", "Accept: application/vnd.github.raw+json")
			if stripEnvToken {
				var filtered []string
				for _, kv := range os.Environ() {
					if strings.HasPrefix(kv, "GITHUB_TOKEN=") || strings.HasPrefix(kv, "GH_TOKEN=") {
						continue
					}
					filtered = append(filtered, kv)
				}
				cmd.Env = filtered
			}
			return cmd.Output()
		}

		// If envToken already failed in step 2, strip it first so gh uses stored credentials
		if envToken != "" {
			if out, err := runGH(true); err == nil && len(out) > 0 {
				return out, rawURL, nil
			}
		}
		if out, err := runGH(false); err == nil && len(out) > 0 {
			return out, rawURL, nil
		}
	}

	if lastStatus != 0 {
		return nil, rawURL, fmt.Errorf("failed to fetch %s: HTTP %d", rawURL, lastStatus)
	}
	if lastNetErr != nil {
		return nil, rawURL, lastNetErr
	}
	return nil, rawURL, fmt.Errorf("failed to fetch %s", rawURL)
}

// FetchGitHubSkill attempts to fetch and parse SKILL.md from a GitHub repository via raw HTTP or authenticated GitHub API.
func FetchGitHubSkill(ctx context.Context, target *GitHubTarget, client *http.Client) (*Item, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	item, err := probeGitHubRef(ctx, target, target.Ref, client)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("no valid SKILL.md found in github repository %s/%s (ref: %s)", target.Owner, target.Repo, target.Ref)
	}
	return item, nil
}

func probeGitHubRef(ctx context.Context, target *GitHubTarget, ref string, client *http.Client) (*Item, error) {
	var candidatePaths []string

	if target.Subpath != "" {
		candidatePaths = append(candidatePaths,
			target.Subpath+"/SKILL.md",
			target.Subpath+"/skill.md",
			"skills/"+target.Subpath+"/SKILL.md",
			"skills/"+target.Subpath+"/skill.md",
			".agents/skills/"+target.Subpath+"/SKILL.md",
			".agents/skills/"+target.Subpath+"/skill.md",
		)
		if strings.HasSuffix(strings.ToLower(target.Subpath), ".md") {
			candidatePaths = append([]string{target.Subpath}, candidatePaths...)
		}
	} else {
		candidatePaths = append(candidatePaths,
			"SKILL.md",
			"skill.md",
			"skills/"+target.Repo+"/SKILL.md",
			".agents/skills/"+target.Repo+"/SKILL.md",
		)
	}

	var lastNetErr error
	gotHTTPResponse := false
	for _, relPath := range candidatePaths {
		bodyBytes, rawURL, err := FetchGitHubRaw(ctx, target.Owner, target.Repo, ref, relPath, 2<<20, client)
		if err != nil {
			if strings.Contains(err.Error(), "HTTP ") {
				gotHTTPResponse = true
			} else {
				lastNetErr = err
			}
			continue
		}
		gotHTTPResponse = true
		if len(bodyBytes) == 0 {
			continue
		}

		fm, _, err := ParseFrontmatter(bodyBytes)
		if err != nil || fm == nil {
			fm = &Frontmatter{}
		}

		skillName := strings.TrimSpace(fm.Name)
		if skillName == "" {
			if target.Subpath != "" {
				skillName = filepath.Base(target.Subpath)
			} else {
				skillName = target.Repo
			}
		}

		item := Item{
			ID:           skillName,
			Name:         skillName,
			Type:         ItemTypeSkill,
			Description:  fm.Description,
			AllowedTools: fm.AllowedTools,
			License:      fm.License,
			Body:         string(bodyBytes),
			URL:          rawURL,
			RelativePath: relPath,
			Source:       "github",
			Metadata: map[string]string{
				"origin":    target.FullRepoURL(),
				"repo":      target.RepoString(),
				"ref":       ref,
				"canonical": rawURL,
			},
		}

		if fm.Metadata != nil {
			maps.Copy(item.Metadata, fm.Metadata)
		}
		if item.Metadata["version"] != "" {
			item.Version = item.Metadata["version"]
		}
		if item.Metadata["category"] != "" {
			item.Category = item.Metadata["category"]
		}
		if item.Metadata["tags"] != "" {
			item.Tags = ParseTags(item.Metadata["tags"])
		}
		if item.License == "" && fm.License != "" {
			item.License = fm.License
		}

		item.Normalize()
		return &item, nil
	}

	if !gotHTTPResponse && lastNetErr != nil {
		return nil, fmt.Errorf("fetching skill from github %s/%s: %w", target.Owner, target.Repo, lastNetErr)
	}

	return nil, nil
}

type githubTreeResponse struct {
	Sha       string `json:"sha"`
	Truncated bool   `json:"truncated"`
	Tree      []struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
		Type string `json:"type"` // "blob", "tree"
		Sha  string `json:"sha"`
		Size int    `json:"size"`
		URL  string `json:"url"`
	} `json:"tree"`
}

// FetchGitHubCatalog discovers skills across a GitHub repository and returns a Manifest.
// The discovery order is:
// 1. .well-known/agent-skills/index.json (Cloudflare Agent Skills Discovery RFC v0.2.0)
// 2. index.json (google/skills standard)
// 3. catalog.json (legacy backwards-compatibility fallback)
// 4. Directory crawl / Git Tree API (agentskills.io standard)
func FetchGitHubCatalog(ctx context.Context, target *GitHubTarget, client *http.Client) (*Manifest, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	manifest, err := probeGitHubCatalogRef(ctx, target, target.Ref, client)
	if err != nil {
		return nil, err
	}
	if manifest == nil || len(manifest.Items) == 0 {
		return nil, fmt.Errorf("no skills found in github repository %s/%s", target.Owner, target.Repo)
	}
	return manifest, nil
}

func tryFetchManifest(ctx context.Context, target *GitHubTarget, ref, catPath string, client *http.Client) (*Manifest, error) {
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", target.Owner, target.Repo, ref, catPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := ReadBounded(resp.Body, 5<<20) // 5MB limit
	if err != nil || len(data) == 0 {
		return nil, fmt.Errorf("empty or unreadable body: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil || len(m.Items) == 0 {
		return nil, fmt.Errorf("invalid manifest JSON: %w", err)
	}

	for i := range m.Items {
		if m.Items[i].URL != "" && !strings.HasPrefix(m.Items[i].URL, "http://") && !strings.HasPrefix(m.Items[i].URL, "https://") {
			rel := strings.TrimPrefix(strings.TrimPrefix(m.Items[i].URL, "./"), "/")
			m.Items[i].URL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", target.Owner, target.Repo, ref, rel)
		}
		m.Items[i].Normalize()
		if m.Items[i].Source == "" {
			m.Items[i].Source = fmt.Sprintf("github:%s/%s", target.Owner, target.Repo)
		}
	}
	return &m, nil
}

func probeGitHubCatalogRef(ctx context.Context, target *GitHubTarget, ref string, client *http.Client) (*Manifest, error) {
	for _, paths := range manifestCandidateGroups(target.Subpath) {
		if m := firstMatchingManifest(ctx, target, ref, paths, client); m != nil {
			return m, nil
		}
	}
	return discoverTreeManifest(ctx, target, ref, client)
}

// manifestCandidateGroups lists the manifest locations to probe, in priority
// order: RFC well-known, index.json, then legacy catalog.json.
func manifestCandidateGroups(subpath string) [][]string {
	groups := [][]string{
		{".well-known/agent-skills/index.json"},
		{"index.json", "skills/index.json"},
		{"catalog.json", "skills/catalog.json"},
	}
	if subpath != "" {
		trimmed := strings.Trim(subpath, "/")
		groups[0] = append([]string{trimmed + "/.well-known/agent-skills/index.json"}, groups[0]...)
		groups[1] = append([]string{trimmed + "/index.json"}, groups[1]...)
		groups[2] = append([]string{trimmed + "/catalog.json"}, groups[2]...)
	}
	return groups
}

func firstMatchingManifest(ctx context.Context, target *GitHubTarget, ref string, paths []string, client *http.Client) *Manifest {
	for _, p := range paths {
		m, err := tryFetchManifest(ctx, target, ref, p, client)
		if err == nil && m != nil && len(m.Items) > 0 {
			return m
		}
	}
	return nil
}

// discoverTreeManifest crawls the Git tree for SKILL.md files and builds a
// manifest from their frontmatter.
func discoverTreeManifest(ctx context.Context, target *GitHubTarget, ref string, client *http.Client) (*Manifest, error) {
	skillPaths, err := listSkillPaths(ctx, target, ref, client)
	if err != nil {
		return nil, err
	}
	if len(skillPaths) == 0 {
		return nil, fmt.Errorf("no SKILL.md files discovered in %s/%s", target.Owner, target.Repo)
	}

	manifest := &Manifest{
		Version: "1.0.0",
		Items:   make([]Item, 0, len(skillPaths)),
	}
	for _, relPath := range skillPaths {
		if item, ok := fetchTreeSkillItem(ctx, target, ref, relPath, client); ok {
			manifest.Items = append(manifest.Items, item)
		}
	}
	return manifest, nil
}

// listSkillPaths reads the recursive Git tree and returns matching SKILL.md
// paths under the target subpath.
func listSkillPaths(ctx context.Context, target *GitHubTarget, ref string, client *http.Client) ([]string, error) {
	treeURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", target.Owner, target.Repo, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, treeURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching git tree for %s/%s: %w", target.Owner, target.Repo, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d for tree of %s/%s", resp.StatusCode, target.Owner, target.Repo)
	}

	bodyBytes, err := ReadBounded(resp.Body, 10<<20)
	if err != nil {
		return nil, fmt.Errorf("reading tree response: %w", err)
	}

	var treeResp githubTreeResponse
	if err := json.Unmarshal(bodyBytes, &treeResp); err != nil {
		return nil, fmt.Errorf("parsing git tree: %w", err)
	}

	prefix := ""
	if target.Subpath != "" {
		prefix = strings.Trim(target.Subpath, "/") + "/"
	}

	var skillPaths []string
	for _, node := range treeResp.Tree {
		if node.Type != "blob" {
			continue
		}
		lower := strings.ToLower(node.Path)
		if !strings.HasSuffix(lower, "/skill.md") && lower != "skill.md" {
			continue
		}
		if prefix != "" && !strings.HasPrefix(node.Path, prefix) {
			continue
		}
		skillPaths = append(skillPaths, node.Path)
	}
	return skillPaths, nil
}

// fetchTreeSkillItem downloads one SKILL.md and converts it to a manifest item.
func fetchTreeSkillItem(ctx context.Context, target *GitHubTarget, ref, relPath string, client *http.Client) (Item, bool) {
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", target.Owner, target.Repo, ref, relPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Item{}, false
	}
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return Item{}, false
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return Item{}, false
	}

	data, err := ReadBounded(resp.Body, 2<<20)
	_ = resp.Body.Close()
	if err != nil || len(data) == 0 {
		return Item{}, false
	}

	fm, _, _ := ParseFrontmatter(data)
	if fm == nil {
		fm = &Frontmatter{}
	}
	return buildTreeItem(target, ref, relPath, rawURL, data, fm), true
}

func buildTreeItem(target *GitHubTarget, ref, relPath, rawURL string, skillData []byte, fm *Frontmatter) Item {
	dirPath := filepath.Dir(relPath)
	if dirPath == "." {
		dirPath = ""
	}

	skillName := strings.TrimSpace(fm.Name)
	if skillName == "" {
		if dirPath != "" {
			skillName = filepath.Base(dirPath)
		} else {
			skillName = target.Repo
		}
	}

	sum := sha256.Sum256(skillData)
	digest := hex.EncodeToString(sum[:])

	item := Item{
		ID:           skillName,
		Name:         skillName,
		Type:         ItemTypeSkill,
		Description:  fm.Description,
		Category:     treeItemCategory(fm, relPath),
		License:      fm.License,
		AllowedTools: fm.AllowedTools,
		Body:         string(skillData),
		URL:          treeItemURL(target, ref, dirPath),
		RelativePath: relPath,
		Digest:       digest,
		Sha256:       digest,
		Source:       fmt.Sprintf("github:%s/%s", target.Owner, target.Repo),
		Metadata: map[string]string{
			"origin":    target.FullRepoURL(),
			"repo":      target.RepoString(),
			"ref":       ref,
			"canonical": rawURL,
		},
	}
	if fm.Metadata != nil {
		maps.Copy(item.Metadata, fm.Metadata)
		if item.Metadata["version"] != "" {
			item.Version = item.Metadata["version"]
		}
		if item.Metadata["author"] != "" {
			item.Author = item.Metadata["author"]
		}
		if item.Metadata["tags"] != "" {
			item.Tags = ParseTags(item.Metadata["tags"])
		}
	}
	item.Normalize()
	return item
}

func treeItemURL(target *GitHubTarget, ref, dirPath string) string {
	url := fmt.Sprintf("https://github.com/%s/%s/tree/%s", target.Owner, target.Repo, ref)
	if dirPath != "" {
		url += "/" + dirPath
	}
	return url
}

func treeItemCategory(fm *Frontmatter, relPath string) string {
	if fm.Metadata != nil {
		if category := fm.Metadata["category"]; category != "" {
			return category
		}
	}
	parts := strings.Split(relPath, "/")
	if len(parts) >= 3 {
		return parts[len(parts)-3]
	}
	return "agents"
}

// FetchHTTPCatalog fetches a remote catalog manifest via HTTP GET.
// Discovery order:
// 1. /.well-known/agent-skills/index.json (Cloudflare Agent Skills Discovery RFC v0.2.0)
// 2. /index.json (google/skills standard)
// 3. /catalog.json (legacy backwards-compatibility fallback)
func FetchHTTPCatalog(ctx context.Context, rawURL string, client *http.Client) (*Manifest, string, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	var endpointsToTry []string
	if strings.HasSuffix(strings.ToLower(rawURL), ".json") {
		endpointsToTry = []string{rawURL}
	} else {
		base := strings.TrimRight(strings.TrimSpace(rawURL), "/")
		if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
			base = "https://" + base
		}
		endpointsToTry = []string{
			ResolveCatalogURL(base),
			base + "/index.json",
			base + "/catalog.json",
		}
	}

	var lastErr error
	for _, endpoint := range endpointsToTry {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", version.UserAgent())
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("fetching catalog from %s: %w", endpoint, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d fetching catalog from %s", resp.StatusCode, endpoint)
			continue
		}

		etag := resp.Header.Get("ETag")
		data, err := ReadBounded(resp.Body, 10<<20)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading catalog response: %w", err)
			continue
		}

		var manifest Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			lastErr = fmt.Errorf("parsing catalog manifest from %s: %w", endpoint, err)
			continue
		}

		ResolveManifestURLs(&manifest, endpoint)

		return &manifest, etag, nil
	}

	return nil, "", lastErr
}

// DiscoverLocalCatalog discovers skills inside a local filesystem directory.
// Discovery order:
// 1. .well-known/agent-skills/index.json (Cloudflare Agent Skills Discovery RFC v0.2.0)
// 2. index.json (google/skills standard)
// 3. catalog.json (legacy backwards-compatibility fallback)
// 4. Directory crawl (agentskills.io standard)
func DiscoverLocalCatalog(dirPath string) (*Manifest, error) {
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", absDir)
	}

	// 1. .well-known/agent-skills/index.json (Agent Skills Discovery RFC v0.2.0)
	wkPath := filepath.Join(absDir, ".well-known", "agent-skills", "index.json")
	if data, err := os.ReadFile(wkPath); err == nil {
		var m Manifest
		if err := json.Unmarshal(data, &m); err == nil && len(m.Items) > 0 {
			for i := range m.Items {
				m.Items[i].Normalize()
				if m.Items[i].Source == "" {
					m.Items[i].Source = absDir
				}
			}
			return &m, nil
		}
	}

	// 2. index.json (google/skills standard)
	indexPath := filepath.Join(absDir, "index.json")
	if data, err := os.ReadFile(indexPath); err == nil {
		var m Manifest
		if err := json.Unmarshal(data, &m); err == nil && len(m.Items) > 0 {
			for i := range m.Items {
				m.Items[i].Normalize()
				if m.Items[i].Source == "" {
					m.Items[i].Source = absDir
				}
			}
			return &m, nil
		}
	}

	// 3. catalog.json (legacy backwards-compatibility fallback for 2 versions)
	catPath := filepath.Join(absDir, "catalog.json")
	if data, err := os.ReadFile(catPath); err == nil {
		var m Manifest
		if err := json.Unmarshal(data, &m); err == nil && len(m.Items) > 0 {
			for i := range m.Items {
				m.Items[i].Normalize()
				if m.Items[i].Source == "" {
					m.Items[i].Source = absDir
				}
			}
			return &m, nil
		}
	}

	// 4. Directory crawl (agentskills.io standard)
	var items []Item
	err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.EqualFold(info.Name(), "skill.md") {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			fm, _, _ := ParseFrontmatter(content)
			if fm == nil {
				fm = &Frontmatter{}
			}

			skillDir := filepath.Dir(path)
			relDir, _ := filepath.Rel(absDir, skillDir)
			skillName := strings.TrimSpace(fm.Name)
			if skillName == "" {
				skillName = filepath.Base(skillDir)
			}

			sum := sha256.Sum256(content)
			digest := hex.EncodeToString(sum[:])

			category := ""
			if fm.Metadata != nil {
				category = fm.Metadata["category"]
			}
			if category == "" {
				category = "agents"
			}

			item := Item{
				ID:           skillName,
				Name:         skillName,
				Type:         ItemTypeSkill,
				Description:  fm.Description,
				Category:     category,
				License:      fm.License,
				AllowedTools: fm.AllowedTools,
				Body:         string(content),
				Path:         skillDir,
				RelativePath: relDir,
				Digest:       digest,
				Sha256:       digest,
				Source:       absDir,
			}
			if fm.Metadata != nil {
				if item.Metadata == nil {
					item.Metadata = make(map[string]string)
				}
				maps.Copy(item.Metadata, fm.Metadata)
				if item.Metadata["version"] != "" {
					item.Version = item.Metadata["version"]
				}
				if item.Metadata["author"] != "" {
					item.Author = item.Metadata["author"]
				}
				if item.Metadata["tags"] != "" {
					item.Tags = ParseTags(item.Metadata["tags"])
				}
			}
			item.Normalize()
			items = append(items, item)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("discovering local skills in %s: %w", absDir, err)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no SKILL.md files found in %s", absDir)
	}

	return &Manifest{
		Version: "1.0.0",
		Items:   items,
	}, nil
}
