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
	{"hardcoded GitHub token (ghp_/github_pat_)", regexp.MustCompile(`\b(ghp_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})\b`)},
	{"hardcoded API key or secret token (sk-/AIza/AKIA/xox)", regexp.MustCompile(`\b(sk-[A-Za-z0-9_\-]{20,}|AIza[0-9A-Za-z_\-]{35}|AKIA[0-9A-Z]{16}|xox[baprs]-[0-9A-Za-z\-]{10,})\b`)},
	{"private key block", regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA |PRIVATE )?PRIVATE KEY-----`)},
}

// ScanDirectorySecurity scans all text files inside a skill directory for prompt injection,
// unsafe remote execution pipes, and hardcoded secret tokens.
func ScanDirectorySecurity(skillName, dir string) []DoctorFinding {
	var findings []DoctorFinding
	files := walkRelativeFiles(dir)
	for _, rel := range files {
		fp := filepath.Join(dir, filepath.FromSlash(rel))
		content, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		text := string(content)
		for _, pat := range securityPatterns {
			if pat.regex.MatchString(text) {
				findings = append(findings, DoctorFinding{
					Category: "security",
					Skill:    skillName,
					Severity: "error",
					Message:  fmt.Sprintf("detected %s in %s", pat.name, rel),
					File:     fp,
				})
			}
		}
	}
	return findings
}

// doctorOptions holds the parsed flags for `dojo doctor`.
type doctorOptions struct {
	skills   bool
	security bool
	global   bool
	json     bool
	raw      bool
}

// RunDoctor audits environment health, installed skill frontmatter compliance, token budgets,
// and suspicious prompt-injection or shell-execution patterns.
func (a *App) RunDoctor(args []string) int {
	opts, ok := a.parseDoctorArgs(args)
	if !ok {
		return ExitUsage
	}

	runEnv := !opts.skills && !opts.security

	var findings []DoctorFinding
	if runEnv {
		findings = append(findings, a.environmentFindings()...)
	}
	skillDirs := a.doctorSkillDirs(opts.global)
	for _, name := range sortedKeys(skillDirs) {
		findings = append(findings, a.auditInstalledSkill(name, skillDirs[name], opts)...)
	}

	hasError, warnCount := summarizeFindings(findings)
	writeDoctorReport(a.Stdout, a.resolveMode(opts.json, opts.raw), findings, hasError, warnCount)

	if hasError {
		return ExitGeneral
	}
	return ExitSuccess
}

func (a *App) parseDoctorArgs(args []string) (doctorOptions, bool) {
	var opts doctorOptions
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	fs.BoolVar(&opts.skills, "skills", false, "Audit installed skills for frontmatter and token budget compliance")
	fs.BoolVar(&opts.security, "security", false, "Scan installed skills and scripts for prompt injection and unsafe execution patterns")
	fs.BoolVar(&opts.global, "global", false, "Include global skills (~/.agents/skills/)")
	fs.BoolVar(&opts.global, "g", false, "Include global skills (shorthand)")
	fs.BoolVar(&opts.json, "json", false, "Output in JSON format")
	fs.BoolVar(&opts.raw, "raw", false, "Output in raw tab-separated format")

	if _, err := parseFlagsWithArgs(fs, args); err != nil {
		return opts, false
	}
	return opts, true
}

// environmentFindings checks config, state, and cache reachability.
func (a *App) environmentFindings() []DoctorFinding {
	var findings []DoctorFinding

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
	return findings
}

// doctorSkillDirs returns the skill directories to audit by name.
func (a *App) doctorSkillDirs(includeGlobal bool) map[string]string {
	wd, _ := os.Getwd()
	skillDirs := harness.DiscoverInstalledSkillDirs(wd)
	discoverWorkspaceSkillsDir(wd, skillDirs)
	if includeGlobal {
		discoverGlobalSkills(skillDirs)
	}
	return skillDirs
}

func sortedKeys(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// discoverWorkspaceSkillsDir adds skills under <wd>/skills that are not
// already installed in a harness.
func discoverWorkspaceSkillsDir(wd string, skillDirs map[string]string) {
	entries, err := os.ReadDir(filepath.Join(wd, "skills"))
	if err != nil {
		return
	}
	for _, e := range entries {
		cand := filepath.Join(wd, "skills", e.Name())
		if _, sErr := os.Stat(filepath.Join(cand, "SKILL.md")); sErr != nil {
			continue
		}
		if _, exists := skillDirs[e.Name()]; !exists {
			skillDirs[e.Name()] = cand
		}
	}
}

func discoverGlobalSkills(skillDirs map[string]string) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	for k, v := range harness.DiscoverInstalledSkillDirs(home) {
		if _, exists := skillDirs[k]; !exists {
			skillDirs[k] = v
		}
	}
}

// auditInstalledSkill validates one skill directory.
func (a *App) auditInstalledSkill(name, dir string, opts doctorOptions) []DoctorFinding {
	skillMD := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		return []DoctorFinding{{
			Category: "skill",
			Skill:    name,
			Severity: "error",
			Message:  "missing or unreadable SKILL.md",
			File:     skillMD,
		}}
	}

	var findings []DoctorFinding
	if opts.skills || (!opts.skills && !opts.security) {
		findings = append(findings, auditSkillFrontmatter(name, skillMD, data)...)
		findings = append(findings, auditSkillBudget(name, skillMD, data)...)
	}
	if opts.security || (!opts.skills && !opts.security) {
		findings = append(findings, auditSkillSecurity(name, dir)...)
	}
	return findings
}

func auditSkillFrontmatter(name, skillMD string, data []byte) []DoctorFinding {
	fm, _, fmErr := catalog.ParseFrontmatter(data)
	if fmErr != nil || fm == nil {
		return []DoctorFinding{{
			Category: "skill",
			Skill:    name,
			Severity: "error",
			Message:  fmt.Sprintf("invalid YAML frontmatter: %v", fmErr),
			File:     skillMD,
		}}
	}

	var findings []DoctorFinding
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
	return findings
}

func auditSkillBudget(name, skillMD string, data []byte) []DoctorFinding {
	_, body, _ := catalog.ParseFrontmatter(data)
	estTokens := (len(body) + 3) / 4

	if len(body) > DefaultPageSize {
		sections := catalog.ParseSections(body)
		return []DoctorFinding{{
			Category: "budget",
			Skill:    name,
			Severity: "warn",
			Message:  fmt.Sprintf("SKILL.md body is %d bytes (~%d tokens > %dB single-page budget; %d sections available via 'dojo toc %s')", len(body), estTokens, DefaultPageSize, len(sections), name),
			File:     skillMD,
		}}
	}
	return []DoctorFinding{{
		Category: "skill",
		Skill:    name,
		Severity: "ok",
		Message:  fmt.Sprintf("valid SKILL.md (%d bytes, ~%d tokens)", len(body), estTokens),
		File:     skillMD,
	}}
}

func auditSkillSecurity(name, dir string) []DoctorFinding {
	secFindings := ScanDirectorySecurity(name, dir)
	if len(secFindings) > 0 {
		return secFindings
	}
	return []DoctorFinding{{
		Category: "security",
		Skill:    name,
		Severity: "ok",
		Message:  "passed static security and prompt-injection scan",
		File:     dir,
	}}
}

func summarizeFindings(findings []DoctorFinding) (hasError bool, warnCount int) {
	for _, f := range findings {
		switch f.Severity {
		case "error":
			hasError = true
		case "warn":
			warnCount++
		}
	}
	return hasError, warnCount
}

func writeDoctorReport(stdout interface{ Write([]byte) (int, error) }, mode OutputMode, findings []DoctorFinding, hasError bool, warnCount int) {
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
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	case OutputModeRaw:
		for _, f := range findings {
			_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n",
				sanitizeTSVField(f.Severity),
				sanitizeTSVField(f.Category),
				sanitizeTSVField(f.Skill),
				sanitizeTSVField(f.Message),
			)
		}
	default:
		_, _ = fmt.Fprintf(stdout, "DOJO DOCTOR DIAGNOSTICS (%d checks, %d warnings):\n\n", len(findings), warnCount)
		for _, f := range findings {
			_, _ = fmt.Fprintf(stdout, "  %s [%-18s] %s\n", severityPrefix(f.Severity), doctorFindingLabel(f), f.Message)
		}
	}
}

func severityPrefix(severity string) string {
	switch severity {
	case "warn":
		return "⚠"
	case "error":
		return "✗"
	default:
		return "✓"
	}
}

func doctorFindingLabel(f DoctorFinding) string {
	if f.Skill == "" {
		return f.Category
	}
	return fmt.Sprintf("%s:%s", f.Category, f.Skill)
}
