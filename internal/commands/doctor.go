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
	"regexp"
	"sort"
	"strings"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/config"
	"github.com/jadmadi/dojo/internal/harness"
	"github.com/jadmadi/dojo/internal/state"
)

// DoctorFinding represents a single diagnostic or security finding from dojo doctor.
type DoctorFinding struct {
	Category string `json:"category"` // "environment", "skill", "security", "budget"
	Skill    string `json:"skill,omitempty"`
	Severity string `json:"severity"` // "ok", "warn", "error"
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
}

var securityPatterns = []struct {
	name  string
	regex *regexp.Regexp
}{
	{"remote pipe to shell (curl/wget | sh/bash)", regexp.MustCompile(`(?i)\b(curl|wget)\b[^\n|]*\|\s*(sudo\s+)?(sh|bash|zsh)\b`)},
	{"base64 decode execution", regexp.MustCompile(`(?i)base64\s+(-d|--decode)[^\n|]*\|\s*(sh|bash|python|perl|node)`)},
	{"prompt injection / instruction override", regexp.MustCompile(`(?i)(ignore\s+(all\s+)?(previous|prior)\s+instructions|disregard\s+system\s+prompt|you\s+are\s+now\s+in\s+developer\s+mode)`)},
	{"suspicious credential exfiltration", regexp.MustCompile(`(?i)(curl|wget)[^\n]*(\$\{?(AWS_SECRET|GITHUB_TOKEN|OPENAI_API_KEY|ANTHROPIC_API_KEY)|\.ssh/id_rsa|\.env\b)`)},
}

// RunDoctor audits environment health, installed skill frontmatter compliance, token budgets,
// and suspicious prompt-injection or shell-execution patterns.
func (a *App) RunDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var skillsFlag, securityFlag, globalFlag, jsonFlag, rawFlag bool

	fs.BoolVar(&skillsFlag, "skills", false, "Audit installed skills for frontmatter and token budget compliance")
	fs.BoolVar(&securityFlag, "security", false, "Scan installed skills and scripts for prompt injection and unsafe execution patterns")
	fs.BoolVar(&globalFlag, "global", false, "Include global skills (~/.agents/skills/)")
	fs.BoolVar(&globalFlag, "g", false, "Include global skills (shorthand)")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	if _, err := parseFlagsWithArgs(fs, args); err != nil {
		return ExitUsage
	}

	// Default to running all checks if neither --skills nor --security was explicitly narrowed
	runEnv := !skillsFlag && !securityFlag
	runSkills := skillsFlag || (!skillsFlag && !securityFlag)
	runSecurity := securityFlag || (!skillsFlag && !securityFlag)

	var findings []DoctorFinding

	if runEnv {
		cfgPath := config.ResolveConfigPath(a.Options.Config)
		findings = append(findings, DoctorFinding{
			Category: "environment",
			Severity: "ok",
			Message:  fmt.Sprintf("config path resolved (%s)", cfgPath),
			File:     cfgPath,
		})

		statePath := state.DefaultStateFilePath()
		if _, err := state.LoadState(statePath); err != nil {
			findings = append(findings, DoctorFinding{
				Category: "environment",
				Severity: "error",
				Message:  fmt.Sprintf("state file unreadable: %v", err),
				File:     statePath,
			})
		} else {
			findings = append(findings, DoctorFinding{
				Category: "environment",
				Severity: "ok",
				Message:  fmt.Sprintf("state manifest healthy (%s)", statePath),
				File:     statePath,
			})
		}

		cacheDir := a.cacheRootDir()
		findings = append(findings, DoctorFinding{
			Category: "environment",
			Severity: "ok",
			Message:  fmt.Sprintf("cache directory ready (%s)", cacheDir),
			File:     cacheDir,
		})
	}

	wd, _ := os.Getwd()
	skillDirs := harness.DiscoverInstalledSkillDirs(wd)
	if entries, err := os.ReadDir(filepath.Join(wd, "skills")); err == nil {
		for _, e := range entries {
			cand := filepath.Join(wd, "skills", e.Name())
			if _, sErr := os.Stat(filepath.Join(cand, "SKILL.md")); sErr == nil {
				if _, exists := skillDirs[e.Name()]; !exists {
					skillDirs[e.Name()] = cand
				}
			}
		}
	}
	if globalFlag {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			for k, v := range harness.DiscoverInstalledSkillDirs(home) {
				if _, exists := skillDirs[k]; !exists {
					skillDirs[k] = v
				}
			}
		}
	}

	var names []string
	for name := range skillDirs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		dir := skillDirs[name]
		skillMD := filepath.Join(dir, "SKILL.md")
		data, err := os.ReadFile(skillMD)
		if err != nil {
			findings = append(findings, DoctorFinding{
				Category: "skill",
				Skill:    name,
				Severity: "error",
				Message:  "missing or unreadable SKILL.md",
				File:     skillMD,
			})
			continue
		}

		if runSkills {
			fm, body, fmErr := catalog.ParseFrontmatter(data)
			if fmErr != nil || fm == nil {
				findings = append(findings, DoctorFinding{
					Category: "skill",
					Skill:    name,
					Severity: "error",
					Message:  fmt.Sprintf("invalid YAML frontmatter: %v", fmErr),
					File:     skillMD,
				})
			} else {
				if strings.TrimSpace(fm.Name) == "" {
					findings = append(findings, DoctorFinding{
						Category: "skill",
						Skill:    name,
						Severity: "warn",
						Message:  "frontmatter missing 'name' field",
						File:     skillMD,
					})
				}
				if strings.TrimSpace(fm.Description) == "" {
					findings = append(findings, DoctorFinding{
						Category: "skill",
						Skill:    name,
						Severity: "warn",
						Message:  "frontmatter missing 'description' field",
						File:     skillMD,
					})
				}
			}

			estTokens := (len(body) + 3) / 4
			if len(body) > DefaultPageSize {
				sections := catalog.ParseSections(body)
				findings = append(findings, DoctorFinding{
					Category: "budget",
					Skill:    name,
					Severity: "warn",
					Message:  fmt.Sprintf("SKILL.md body is %d bytes (~%d tokens > %dB single-page budget; %d sections available via 'dojo toc %s')", len(body), estTokens, DefaultPageSize, len(sections), name),
					File:     skillMD,
				})
			} else {
				findings = append(findings, DoctorFinding{
					Category: "skill",
					Skill:    name,
					Severity: "ok",
					Message:  fmt.Sprintf("valid SKILL.md (%d bytes, ~%d tokens)", len(body), estTokens),
					File:     skillMD,
				})
			}
		}

		if runSecurity {
			files := walkRelativeFiles(dir)
			flagged := false
			for _, rel := range files {
				fp := filepath.Join(dir, filepath.FromSlash(rel))
				content, err := os.ReadFile(fp)
				if err != nil {
					continue
				}
				text := string(content)
				for _, pat := range securityPatterns {
					if pat.regex.MatchString(text) {
						flagged = true
						findings = append(findings, DoctorFinding{
							Category: "security",
							Skill:    name,
							Severity: "warn",
							Message:  fmt.Sprintf("detected %s in %s", pat.name, rel),
							File:     fp,
						})
					}
				}
			}
			if !flagged {
				findings = append(findings, DoctorFinding{
					Category: "security",
					Skill:    name,
					Severity: "ok",
					Message:  "passed static security and prompt-injection scan",
					File:     dir,
				})
			}
		}
	}

	hasError := false
	warnCount := 0
	for _, f := range findings {
		if f.Severity == "error" {
			hasError = true
		} else if f.Severity == "warn" {
			warnCount++
		}
	}

	mode := a.resolveMode(jsonFlag, rawFlag)
	switch mode {
	case OutputModeJSON:
		res := struct {
			Healthy  bool            `json:"healthy"`
			Warnings int             `json:"warnings"`
			Findings []DoctorFinding `json:"findings"`
		}{
			Healthy:  !hasError,
			Warnings: warnCount,
			Findings: findings,
		}
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	case OutputModeRaw:
		for _, f := range findings {
			_, _ = fmt.Fprintf(a.Stdout, "%s\t%s\t%s\t%s\n",
				sanitizeTSVField(f.Severity),
				sanitizeTSVField(f.Category),
				sanitizeTSVField(f.Skill),
				sanitizeTSVField(f.Message),
			)
		}
	default:
		_, _ = fmt.Fprintf(a.Stdout, "DOJO DOCTOR DIAGNOSTICS (%d checks, %d warnings):\n\n", len(findings), warnCount)
		for _, f := range findings {
			prefix := "✓"
			switch f.Severity {
			case "warn":
				prefix = "⚠"
			case "error":
				prefix = "✗"
			}
			label := f.Category
			if f.Skill != "" {
				label = fmt.Sprintf("%s:%s", f.Category, f.Skill)
			}
			_, _ = fmt.Fprintf(a.Stdout, "  %s [%-18s] %s\n", prefix, label, f.Message)
		}
	}

	if hasError {
		return ExitGeneral
	}
	return ExitSuccess
}
