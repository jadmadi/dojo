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
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/harness"
	"github.com/jadmadi/dojo/internal/profile"
)

// RunProfile manages the traveling skills.json manifest and Git-backed Local<->Cloud profile continuity.
func (a *App) RunProfile(args []string) int {
	if len(args) == 0 {
		return a.runProfileExport(nil)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "export", "save":
		return a.runProfileExport(rest)
	case "apply", "restore", "install":
		return a.runProfileApply(rest)
	case "sync":
		return a.runProfileSync(rest)
	case "push", "publish":
		return a.runProfilePush(rest)
	default:
		if strings.HasPrefix(sub, "-") {
			return a.runProfileExport(args)
		}
		_, _ = fmt.Fprintf(a.Stderr, "error: unknown profile subcommand %q (valid: export, apply, sync, push)\n", sub)
		return ExitUsage
	}
}

func resolveProfileScopeAndDir(globalFlag bool) (string, string) {
	scope, rootDir := resolveScopeAndDir(globalFlag)
	if globalFlag || rootDir == "" {
		rootDir, _ = os.UserHomeDir()
	}
	return scope, rootDir
}

func (a *App) runProfileExport(args []string) int {
	fs := flag.NewFlagSet("profile export", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var outFlag string
	var globalFlag, stdoutFlag, jsonFlag, rawFlag bool

	fs.StringVar(&outFlag, "out", "", "Output path for skills.json (default: ./skills.json or ~/.config/dojo/skills.json with -g)")
	fs.StringVar(&outFlag, "o", "", "Output path for skills.json (shorthand)")
	fs.BoolVar(&globalFlag, "global", false, "Export global installed skills (~/.agents/skills/)")
	fs.BoolVar(&globalFlag, "g", false, "Export global installed skills (shorthand)")
	fs.BoolVar(&stdoutFlag, "stdout", false, "Print skills.json directly to stdout instead of writing a file")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return ExitUsage
	}
	if len(posArgs) > 0 && outFlag == "" {
		outFlag = posArgs[0]
	}

	scope, rootDir := resolveProfileScopeAndDir(globalFlag)
	_ = harness.ReconcileHarnesses(rootDir)

	st := a.loadState()
	targets := harness.DetectTargets(rootDir)
	manifest := profile.BuildManifest(st, scope, rootDir, targets)

	if stdoutFlag {
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(manifest)
		return ExitSuccess
	}

	if outFlag == "" {
		if globalFlag {
			outFlag = filepath.Join(rootDir, profile.DefaultManifestFilename)
		} else {
			wd, _ := os.Getwd()
			outFlag = filepath.Join(wd, profile.DefaultManifestFilename)
		}
	}

	if err := profile.SaveManifest(outFlag, manifest); err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: saving %s: %v\n", outFlag, err)
		return ExitGeneral
	}

	mode := a.resolveMode(jsonFlag, rawFlag)
	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"status":   "exported",
			"path":     outFlag,
			"skills":   len(manifest.Skills),
			"manifest": manifest,
		})
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "exported\t%s\t%d\n", sanitizeTSVField(outFlag), len(manifest.Skills))
	default:
		_, _ = fmt.Fprintf(a.Stdout, "✓ Exported %d skill(s) and %d harness target(s) to %s\n",
			len(manifest.Skills), len(manifest.Harnesses), outFlag)
	}
	return ExitSuccess
}

func (a *App) runProfileApply(args []string) int {
	fs := flag.NewFlagSet("profile apply", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var globalFlag, jsonFlag, rawFlag bool

	fs.BoolVar(&globalFlag, "global", false, "Apply skills into global scope (~/.agents/skills/)")
	fs.BoolVar(&globalFlag, "g", false, "Apply skills into global scope (shorthand)")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return ExitUsage
	}

	source := profile.DefaultManifestFilename
	if len(posArgs) > 0 {
		source = posArgs[0]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manifest, err := profile.LoadManifest(ctx, source, a.getHTTPClient())
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: %v\n", err)
		return MapErrorToExitCode(err)
	}

	installedSkills, projected, applyErr := a.applySkillsManifest(manifest, globalFlag)
	if applyErr != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: %v\n", applyErr)
		return ExitGeneral
	}

	mode := a.resolveMode(jsonFlag, rawFlag)
	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"status":      "applied",
			"source":      source,
			"skills":      installedSkills,
			"projections": projected,
		})
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "applied\t%s\t%d\t%d\n",
			sanitizeTSVField(source), len(installedSkills), len(projected))
	default:
		_, _ = fmt.Fprintf(a.Stdout, "✓ Applied profile %q (%d skill(s) installed, %d harness projection(s) synced)\n",
			source, len(installedSkills), len(projected))
	}
	return ExitSuccess
}

func (a *App) applySkillsManifest(manifest *profile.SkillsManifest, globalFlag bool) ([]string, []harness.ProjectionResult, error) {
	_, rootDir := resolveProfileScopeAndDir(globalFlag)

	// 1. Register any custom catalogs declared in the manifest
	for catName, catSource := range manifest.Catalogs {
		if catName == "" || catSource == "" || catName == "default" {
			continue
		}
		var subOut, subErr bytes.Buffer
		subApp := *a
		subApp.Stdout = &subOut
		subApp.Stderr = &subErr
		_ = subApp.RunCatalog([]string{"add", catSource, "--name", catName})
	}

	// 2. Determine target harnesses
	targetSpec := strings.Join(manifest.EffectiveHarnesses(), ",")
	if targetSpec == "" {
		targetSpec = "auto"
	}

	// 3. Install each listed skill
	var installed []string
	for _, entry := range manifest.Skills {
		if strings.TrimSpace(entry.Name) == "" {
			continue
		}
		spec := entry.Name
		if entry.Repo != "" {
			spec = entry.Repo
			if !strings.Contains(spec, "/") {
				spec = entry.Name
			}
		} else if entry.Catalog != "" && entry.Catalog != "default" {
			spec = fmt.Sprintf("%s@%s", entry.Name, entry.Catalog)
		}

		// If skill already exists locally in .agents/skills/<name>/SKILL.md, no network learn is required
		canPath := filepath.Join(rootDir, ".agents", "skills", entry.Name, "SKILL.md")
		if _, err := os.Stat(canPath); err == nil {
			installed = append(installed, entry.Name)
			continue
		}

		learnArgs := []string{spec, "--with-deps"}
		if globalFlag {
			learnArgs = append(learnArgs, "-g")
		}
		var lOut, lErr bytes.Buffer
		subApp := *a
		subApp.Stdout = &lOut
		subApp.Stderr = &lErr
		if code := subApp.RunLearn(learnArgs); code == ExitSuccess {
			installed = append(installed, entry.Name)
		} else if manifest.SourceRepo != "" && entry.Repo == "" {
			// Fallback: skill may be hosted directly inside the profile repository itself (e.g. jadmadi/agy-vm/skills/<name>)
			repoSpec := fmt.Sprintf("github:%s/skills/%s", manifest.SourceRepo, entry.Name)
			if manifest.SourceRef != "" && manifest.SourceRef != "main" {
				repoSpec = fmt.Sprintf("%s@%s", repoSpec, manifest.SourceRef)
			}
			fallbackArgs := []string{repoSpec, "--with-deps"}
			if globalFlag {
				fallbackArgs = append(fallbackArgs, "-g")
			}
			lOut.Reset()
			lErr.Reset()
			if fCode := subApp.RunLearn(fallbackArgs); fCode == ExitSuccess {
				installed = append(installed, entry.Name)
			}
		}
	}

	// 4. Reconcile & sync across target harnesses
	_ = harness.ReconcileHarnesses(rootDir)
	targets, err := harness.ParseTargets(targetSpec, rootDir)
	if err != nil {
		targets = harness.DetectTargets(rootDir)
	}
	discovered := harness.DiscoverInstalledSkillDirs(rootDir)
	var projections []harness.ProjectionResult
	for name, dir := range discovered {
		res := harness.SyncSkill(name, dir, rootDir, targets)
		projections = append(projections, res...)
	}

	return installed, projections, nil
}

func (a *App) runProfileSync(args []string) int {
	fs := flag.NewFlagSet("profile sync", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var dirFlag string
	var globalFlag, pushFlag, jsonFlag, rawFlag bool

	fs.StringVar(&dirFlag, "dir", "", "Target directory or Git repository path (default: current directory or ~ with -g)")
	fs.StringVar(&dirFlag, "d", "", "Target directory or Git repository path (shorthand)")
	fs.BoolVar(&globalFlag, "global", false, "Sync global profile (~/.agents/skills/ and ~/skills.json)")
	fs.BoolVar(&globalFlag, "g", false, "Sync global profile (shorthand)")
	fs.BoolVar(&pushFlag, "push", false, "Commit and push skills.json to the remote Git repository")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	if _, err := parseFlagsWithArgs(fs, args); err != nil {
		return ExitUsage
	}

	scope, rootDir := resolveProfileScopeAndDir(globalFlag)
	if dirFlag != "" {
		rootDir = dirFlag
	}

	// 1. Bi-directionally reconcile harnesses first
	reconciled := harness.ReconcileHarnesses(rootDir)

	// 2. If skills.json already exists in rootDir, apply any missing skills from it
	manifestPath := filepath.Join(rootDir, profile.DefaultManifestFilename)
	if existing, err := profile.LoadManifest(context.Background(), manifestPath, a.getHTTPClient()); err == nil && existing != nil {
		_, _, _ = a.applySkillsManifest(existing, globalFlag)
	}

	// 3. Export updated skills.json reflecting all currently installed & reconciled skills
	st := a.loadState()
	targets := harness.DetectTargets(rootDir)
	updatedManifest := profile.BuildManifest(st, scope, rootDir, targets)
	if err := profile.SaveManifest(manifestPath, updatedManifest); err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: writing %s: %v\n", manifestPath, err)
		return ExitGeneral
	}

	// 4. Project all canonical skills to all detected harnesses
	discovered := harness.DiscoverInstalledSkillDirs(rootDir)
	var projections []harness.ProjectionResult
	projections = append(projections, reconciled...)
	for name, dir := range discovered {
		projections = append(projections, harness.SyncSkill(name, dir, rootDir, targets)...)
	}

	// 5. Optional Git commit & push if --push is requested and rootDir is a git repo
	gitPushed := false
	if pushFlag {
		if gitBin, err := exec.LookPath("git"); err == nil && gitBin != "" {
			addCmd := exec.Command(gitBin, "-C", rootDir, "add", profile.DefaultManifestFilename)
			_ = addCmd.Run()
			commitCmd := exec.Command(gitBin, "-C", rootDir, "commit", "-m", "chore(dojo): sync traveling skills.json manifest")
			_ = commitCmd.Run()
			pushCmd := exec.Command(gitBin, "-C", rootDir, "push")
			if err := pushCmd.Run(); err == nil {
				gitPushed = true
			}
		}
	}

	mode := a.resolveMode(jsonFlag, rawFlag)
	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"status":      "synced",
			"manifest":    manifestPath,
			"skills":      len(updatedManifest.Skills),
			"projections": len(projections),
			"git_pushed":  gitPushed,
		})
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "synced\t%s\t%d\t%d\n",
			sanitizeTSVField(manifestPath), len(updatedManifest.Skills), len(projections))
	default:
		_, _ = fmt.Fprintf(a.Stdout, "✓ Synced profile at %s (%d skill(s), %d harness projection(s))\n",
			manifestPath, len(updatedManifest.Skills), len(projections))
		if gitPushed {
			_, _ = fmt.Fprintln(a.Stdout, "  ✓ Committed and pushed skills.json to remote Git repository")
		}
	}
	return ExitSuccess
}

// runProfilePush packages local skills, rules (.mdc/.md), or slash commands/workflows
// and pushes them (along with an updated skills.json) to a target directory or GitHub profile repository
// (e.g. `dojo profile push github:jadmadi/agy-vm my-skill my-command`).
func (a *App) runProfilePush(args []string) int {
	fs := flag.NewFlagSet("profile push", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var repoFlag string
	var allFlag, globalFlag, jsonFlag, rawFlag bool

	fs.StringVar(&repoFlag, "repo", "", "Target GitHub repository (github:owner/repo) or local profile directory")
	fs.StringVar(&repoFlag, "r", "", "Target GitHub repository or directory (shorthand)")
	fs.BoolVar(&allFlag, "all", false, "Push all installed skills in workspace (or global with -g)")
	fs.BoolVar(&allFlag, "a", false, "Push all installed skills (shorthand)")
	fs.BoolVar(&globalFlag, "global", false, "Search global scope first")
	fs.BoolVar(&globalFlag, "g", false, "Search global scope first (shorthand)")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return ExitUsage
	}

	targetSpec := strings.TrimSpace(repoFlag)
	var rawSpecs []string
	if targetSpec == "" {
		if len(posArgs) == 0 {
			_, _ = fmt.Fprintln(a.Stderr, "error: profile push requires a target repository/directory and at least one skill/command name (or --all)\nusage: dojo profile push <github:owner/repo | dir> <skill-or-command...>")
			return ExitUsage
		}
		targetSpec = posArgs[0]
		rawSpecs = posArgs[1:]
	} else {
		rawSpecs = posArgs
	}
	skillSpecs := a.resolveSkillSpecs(rawSpecs)

	wd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()
	var searchRoots []string
	if globalFlag {
		searchRoots = []string{homeDir, wd}
	} else {
		searchRoots = []string{wd, homeDir}
	}

	for _, r := range searchRoots {
		if r != "" {
			_ = harness.ReconcileHarnesses(r)
		}
	}

	if allFlag && len(skillSpecs) == 0 {
		primaryRoot := wd
		if globalFlag && homeDir != "" {
			primaryRoot = homeDir
		}
		for name := range harness.DiscoverInstalledSkillDirs(primaryRoot) {
			skillSpecs = append(skillSpecs, name)
		}
		sort.Strings(skillSpecs)
	}

	if len(skillSpecs) == 0 {
		_, _ = fmt.Fprintln(a.Stderr, "error: specify at least one skill, rule, or command to push (or pass --all)\nexample: dojo profile push github:jadmadi/agy-vm my-skill my-command")
		return ExitUsage
	}

	type locatedItem struct {
		name    string
		dir     string
		cleanup func()
	}
	var items []locatedItem
	defer func() {
		for _, it := range items {
			if it.cleanup != nil {
				it.cleanup()
			}
		}
	}()

	for _, spec := range skillSpecs {
		name, dir, cleanup, lErr := harness.LocateOrPackageLocalSkill(spec, searchRoots)
		if lErr != nil {
			_, _ = fmt.Fprintf(a.Stderr, "error: %v\n", lErr)
			return ExitNotFound
		}
		items = append(items, locatedItem{name: name, dir: dir, cleanup: cleanup})
	}

	// Determine whether targetSpec is a local directory or a remote GitHub repository
	var workTree string
	isRemoteClone := false
	if info, statErr := os.Stat(targetSpec); statErr == nil && info.IsDir() {
		workTree = targetSpec
	} else if ghTarget, ok := catalog.ParseGitHubTarget(strings.TrimPrefix(targetSpec, "github:")); ok {
		tmpClone, tErr := os.MkdirTemp("", "dojo-profile-push-*")
		if tErr != nil {
			_, _ = fmt.Fprintf(a.Stderr, "error: creating temporary directory: %v\n", tErr)
			return ExitGeneral
		}
		defer func() { _ = os.RemoveAll(tmpClone) }()

		cloned := false
		ghBin, ghErr := exec.LookPath("gh")
		if ghErr != nil {
			if _, sErr := os.Stat("/opt/antigravity/bin/gh"); sErr == nil {
				ghBin = "/opt/antigravity/bin/gh"
				ghErr = nil
			}
		}
		if ghErr == nil && ghBin != "" {
			cloneArgs := []string{"repo", "clone", ghTarget.RepoString(), tmpClone, "--", "--depth", "1"}
			if ghTarget.Ref != "" && ghTarget.Ref != "main" {
				cloneArgs = append(cloneArgs, "-b", ghTarget.Ref)
			}
			cmd := exec.Command(ghBin, cloneArgs...)
			if err := cmd.Run(); err == nil {
				cloned = true
			}
		}
		if !cloned {
			gitBin, gErr := exec.LookPath("git")
			if gErr != nil {
				_, _ = fmt.Fprintln(a.Stderr, "error: neither 'gh' nor 'git' is available to clone target repository")
				return ExitGeneral
			}
			cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", ghTarget.Owner, ghTarget.Repo)
			cmd := exec.Command(gitBin, "clone", "--depth", "1", cloneURL, tmpClone)
			if out, err := cmd.CombinedOutput(); err != nil {
				_, _ = fmt.Fprintf(a.Stderr, "error: cloning %s: %v (%s)\n", ghTarget.RepoString(), err, strings.TrimSpace(string(out)))
				return ExitGeneral
			}
		}
		workTree = tmpClone
		isRemoteClone = true
	} else {
		_, _ = fmt.Fprintf(a.Stderr, "error: target %q is neither an existing local directory nor a valid GitHub repository (github:owner/repo)\n", targetSpec)
		return ExitUsage
	}

	// Copy each skill package into <workTree>/skills/<name>/ and upsert into <workTree>/skills.json
	manifestPath := filepath.Join(workTree, profile.DefaultManifestFilename)
	manifest, _ := profile.LoadManifest(context.Background(), manifestPath, a.getHTTPClient())
	if manifest == nil {
		manifest = &profile.SkillsManifest{
			Schema:    profile.DefaultSchemaURL,
			Version:   "1.0.0",
			Harnesses: []string{"agents", "antigravity", "claude", "cursor", "sila"},
			Skills:    []profile.SkillEntry{},
		}
	}

	existingByName := make(map[string]profile.SkillEntry)
	for _, s := range manifest.Skills {
		existingByName[s.Name] = s
	}

	var pushedNames []string
	for _, it := range items {
		destSkillDir := filepath.Join(workTree, "skills", it.name)
		if err := harness.CopySkillDir(it.dir, destSkillDir); err != nil {
			_, _ = fmt.Fprintf(a.Stderr, "error: copying skill %q to %s: %v\n", it.name, destSkillDir, err)
			return ExitGeneral
		}
		ver := ""
		if data, rErr := os.ReadFile(filepath.Join(destSkillDir, "SKILL.md")); rErr == nil {
			if fm, _, fErr := catalog.ParseFrontmatter(data); fErr == nil && fm != nil && fm.Metadata != nil {
				ver = fm.Metadata["version"]
			}
		}
		entry := existingByName[it.name]
		entry.Name = it.name
		if ver != "" {
			entry.Version = ver
		}
		existingByName[it.name] = entry
		pushedNames = append(pushedNames, it.name)
	}

	var allNames []string
	for name := range existingByName {
		allNames = append(allNames, name)
	}
	sort.Strings(allNames)
	manifest.Skills = make([]profile.SkillEntry, 0, len(allNames))
	for _, name := range allNames {
		manifest.Skills = append(manifest.Skills, existingByName[name])
	}

	if err := profile.SaveManifest(manifestPath, manifest); err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: saving %s: %v\n", manifestPath, err)
		return ExitGeneral
	}

	// Commit and push if workTree is a git repository
	gitPushed := false
	if _, gitStatErr := os.Stat(filepath.Join(workTree, ".git")); gitStatErr == nil {
		if gitBin, gErr := exec.LookPath("git"); gErr == nil && gitBin != "" {
			_ = exec.Command(gitBin, "-C", workTree, "add", "-f", profile.DefaultManifestFilename, "skills").Run()
			statusOut, _ := exec.Command(gitBin, "-C", workTree, "status", "--porcelain").Output()
			if len(bytes.TrimSpace(statusOut)) > 0 {
				commitMsg := fmt.Sprintf("feat(dojo): push skill(s) %s to profile", strings.Join(pushedNames, ", "))
				_ = exec.Command(gitBin, "-C", workTree, "commit", "-m", commitMsg).Run()
				if pushOut, pErr := exec.Command(gitBin, "-C", workTree, "push").CombinedOutput(); pErr == nil {
					gitPushed = true
				} else if isRemoteClone {
					_, _ = fmt.Fprintf(a.Stderr, "error: git push to %s failed: %v (%s)\n", targetSpec, pErr, strings.TrimSpace(string(pushOut)))
					return ExitGeneral
				}
			} else {
				gitPushed = true
			}
		}
	}

	mode := a.resolveMode(jsonFlag, rawFlag)
	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"status":     "pushed",
			"target":     targetSpec,
			"skills":     pushedNames,
			"git_pushed": gitPushed,
		})
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "pushed\t%s\t%s\n", sanitizeTSVField(targetSpec), strings.Join(pushedNames, ","))
	default:
		_, _ = fmt.Fprintf(a.Stdout, "✓ Pushed %d skill(s) [%s] and updated skills.json in %s\n",
			len(pushedNames), strings.Join(pushedNames, ", "), targetSpec)
	}
	return ExitSuccess
}

