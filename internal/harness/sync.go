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

package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jadmadi/dojo/internal/catalog"
)

// Target represents a supported AI coding agent harness target.
type Target string

const (
	TargetAgents   Target = "agents"   // .agents/skills/<name>/SKILL.md (Universal standard)
	TargetClaude   Target = "claude"   // .claude/skills/<name>/SKILL.md (Claude Code)
	TargetCursor   Target = "cursor"   // .cursor/rules/<name>.mdc (Cursor)
	TargetWindsurf Target = "windsurf" // .windsurf/rules/<name>.md (Windsurf)
	TargetGemini   Target = "gemini"   // .gemini/skills/<name>/SKILL.md or .antigravity/skills (Antigravity Desktop & CLI)
	TargetOpenCode Target = "opencode" // .opencode/skills/<name>/SKILL.md (OpenCode)
	TargetSila     Target = "sila"     // .sila/skills/<name>/SKILL.md (Sila)
)

// AllTargets lists all supported agent harnesses in deterministic order.
var AllTargets = []Target{
	TargetAgents,
	TargetClaude,
	TargetCursor,
	TargetWindsurf,
	TargetGemini,
	TargetOpenCode,
	TargetSila,
}

// ProjectionResult records the result of syncing a skill to a specific harness.
type ProjectionResult struct {
	Skill   string `json:"skill"`
	Harness string `json:"harness"`
	Path    string `json:"path"`
	Method  string `json:"method"` // "symlink", "copy", "mdc"
	Status  string `json:"status"` // "synced", "error"
	Message string `json:"message,omitempty"`
}

// ParseTargets parses a comma-separated list of harness names (or "all" / "auto").
func ParseTargets(spec string, rootDir string) ([]Target, error) {
	clean := strings.ToLower(strings.TrimSpace(spec))
	if clean == "" || clean == "auto" {
		return DetectTargets(rootDir), nil
	}
	if clean == "all" {
		return append([]Target(nil), AllTargets...), nil
	}

	parts := strings.Split(clean, ",")
	seen := make(map[Target]bool)
	var out []Target
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var t Target
		switch p {
		case "agents", "universal":
			t = TargetAgents
		case "claude", "ccode", "claude-code":
			t = TargetClaude
		case "cursor":
			t = TargetCursor
		case "windsurf", "codeium":
			t = TargetWindsurf
		case "gemini", "agy", "antigravity":
			t = TargetGemini
		case "opencode", "oc":
			t = TargetOpenCode
		case "sila":
			t = TargetSila
		case "all":
			return append([]Target(nil), AllTargets...), nil
		default:
			return nil, fmt.Errorf("unknown harness target %q (valid: all, auto, agents, claude, cursor, windsurf, gemini, opencode, sila)", p)
		}
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out, nil
}

// DetectTargets detects which agent harnesses are active in rootDir, always including agents, claude, and gemini if present.
func DetectTargets(rootDir string) []Target {
	seen := map[Target]bool{
		TargetAgents: true,
	}
	checks := []struct {
		dir    string
		target Target
	}{
		{".claude", TargetClaude},
		{"CLAUDE.md", TargetClaude},
		{".cursor", TargetCursor},
		{".cursorrules", TargetCursor},
		{".windsurf", TargetWindsurf},
		{".windsurfrules", TargetWindsurf},
		{".gemini", TargetGemini},
		{"GEMINI.md", TargetGemini},
		{".antigravity", TargetGemini},
		{"ANTIGRAVITY.md", TargetGemini},
		{".opencode", TargetOpenCode},
		{".sila", TargetSila},
	}

	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(rootDir, c.dir)); err == nil {
			seen[c.target] = true
		}
	}

	// If only .agents was detected, default to syncing claude, cursor, and gemini as well for frictionless multi-harness support
	if len(seen) == 1 {
		seen[TargetClaude] = true
		seen[TargetCursor] = true
		seen[TargetGemini] = true
	}

	var out []Target
	for _, t := range AllTargets {
		if seen[t] {
			out = append(out, t)
		}
	}
	return out
}

// ConvertToCursorMDC converts a standard SKILL.md document into Cursor's .mdc rule format.
func ConvertToCursorMDC(skillName string, skillMD []byte) []byte {
	fm, body, err := catalog.ParseFrontmatter(skillMD)
	desc := fmt.Sprintf("Agent skill %s", skillName)
	if err == nil && fm != nil && fm.Description != "" {
		desc = strings.ReplaceAll(fm.Description, "\n", " ")
	} else {
		body = string(skillMD)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("description: %q\n", desc))
	sb.WriteString("globs:\n")
	sb.WriteString("alwaysApply: false\n")
	sb.WriteString("---\n\n")
	sb.WriteString(strings.TrimLeft(body, "\r\n"))
	if !strings.HasSuffix(body, "\n") {
		sb.WriteString("\n")
	}
	return []byte(sb.String())
}

// SyncSkill projects an installed skill from its canonical sourceDir (e.g. .agents/skills/<name>)
// into the specified target harnesses under rootDir.
func SyncSkill(skillName, sourceDir, rootDir string, targets []Target) []ProjectionResult {
	var results []ProjectionResult
	skillMDPath := filepath.Join(sourceDir, "SKILL.md")
	skillBytes, err := os.ReadFile(skillMDPath)
	if err != nil {
		return []ProjectionResult{{
			Skill:   skillName,
			Harness: "source",
			Path:    skillMDPath,
			Status:  "error",
			Message: fmt.Sprintf("reading canonical SKILL.md: %v", err),
		}}
	}

	for _, target := range targets {
		switch target {
		case TargetAgents:
			destDir := filepath.Join(rootDir, ".agents", "skills", skillName)
			if filepath.Clean(destDir) == filepath.Clean(sourceDir) {
				results = append(results, ProjectionResult{
					Skill:   skillName,
					Harness: string(target),
					Path:    destDir,
					Method:  "canonical",
					Status:  "synced",
				})
				continue
			}
			res := projectSkillDirectory(skillName, string(target), sourceDir, destDir)
			results = append(results, res)

		case TargetClaude:
			destDir := filepath.Join(rootDir, ".claude", "skills", skillName)
			results = append(results, projectSkillDirectory(skillName, string(target), sourceDir, destDir))

		case TargetGemini:
			destDir := filepath.Join(rootDir, ".gemini", "skills", skillName)
			if _, aErr := os.Stat(filepath.Join(rootDir, ".antigravity")); aErr == nil {
				destDir = filepath.Join(rootDir, ".antigravity", "skills", skillName)
			}
			results = append(results, projectSkillDirectory(skillName, string(target), sourceDir, destDir))

		case TargetOpenCode:
			destDir := filepath.Join(rootDir, ".opencode", "skills", skillName)
			results = append(results, projectSkillDirectory(skillName, string(target), sourceDir, destDir))

		case TargetSila:
			destDir := filepath.Join(rootDir, ".sila", "skills", skillName)
			results = append(results, projectSkillDirectory(skillName, string(target), sourceDir, destDir))

		case TargetCursor:
			rulesDir := filepath.Join(rootDir, ".cursor", "rules")
			destFile := filepath.Join(rulesDir, skillName+".mdc")
			if mkErr := os.MkdirAll(rulesDir, 0755); mkErr != nil {
				results = append(results, ProjectionResult{
					Skill:   skillName,
					Harness: string(target),
					Path:    destFile,
					Status:  "error",
					Message: mkErr.Error(),
				})
				continue
			}
			mdcBytes := ConvertToCursorMDC(skillName, skillBytes)
			if wErr := os.WriteFile(destFile, mdcBytes, 0644); wErr != nil {
				results = append(results, ProjectionResult{
					Skill:   skillName,
					Harness: string(target),
					Path:    destFile,
					Status:  "error",
					Message: wErr.Error(),
				})
				continue
			}
			results = append(results, ProjectionResult{
				Skill:   skillName,
				Harness: string(target),
				Path:    destFile,
				Method:  "mdc",
				Status:  "synced",
			})

		case TargetWindsurf:
			rulesDir := filepath.Join(rootDir, ".windsurf", "rules")
			destFile := filepath.Join(rulesDir, skillName+".md")
			if mkErr := os.MkdirAll(rulesDir, 0755); mkErr != nil {
				results = append(results, ProjectionResult{
					Skill:   skillName,
					Harness: string(target),
					Path:    destFile,
					Status:  "error",
					Message: mkErr.Error(),
				})
				continue
			}
			if wErr := os.WriteFile(destFile, skillBytes, 0644); wErr != nil {
				results = append(results, ProjectionResult{
					Skill:   skillName,
					Harness: string(target),
					Path:    destFile,
					Status:  "error",
					Message: wErr.Error(),
				})
				continue
			}
			results = append(results, ProjectionResult{
				Skill:   skillName,
				Harness: string(target),
				Path:    destFile,
				Method:  "copy",
				Status:  "synced",
			})
		}
	}

	return results
}

func projectSkillDirectory(skillName, harnessName, sourceDir, destDir string) ProjectionResult {
	parentDir := filepath.Dir(destDir)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return ProjectionResult{
			Skill:   skillName,
			Harness: harnessName,
			Path:    destDir,
			Status:  "error",
			Message: err.Error(),
		}
	}

	// Remove existing symlink or directory at destDir
	_ = os.RemoveAll(destDir)

	// Try relative symlink first for zero-duplication sync
	relTarget, relErr := filepath.Rel(parentDir, sourceDir)
	if relErr == nil {
		if err := os.Symlink(relTarget, destDir); err == nil {
			return ProjectionResult{
				Skill:   skillName,
				Harness: harnessName,
				Path:    destDir,
				Method:  "symlink",
				Status:  "synced",
			}
		}
	}

	// Fallback to recursive copy (e.g. across filesystems or Windows without symlink privs)
	if err := copyDirRecursive(sourceDir, destDir); err != nil {
		return ProjectionResult{
			Skill:   skillName,
			Harness: harnessName,
			Path:    destDir,
			Status:  "error",
			Message: err.Error(),
		}
	}

	return ProjectionResult{
		Skill:   skillName,
		Harness: harnessName,
		Path:    destDir,
		Method:  "copy",
		Status:  "synced",
	}
}

func copyDirRecursive(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		perm := os.FileMode(0644)
		if err == nil && info.Mode().Perm() != 0 {
			perm = info.Mode().Perm()
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, perm)
	})
}

// DiscoverInstalledSkillDirs finds all skill directories installed in rootDir/.agents/skills.
func DiscoverInstalledSkillDirs(rootDir string) map[string]string {
	out := make(map[string]string)
	skillsBase := filepath.Join(rootDir, ".agents", "skills")
	entries, err := os.ReadDir(skillsBase)
	if err != nil {
		return out
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		dir := filepath.Join(skillsBase, name)
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
			out[name] = dir
		}
	}
	return out
}

// ConvertFromCursorMDC converts a Cursor .mdc rule file back into canonical SKILL.md format,
// preserving existing canonical YAML frontmatter fields when present.
func ConvertFromCursorMDC(skillName string, mdcBytes []byte, existingCanonical []byte) []byte {
	mdcFM, mdcBody, mdcHasFM, mdcErr := catalog.ExtractFrontmatter(mdcBytes)
	body := string(mdcBytes)
	desc := fmt.Sprintf("Agent skill %s", skillName)
	if mdcErr == nil && mdcHasFM {
		body = string(mdcBody)
		if parsed, _, pErr := catalog.ParseFrontmatter(mdcBytes); pErr == nil && parsed != nil && parsed.Description != "" {
			desc = strings.TrimSpace(parsed.Description)
		}
	}

	if len(existingCanonical) > 0 {
		canFM, _, canHasFM, canErr := catalog.ExtractFrontmatter(existingCanonical)
		if canErr == nil && canHasFM && len(canFM) > 0 {
			var sb strings.Builder
			sb.WriteString("---\n")
			sb.WriteString(strings.TrimRight(string(canFM), "\r\n"))
			sb.WriteString("\n---\n\n")
			sb.WriteString(strings.TrimLeft(body, "\r\n"))
			if !strings.HasSuffix(body, "\n") {
				sb.WriteString("\n")
			}
			return []byte(sb.String())
		}
	}

	_ = mdcFM
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", skillName))
	sb.WriteString(fmt.Sprintf("description: %q\n", desc))
	sb.WriteString("---\n\n")
	sb.WriteString(strings.TrimLeft(body, "\r\n"))
	if !strings.HasSuffix(body, "\n") {
		sb.WriteString("\n")
	}
	return []byte(sb.String())
}

// ReconcileHarnesses performs bi-directional reconciliation from active harness directories
// (.antigravity/skills, .gemini/skills, .claude/skills, .opencode/skills, .sila/skills,
// .cursor/rules/*.mdc, .windsurf/rules/*.md) back into the canonical .agents/skills/<name> store.
func ReconcileHarnesses(rootDir string) []ProjectionResult {
	var reconciled []ProjectionResult
	canonicalBase := filepath.Join(rootDir, ".agents", "skills")

	// 1. Directory-based harnesses
	dirSources := []struct {
		harness string
		relPath string
	}{
		{"antigravity", filepath.Join(".antigravity", "skills")},
		{"gemini", filepath.Join(".gemini", "skills")},
		{"claude", filepath.Join(".claude", "skills")},
		{"opencode", filepath.Join(".opencode", "skills")},
		{"sila", filepath.Join(".sila", "skills")},
	}

	for _, src := range dirSources {
		hBase := filepath.Join(rootDir, src.relPath)
		entries, err := os.ReadDir(hBase)
		if err != nil {
			continue
		}
		for _, e := range entries {
			skillName := e.Name()
			hSkillDir := filepath.Join(hBase, skillName)
			hSkillMD := filepath.Join(hSkillDir, "SKILL.md")
			hInfo, err := os.Stat(hSkillMD)
			if err != nil {
				continue
			}

			canSkillDir := filepath.Join(canonicalBase, skillName)
			canSkillMD := filepath.Join(canSkillDir, "SKILL.md")
			canInfo, canErr := os.Stat(canSkillMD)

			if canErr != nil {
				// Orphan skill created directly in a harness -> import into canonical .agents/skills
				if err := copyDirRecursive(hSkillDir, canSkillDir); err == nil {
					reconciled = append(reconciled, ProjectionResult{
						Skill:   skillName,
						Harness: "agents",
						Path:    canSkillDir,
						Method:  "import-from-" + src.harness,
						Status:  "synced",
					})
				}
				continue
			}

			// If hSkillDir is a symlink pointing to canSkillDir, they are already identical
			if linfo, lerr := os.Lstat(hSkillDir); lerr == nil && (linfo.Mode()&os.ModeSymlink != 0) {
				continue
			}

			// Copy mode: check if harness SKILL.md was modified more recently than canonical SKILL.md
			if hInfo.ModTime().After(canInfo.ModTime()) {
				hBytes, rErr1 := os.ReadFile(hSkillMD)
				cBytes, rErr2 := os.ReadFile(canSkillMD)
				if rErr1 == nil && rErr2 == nil && string(hBytes) != string(cBytes) {
					if err := copyDirRecursive(hSkillDir, canSkillDir); err == nil {
						reconciled = append(reconciled, ProjectionResult{
							Skill:   skillName,
							Harness: "agents",
							Path:    canSkillDir,
							Method:  "reconcile-from-" + src.harness,
							Status:  "synced",
						})
					}
				}
			}
		}
	}

	// 2. Cursor .mdc rules (.cursor/rules/<name>.mdc)
	cursorRulesDir := filepath.Join(rootDir, ".cursor", "rules")
	if entries, err := os.ReadDir(cursorRulesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".mdc") {
				continue
			}
			skillName := strings.TrimSuffix(e.Name(), ".mdc")
			if skillName == "" {
				continue
			}
			mdcPath := filepath.Join(cursorRulesDir, e.Name())
			mdcInfo, sErr := os.Stat(mdcPath)
			if sErr != nil {
				continue
			}
			mdcBytes, rErr := os.ReadFile(mdcPath)
			if rErr != nil {
				continue
			}

			canSkillDir := filepath.Join(canonicalBase, skillName)
			canSkillMD := filepath.Join(canSkillDir, "SKILL.md")
			canInfo, canErr := os.Stat(canSkillMD)

			if canErr != nil {
				// Brand-new .mdc rule created in Cursor -> import to canonical SKILL.md
				if mkErr := os.MkdirAll(canSkillDir, 0755); mkErr == nil {
					converted := ConvertFromCursorMDC(skillName, mdcBytes, nil)
					if wErr := os.WriteFile(canSkillMD, converted, 0644); wErr == nil {
						reconciled = append(reconciled, ProjectionResult{
							Skill:   skillName,
							Harness: "agents",
							Path:    canSkillMD,
							Method:  "import-from-cursor",
							Status:  "synced",
						})
					}
				}
				continue
			}

			if mdcInfo.ModTime().After(canInfo.ModTime()) {
				canBytes, cErr := os.ReadFile(canSkillMD)
				if cErr == nil {
					// Compare bodies to see if the Cursor rule was genuinely edited
					_, mdcBody, _, _ := catalog.ExtractFrontmatter(mdcBytes)
					_, canBody, canHasFM, _ := catalog.ExtractFrontmatter(canBytes)
					if !canHasFM {
						canBody = canBytes
					}
					if strings.TrimSpace(string(mdcBody)) != strings.TrimSpace(string(canBody)) {
						converted := ConvertFromCursorMDC(skillName, mdcBytes, canBytes)
						if wErr := os.WriteFile(canSkillMD, converted, 0644); wErr == nil {
							reconciled = append(reconciled, ProjectionResult{
								Skill:   skillName,
								Harness: "agents",
								Path:    canSkillMD,
								Method:  "reconcile-from-cursor",
								Status:  "synced",
							})
						}
					}
				}
			}
		}
	}

	// 3. Windsurf .md rules (.windsurf/rules/<name>.md)
	wsRulesDir := filepath.Join(rootDir, ".windsurf", "rules")
	if entries, err := os.ReadDir(wsRulesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			skillName := strings.TrimSuffix(e.Name(), ".md")
			if skillName == "" {
				continue
			}
			wsPath := filepath.Join(wsRulesDir, e.Name())
			wsInfo, sErr := os.Stat(wsPath)
			if sErr != nil {
				continue
			}
			wsBytes, rErr := os.ReadFile(wsPath)
			if rErr != nil {
				continue
			}

			canSkillDir := filepath.Join(canonicalBase, skillName)
			canSkillMD := filepath.Join(canSkillDir, "SKILL.md")
			canInfo, canErr := os.Stat(canSkillMD)

			if canErr != nil {
				if mkErr := os.MkdirAll(canSkillDir, 0755); mkErr == nil {
					contentToWrite := wsBytes
					if _, _, hasFM, _ := catalog.ExtractFrontmatter(wsBytes); !hasFM {
						contentToWrite = ConvertFromCursorMDC(skillName, wsBytes, nil)
					}
					if wErr := os.WriteFile(canSkillMD, contentToWrite, 0644); wErr == nil {
						reconciled = append(reconciled, ProjectionResult{
							Skill:   skillName,
							Harness: "agents",
							Path:    canSkillMD,
							Method:  "import-from-windsurf",
							Status:  "synced",
						})
					}
				}
				continue
			}

			if wsInfo.ModTime().After(canInfo.ModTime()) {
				canBytes, cErr := os.ReadFile(canSkillMD)
				if cErr == nil && strings.TrimSpace(string(wsBytes)) != strings.TrimSpace(string(canBytes)) {
					if wErr := os.WriteFile(canSkillMD, wsBytes, 0644); wErr == nil {
						reconciled = append(reconciled, ProjectionResult{
							Skill:   skillName,
							Harness: "agents",
							Path:    canSkillMD,
							Method:  "reconcile-from-windsurf",
							Status:  "synced",
						})
					}
				}
			}
		}
	}

	return reconciled
}

