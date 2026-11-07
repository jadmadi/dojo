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

	"github.com/jadmadi/dojo/internal/harness"
	"github.com/jadmadi/dojo/internal/profile"
	"github.com/jadmadi/dojo/internal/state"
)

// RunInit scaffolds a new Agent Skill package with compliant SKILL.md frontmatter,
// or initializes a multi-harness workspace (--workspace / -w).
func (a *App) RunInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var dirFlag, categoryFlag, descFlag string
	var workspaceFlag, forceFlag, jsonFlag, rawFlag bool

	fs.BoolVar(&workspaceFlag, "workspace", false, "Initialize current project workspace (.agents/skills, skills.json, and IDE harness projections)")
	fs.BoolVar(&workspaceFlag, "w", false, "Initialize current project workspace (shorthand)")
	fs.StringVar(&dirFlag, "dir", "", "Target parent directory (default: .agents/skills)")
	fs.StringVar(&dirFlag, "d", "", "Target parent directory (shorthand)")
	fs.StringVar(&categoryFlag, "category", "coding", "Skill category")
	fs.StringVar(&categoryFlag, "c", "coding", "Skill category (shorthand)")
	fs.StringVar(&descFlag, "description", "", "Skill description")
	fs.BoolVar(&forceFlag, "force", false, "Overwrite existing files")
	fs.BoolVar(&forceFlag, "f", false, "Overwrite existing files (shorthand)")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return ExitUsage
	}
	if workspaceFlag || (len(posArgs) == 1 && posArgs[0] == "workspace") {
		return a.runWorkspaceInit(dirFlag, jsonFlag, rawFlag)
	}
	if len(posArgs) == 0 {
		_, _ = fmt.Fprintln(a.Stderr, "error: init requires a skill name (e.g. 'dojo init my-skill') or --workspace")
		return ExitUsage
	}

	rawName := strings.TrimSpace(posArgs[0])
	cleanName := filepath.Base(filepath.Clean(rawName))
	if cleanName == "" || cleanName == "." || cleanName == ".." || strings.ContainsAny(cleanName, `/\`) {
		_, _ = fmt.Fprintf(a.Stderr, "error: invalid skill name %q\n", rawName)
		return ExitUsage
	}

	parentDir := dirFlag
	if parentDir == "" {
		wd, _ := os.Getwd()
		parentDir = filepath.Join(wd, ".agents", "skills")
	}
	skillDir := filepath.Join(parentDir, cleanName)
	skillMDPath := filepath.Join(skillDir, "SKILL.md")

	if _, err := os.Stat(skillMDPath); err == nil && !forceFlag {
		_, _ = fmt.Fprintf(a.Stderr, "error: skill %q already exists at %s (pass -f/--force to overwrite)\n", cleanName, skillDir)
		return ExitGeneral
	}

	if descFlag == "" {
		descFlag = fmt.Sprintf("Specialized workflow and domain instructions for %s.", cleanName)
	}

	for _, sub := range []string{"scripts", "references", "evals"} {
		if err := os.MkdirAll(filepath.Join(skillDir, sub), 0755); err != nil {
			_, _ = fmt.Fprintf(a.Stderr, "error: creating directory %s: %v\n", sub, err)
			return ExitGeneral
		}
	}

	title := strings.ReplaceAll(cleanName, "-", " ")
	title = strings.ToUpper(title[:1]) + title[1:]

	skillMDContent := fmt.Sprintf(`---
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
`, cleanName, descFlag, categoryFlag, cleanName, title, descFlag, cleanName, cleanName)

	refContent := fmt.Sprintf("# %s Reference Guide\n\nDetailed reference notes and examples for `%s`.\n", title, cleanName)
	scriptContent := "#!/usr/bin/env bash\nset -euo pipefail\necho \"[" + cleanName + "] check passed\"\n"
	evalsContent := fmt.Sprintf(`{
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

	filesToCreate := map[string]struct {
		content string
		perm    os.FileMode
	}{
		"SKILL.md":            {skillMDContent, 0644},
		"references/guide.md": {refContent, 0644},
		"scripts/check.sh":    {scriptContent, 0755},
		"evals/evals.json":    {evalsContent, 0644},
	}

	var createdFiles []string
	for rel, spec := range filesToCreate {
		target := filepath.Join(skillDir, filepath.FromSlash(rel))
		if err := os.WriteFile(target, []byte(spec.content), spec.perm); err != nil {
			_, _ = fmt.Fprintf(a.Stderr, "error: writing %s: %v\n", target, err)
			return ExitGeneral
		}
		createdFiles = append(createdFiles, rel)
	}

	mode := a.resolveMode(jsonFlag, rawFlag)
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
			Files:  createdFiles,
		}
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "initialized\t%s\t%s\t%d\n",
			sanitizeTSVField(cleanName),
			sanitizeTSVField(skillDir),
			len(createdFiles),
		)
	default:
		_, _ = fmt.Fprintf(a.Stdout, "✓ Initialized skill %q at %s\n", cleanName, skillDir)
		_, _ = fmt.Fprintln(a.Stdout, "  • SKILL.md")
		_, _ = fmt.Fprintln(a.Stdout, "  • references/guide.md")
		_, _ = fmt.Fprintln(a.Stdout, "  • scripts/check.sh")
		_, _ = fmt.Fprintln(a.Stdout, "  • evals/evals.json")
	}
	return ExitSuccess
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

