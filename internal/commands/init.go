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
	"regexp"
	"strings"

	"github.com/jadmadi/dojo/internal/harness"
	"github.com/jadmadi/dojo/internal/profile"
	"github.com/jadmadi/dojo/internal/state"
)

// initOptions holds the parsed flags for `dojo init`.
type initOptions struct {
	dir         string
	category    string
	description string
	workspace   bool
	force       bool
	json        bool
	raw         bool
}

// scaffoldFile is one generated file in a new skill package.
type scaffoldFile struct {
	rel     string
	content string
	perm    os.FileMode
}

var validSkillNameRegex = regexp.MustCompile(`^[a-z0-9]+([-_][a-z0-9]+)*$`)

// RunInit scaffolds a new Agent Skill package with compliant SKILL.md frontmatter,
// or initializes a multi-harness workspace (--workspace / -w).
func (a *App) RunInit(args []string) int {
	opts, posArgs, ok := a.parseInitArgs(args)
	if !ok {
		return ExitUsage
	}
	if opts.workspace || (len(posArgs) == 1 && posArgs[0] == "workspace") {
		return a.runWorkspaceInit(opts.dir, opts.json, opts.raw)
	}
	if len(posArgs) == 0 {
		_, _ = fmt.Fprintln(a.Stderr, "error: init requires a skill name (e.g. 'dojo init my-skill') or --workspace")
		return ExitUsage
	}

	cleanName, ok := a.validateSkillName(posArgs[0])
	if !ok {
		return ExitUsage
	}

	skillDir := filepath.Join(initParentDir(opts.dir), cleanName)
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillMDPath); err == nil && !opts.force {
		_, _ = fmt.Fprintf(a.Stderr, "error: skill %q already exists at %s (pass -f/--force to overwrite)\n", cleanName, skillDir)
		return ExitGeneral
	}

	files := scaffoldFiles(cleanName, opts.description, opts.category)
	if code := writeScaffold(skillDir, files); code != ExitSuccess {
		return code
	}
	writeInitReport(a.Stdout, a.resolveMode(opts.json, opts.raw), cleanName, skillDir, files)
	return ExitSuccess
}

func (a *App) parseInitArgs(args []string) (initOptions, []string, bool) {
	opts := initOptions{category: "coding"}
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	fs.BoolVar(&opts.workspace, "workspace", false, "Initialize current project workspace (.agents/skills, skills.json, and IDE harness projections)")
	fs.BoolVar(&opts.workspace, "w", false, "Initialize current project workspace (shorthand)")
	fs.StringVar(&opts.dir, "dir", "", "Target parent directory (default: .agents/skills)")
	fs.StringVar(&opts.dir, "d", "", "Target parent directory (shorthand)")
	fs.StringVar(&opts.category, "category", "coding", "Skill category")
	fs.StringVar(&opts.category, "c", "coding", "Skill category (shorthand)")
	fs.StringVar(&opts.description, "description", "", "Skill description")
	fs.BoolVar(&opts.force, "force", false, "Overwrite existing files")
	fs.BoolVar(&opts.force, "f", false, "Overwrite existing files (shorthand)")
	fs.BoolVar(&opts.json, "json", false, "Output in JSON format")
	fs.BoolVar(&opts.raw, "raw", false, "Output in raw tab-separated format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return opts, nil, false
	}
	return opts, posArgs, true
}

// validateSkillName normalizes and validates the requested skill name.
func (a *App) validateSkillName(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	cleanName := filepath.Base(filepath.Clean(trimmed))
	if cleanName == "" || cleanName == "." || cleanName == ".." || strings.ContainsAny(cleanName, `/\`) {
		_, _ = fmt.Fprintf(a.Stderr, "error: invalid skill name %q\n", trimmed)
		return "", false
	}
	if !validSkillNameRegex.MatchString(cleanName) {
		_, _ = fmt.Fprintf(a.Stderr, "error: invalid skill name %q: skill names must consist of lowercase alphanumeric characters and hyphens (e.g. 'my-custom-skill')\n", trimmed)
		return "", false
	}
	return cleanName, true
}

func initParentDir(dirFlag string) string {
	if dirFlag != "" {
		return dirFlag
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".agents", "skills")
}

// scaffoldFiles builds the skill package contents in a stable order.
func scaffoldFiles(cleanName, description, category string) []scaffoldFile {
	if description == "" {
		description = fmt.Sprintf("Specialized workflow and domain instructions for %s.", cleanName)
	}
	title := strings.ReplaceAll(cleanName, "-", " ")
	title = strings.ToUpper(title[:1]) + title[1:]

	return []scaffoldFile{
		{rel: "SKILL.md", content: scaffoldSkillMD(cleanName, title, description, category), perm: 0644},
		{rel: "references/guide.md", content: scaffoldReference(title, cleanName), perm: 0644},
		{rel: "scripts/check.sh", content: scaffoldScript(cleanName), perm: 0755},
		{rel: "evals/evals.json", content: scaffoldEvals(cleanName), perm: 0644},
	}
}

func scaffoldSkillMD(cleanName, title, description, category string) string {
	return fmt.Sprintf(`---
name: %s
description: %s
license: Apache-2.0
metadata:
  version: "0.1.0"
  category: %s
  tags: "%s, dojo"
---

# %s

%s

## When to Use

- Use this skill when working with %s tasks or workflows.

## Quick Start

1. Inspect the workspace context before making changes.
2. Follow the guidelines in `+"`references/guide.md`"+`.
3. Run helper scripts via `+"`dojo run %s check.sh`"+`.
`, cleanName, description, category, cleanName, title, description, cleanName, cleanName)
}

func scaffoldReference(title, cleanName string) string {
	return fmt.Sprintf("# %s Reference Guide\n\nDetailed reference notes and examples for `%s`.\n", title, cleanName)
}

func scaffoldScript(cleanName string) string {
	return "#!/usr/bin/env bash\nset -euo pipefail\necho \"[" + cleanName + "] check passed\"\n"
}

func scaffoldEvals(cleanName string) string {
	return fmt.Sprintf(`{
  "skill": %q,
  "evals": [
    {
      "id": "basic-usage",
      "prompt": "Apply the %s skill workflow to the current project.",
      "assertions": [
        "Follows Quick Start steps from SKILL.md"
      ]
    }
  ]
}
`, cleanName, cleanName)
}

// writeScaffold creates the directories and files for a new skill.
func writeScaffold(skillDir string, files []scaffoldFile) int {
	for _, sub := range []string{"scripts", "references", "evals"} {
		if err := os.MkdirAll(filepath.Join(skillDir, sub), 0755); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: creating directory %s: %v\n", sub, err)
			return ExitGeneral
		}
	}

	for _, f := range files {
		target := filepath.Join(skillDir, filepath.FromSlash(f.rel))
		if err := os.WriteFile(target, []byte(f.content), f.perm); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: writing %s: %v\n", target, err)
			return ExitGeneral
		}
	}
	return ExitSuccess
}

func writeInitReport(w interface{ Write([]byte) (int, error) }, mode OutputMode, cleanName, skillDir string, files []scaffoldFile) {
	switch mode {
	case OutputModeJSON:
		res := struct {
			Name   string   `json:"name"`
			Path   string   `json:"path"`
			Status string   `json:"status"`
			Files  []string `json:"files"`
		}{
			Name:   cleanName,
			Path:   skillDir,
			Status: "initialized",
			Files:  scaffoldRelPaths(files),
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	case OutputModeRaw:
		_, _ = fmt.Fprintf(w, "initialized\t%s\t%s\t%d\n",
			sanitizeTSVField(cleanName),
			sanitizeTSVField(skillDir),
			len(files),
		)
	default:
		_, _ = fmt.Fprintf(w, "✓ Initialized skill %q at %s\n", cleanName, skillDir)
		for _, rel := range scaffoldRelPaths(files) {
			_, _ = fmt.Fprintf(w, "  • %s\n", rel)
		}
	}
}

func scaffoldRelPaths(files []scaffoldFile) []string {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.rel)
	}
	return paths
}

func (a *App) runWorkspaceInit(dirFlag string, jsonFlag, rawFlag bool) int {
	wsDir := dirFlag
	if wsDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintf(a.Stderr, "error: resolving workspace directory: %v\n", err)
			return ExitGeneral
		}
		wsDir = wd
	}

	canonicalDir := filepath.Join(wsDir, ".agents", "skills")
	if err := os.MkdirAll(canonicalDir, 0755); err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: creating %s: %v\n", canonicalDir, err)
		return ExitGeneral
	}

	// 1. Reconcile any existing rules/skills from active harnesses into .agents/skills
	reconciled := harness.ReconcileHarnesses(wsDir)

	// 2. If skills.json already exists in the workspace, apply it automatically
	manifestPath := filepath.Join(wsDir, profile.DefaultManifestFilename)
	var appliedSkills []string
	var projections []harness.ProjectionResult
	projections = append(projections, reconciled...)

	if existingManifest, err := profile.LoadManifest(context.Background(), manifestPath, a.getHTTPClient()); err == nil && existingManifest != nil {
		installed, proj, _ := a.applySkillsManifest(existingManifest, false)
		appliedSkills = append(appliedSkills, installed...)
		projections = append(projections, proj...)
	}

	// 3. Detect active harnesses and export/update skills.json
	targets := harness.DetectTargets(wsDir)
	st := a.loadState()
	manifest := profile.BuildManifest(st, state.ScopeWorkspace, wsDir, targets)
	if err := profile.SaveManifest(manifestPath, manifest); err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: writing %s: %v\n", manifestPath, err)
		return ExitGeneral
	}

	// 4. Ensure all discovered canonical skills are projected across detected harnesses
	discovered := harness.DiscoverInstalledSkillDirs(wsDir)
	for name, dir := range discovered {
		projections = append(projections, harness.SyncSkill(name, dir, wsDir, targets)...)
	}

	mode := a.resolveMode(jsonFlag, rawFlag)
	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"status":         "workspace_initialized",
			"workspace":      wsDir,
			"canonical_dir":  canonicalDir,
			"manifest_path":  manifestPath,
			"harnesses":      manifest.Harnesses,
			"skills":         len(manifest.Skills),
			"applied_skills": appliedSkills,
			"projections":    len(projections),
		})
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "workspace_initialized\t%s\t%s\t%d\n",
			sanitizeTSVField(wsDir),
			sanitizeTSVField(manifestPath),
			len(manifest.Skills),
		)
	default:
		_, _ = fmt.Fprintf(a.Stdout, "✓ Initialized Dojo workspace in %s\n", wsDir)
		_, _ = fmt.Fprintf(a.Stdout, "  • Canonical store: %s\n", canonicalDir)
		_, _ = fmt.Fprintf(a.Stdout, "  • Traveling manifest: %s (%d skills)\n", manifestPath, len(manifest.Skills))
		_, _ = fmt.Fprintf(a.Stdout, "  • Active harnesses: %s\n", strings.Join(manifest.Harnesses, ", "))
	}
	return ExitSuccess
}
