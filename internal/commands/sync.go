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
	"path/filepath"
	"strings"

	"github.com/jadmadi/dojo/internal/harness"
)

// RunSync projects/synchronizes installed skills across multiple AI coding agent harnesses
// (.claude/skills, .cursor/rules/*.mdc, .windsurf/rules/*.md, .gemini/skills, .opencode/skills, .sila/skills).
func (a *App) RunSync(args []string) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var globalFlag, jsonFlag, rawFlag bool
	var targetFlag string

	fs.BoolVar(&globalFlag, "global", false, "Sync global skills (~/.agents/skills/) across global agent harness directories")
	fs.BoolVar(&globalFlag, "g", false, "Sync global skills (shorthand)")
	fs.StringVar(&targetFlag, "target", "", "Comma-separated harness targets (all, claude, cursor, gemini, windsurf, opencode, sila)")
	fs.StringVar(&targetFlag, "t", "", "Comma-separated harness targets (shorthand)")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return ExitUsage
	}

	mode := a.resolveMode(jsonFlag, rawFlag)

	baseDir, err := os.Getwd()
	if globalFlag {
		baseDir, err = os.UserHomeDir()
	}
	if err != nil || baseDir == "" {
		_, _ = fmt.Fprintf(a.Stderr, "error: unable to resolve base directory: %v\n", err)
		return ExitGeneral
	}

	var targets []harness.Target
	if targetFlag != "" {
		targets, err = harness.ParseTargets(targetFlag, baseDir)
		if err != nil {
			_, _ = fmt.Fprintf(a.Stderr, "error: %v\n", err)
			return ExitUsage
		}
	} else {
		targets = harness.DetectTargets(baseDir)
	}

	discovered := harness.DiscoverInstalledSkillDirs(baseDir)
	var skillNames []string
	if len(posArgs) > 0 {
		for _, name := range posArgs {
			if _, ok := discovered[name]; !ok {
				cand := filepath.Join(baseDir, ".agents", "skills", name)
				if _, statErr := os.Stat(filepath.Join(cand, "SKILL.md")); statErr == nil {
					discovered[name] = cand
				} else {
					_, _ = fmt.Fprintf(a.Stderr, "error: installed skill %q not found in %s/.agents/skills\n", name, baseDir)
					return ExitNotFound
				}
			}
			skillNames = append(skillNames, name)
		}
	} else {
		for name := range discovered {
			skillNames = append(skillNames, name)
		}
	}

	if len(skillNames) == 0 {
		switch mode {
		case OutputModeJSON:
			_ = json.NewEncoder(a.Stdout).Encode([]any{})
		case OutputModeRaw:
		default:
			_, _ = fmt.Fprintf(a.Stdout, "No installed skills found in %s/.agents/skills to sync.\n", baseDir)
		}
		return ExitSuccess
	}

	var allResults []harness.ProjectionResult
	hasError := false
	for _, name := range skillNames {
		res := harness.SyncSkill(name, discovered[name], baseDir, targets)
		for _, r := range res {
			if r.Status == "error" {
				hasError = true
			}
			allResults = append(allResults, r)
		}
	}

	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(allResults)
	case OutputModeRaw:
		for _, r := range allResults {
			_, _ = fmt.Fprintf(a.Stdout, "%s\t%s\t%s\t%s\t%s\n",
				sanitizeTSVField(r.Status),
				sanitizeTSVField(r.Skill),
				sanitizeTSVField(r.Harness),
				sanitizeTSVField(r.Method),
				sanitizeTSVField(r.Path),
			)
		}
	default:
		var targetLabels []string
		for _, t := range targets {
			targetLabels = append(targetLabels, string(t))
		}
		_, _ = fmt.Fprintf(a.Stdout, "MULTI-HARNESS CONTINUITY SYNC (%d skills -> [%s]):\n\n", len(skillNames), strings.Join(targetLabels, ", "))
		for _, r := range allResults {
			if r.Status == "synced" {
				_, _ = fmt.Fprintf(a.Stdout, "  ✓ %-18s -> %-10s (%s: %s)\n", r.Skill, r.Harness, r.Method, r.Path)
			} else {
				_, _ = fmt.Fprintf(a.Stderr, "  ✗ %-18s -> %-10s (%s)\n", r.Skill, r.Harness, r.Message)
			}
		}
	}

	if hasError {
		return ExitGeneral
	}
	return ExitSuccess
}
