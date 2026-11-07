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

package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/harness"
	"github.com/jadmadi/dojo/internal/state"
	"github.com/jadmadi/dojo/internal/version"
)

const (
	DefaultManifestFilename = "skills.json"
	DefaultSchemaURL        = "https://raw.githubusercontent.com/jadmadi/dojo/main/.well-known/agent-skills/skills-manifest.schema.json"
)

// SkillEntry represents a single declarative skill entry in skills.json.
type SkillEntry struct {
	Name    string `json:"name"`
	Catalog string `json:"catalog,omitempty"`
	Repo    string `json:"repo,omitempty"`
	Ref     string `json:"ref,omitempty"`
	Version string `json:"version,omitempty"`
}

// SkillsManifest represents a portable, Git-friendly skills.json manifest
// for traveling across local machines, workspaces, and cloud VMs (e.g. agy-vm).
type SkillsManifest struct {
	Schema     string            `json:"$schema,omitempty"`
	Version    string            `json:"version"`
	Harnesses  []string          `json:"harnesses,omitempty"`
	Targets    []string          `json:"targets,omitempty"`
	Catalogs   map[string]string `json:"catalogs,omitempty"`
	Skills     []SkillEntry      `json:"skills"`
	SourceRepo string            `json:"-"`
	SourceRef  string            `json:"-"`
}

// EffectiveHarnesses returns Harnesses if non-empty, or falls back to Targets.
func (m *SkillsManifest) EffectiveHarnesses() []string {
	if len(m.Harnesses) > 0 {
		return m.Harnesses
	}
	return m.Targets
}

// BuildManifest constructs a SkillsManifest from the active state and discovered skills in rootDir.
func BuildManifest(st *state.State, scope, rootDir string, targets []harness.Target) *SkillsManifest {
	m := &SkillsManifest{
		Schema:   DefaultSchemaURL,
		Version:  "1.0.0",
		Catalogs: make(map[string]string),
		Skills:   []SkillEntry{},
	}

	seenTargets := make(map[string]bool)
	for _, t := range targets {
		name := string(t)
		if name == "gemini" {
			name = "antigravity"
		}
		if !seenTargets[name] {
			seenTargets[name] = true
			m.Harnesses = append(m.Harnesses, name)
		}
	}

	if st != nil {
		for _, c := range st.Catalogs {
			if c.Name == "default" || c.Name == "dojo" || c.Name == "kungfu" || c.Source == "" {
				continue
			}
			m.Catalogs[c.Name] = c.Source
		}
	}

	entriesByName := make(map[string]SkillEntry)
	if st != nil {
		var ws *state.WorkspaceState
		if scope == state.ScopeGlobal {
			ws = st.Global
		} else if st.Workspaces != nil {
			ws = st.Workspaces[rootDir]
		}
		if ws != nil {
			for _, rec := range ws.Skills {
				if rec.Status != state.StatusInstalled {
					continue
				}
				entriesByName[rec.Name] = SkillEntry{
					Name:    rec.Name,
					Catalog: rec.Catalog,
					Repo:    rec.Repo,
					Ref:     rec.Ref,
					Version: rec.Version,
				}
			}
		}
	}

	// Also include any skills discovered on disk in rootDir/.agents/skills
	discovered := harness.DiscoverInstalledSkillDirs(rootDir)
	for name, dir := range discovered {
		if _, exists := entriesByName[name]; !exists {
			ver := ""
			if data, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err == nil {
				if fm, _, fErr := catalog.ParseFrontmatter(data); fErr == nil && fm != nil && fm.Metadata != nil {
					ver = fm.Metadata["version"]
				}
			}
			entriesByName[name] = SkillEntry{
				Name:    name,
				Version: ver,
			}
		}
	}

	var names []string
	for name := range entriesByName {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		m.Skills = append(m.Skills, entriesByName[name])
	}

	if len(m.Catalogs) == 0 {
		m.Catalogs = nil
	}
	return m
}

// SaveManifest writes a SkillsManifest as formatted JSON to path.
func SaveManifest(path string, m *SkillsManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

// LoadManifest loads a SkillsManifest from a local file path, directory, HTTP URL, or GitHub repository ref.
// Supports both public and private GitHub repositories (via GITHUB_TOKEN, GH_TOKEN, or `gh api`).
func LoadManifest(ctx context.Context, pathOrRef string, client *http.Client) (*SkillsManifest, error) {
	clean := strings.TrimSpace(pathOrRef)
	if clean == "" {
		clean = DefaultManifestFilename
	}

	// 1. Check local filesystem first
	if info, err := os.Stat(clean); err == nil {
		targetPath := clean
		sourceDir := filepath.Dir(clean)
		if info.IsDir() {
			targetPath = filepath.Join(clean, DefaultManifestFilename)
			sourceDir = clean
		}
		data, rErr := os.ReadFile(targetPath)
		if rErr != nil {
			return nil, rErr
		}
		var m SkillsManifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("invalid skills.json in %s: %w", targetPath, err)
		}
		if absDir, aErr := filepath.Abs(sourceDir); aErr == nil {
			m.SourceRepo = absDir
		} else {
			m.SourceRepo = sourceDir
		}
		return &m, nil
	}

	// 2. Check GitHub repository target (github:owner/repo, owner/repo, or raw.githubusercontent.com)
	if !strings.HasPrefix(clean, "http://") && !strings.HasPrefix(clean, "https://") || strings.Contains(clean, "github.com/") || strings.Contains(clean, "raw.githubusercontent.com/") {
		if target, ok := catalog.ParseGitHubTarget(strings.TrimPrefix(clean, "github:")); ok {
			ref := target.Ref
			if ref == "" {
				ref = "main"
			}
			sub := strings.Trim(target.Subpath, "/")
			if sub == "" {
				sub = DefaultManifestFilename
			} else if !strings.HasSuffix(sub, ".json") {
				sub = sub + "/" + DefaultManifestFilename
			}
			data, rawURL, err := catalog.FetchGitHubRaw(ctx, target.Owner, target.Repo, ref, sub, 4<<20, client)
			if err != nil {
				return nil, err
			}
			var m SkillsManifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, fmt.Errorf("invalid remote skills.json from %s: %w", rawURL, err)
			}
			m.SourceRepo = target.RepoString()
			m.SourceRef = ref
			return &m, nil
		}
	}

	// 3. Generic HTTP/HTTPS URL
	if strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://") {
		remoteURL := clean
		if !strings.HasSuffix(remoteURL, ".json") {
			remoteURL = strings.TrimRight(remoteURL, "/") + "/" + DefaultManifestFilename
		}
		if client == nil {
			client = http.DefaultClient
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
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
			return nil, fmt.Errorf("failed to fetch %s: HTTP %d", remoteURL, resp.StatusCode)
		}
		data, err := catalog.ReadBounded(resp.Body, 4<<20)
		if err != nil {
			return nil, err
		}
		var m SkillsManifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("invalid remote skills.json from %s: %w", remoteURL, err)
		}
		return &m, nil
	}

	return nil, fmt.Errorf("skills manifest %q not found on disk and is not a valid URL or GitHub repository", pathOrRef)
}
