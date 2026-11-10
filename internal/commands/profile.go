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
	"github.com/jadmadi/dojo/internal/config"
	"github.com/jadmadi/dojo/internal/harness"
	"github.com/jadmadi/dojo/internal/profile"
	"github.com/jadmadi/dojo/internal/runner"
	"github.com/jadmadi/dojo/internal/state"
)

// RunProfile manages the traveling skills.json manifest and Git-backed Local<->Cloud profile continuity.
func (a *App) RunProfile(args []string) int {
	if len(args) == 0 {
		return a.runProfileExport(nil)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "link", "remote", "repo":
		return a.runProfileLink(rest)
	case "unlink":
		return a.runProfileUnlink(rest)
	case "export", "save":
		return a.runProfileExport(rest)
	case "apply", "restore", "install", "pull":
		return a.runProfileApply(rest)
	case "sync":
		return a.runProfileSync(rest)
	case "push", "publish":
		return a.runProfilePush(rest)
	default:
		if strings.HasPrefix(sub, "-") {
			return a.runProfileExport(args)
		}
		_, _ = fmt.Fprintf(a.Stderr, "error: unknown profile subcommand %q (valid: link, unlink, export, apply, pull, sync, push)\n", sub)
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

func (a *App) resolveEffectiveGlobalFlag(explicitGlobal bool) bool {
	if explicitGlobal {
		return true
	}
	if a.Config != nil && a.Config.ProfileScope == state.ScopeGlobal {
		return true
	}
	return false
}

func (a *App) runProfileLink(args []string) int {
	fs := flag.NewFlagSet("profile link", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var globalFlag, jsonFlag, rawFlag bool

	fs.BoolVar(&globalFlag, "global", false, "Default linked profile operations to global scope (~/.agents/skills/)")
	fs.BoolVar(&globalFlag, "g", false, "Default linked profile operations to global scope (shorthand)")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return ExitUsage
	}

	mode := a.resolveMode(jsonFlag, rawFlag)

	cfg := a.Config
	if cfg == nil {
		var lErr error
		cfg, lErr = config.Load(a.Options.Config)
		if lErr != nil {
			cfg = config.DefaultConfig()
		}
	}

	if len(posArgs) == 0 {
		repo := cfg.GetProfileRepo()
		scope := cfg.ProfileScope
		if scope == "" {
			scope = state.ScopeWorkspace
		}
		switch mode {
		case OutputModeJSON:
			enc := json.NewEncoder(a.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{
				"linked":       repo != "",
				"profile_repo": repo,
				"scope":        scope,
			})
		case OutputModeRaw:
			_, _ = fmt.Fprintf(a.Stdout, "%s\t%s\n", sanitizeTSVField(repo), scope)
		default:
			if repo == "" {
				_, _ = fmt.Fprintln(a.Stdout, "No personal skills registry linked.\nRun 'dojo profile link github:<owner>/<repo> -g' to link a repository.")
			} else {
				_, _ = fmt.Fprintf(a.Stdout, "Linked personal skills registry: %s (default scope: %s)\n", repo, scope)
			}
		}
		return ExitSuccess
	}

	target := strings.TrimSpace(posArgs[0])
	// Validate that target is either an existing local directory or a valid GitHub repo target
	isDir := false
	if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
		if abs, aErr := filepath.Abs(target); aErr == nil {
			target = abs
		}
		isDir = true
	}
	if !isDir {
		cleanGH := strings.TrimPrefix(target, "github:")
		if ghTarget, ok := catalog.ParseGitHubTarget(cleanGH); ok && ghTarget.Owner != "" && ghTarget.Repo != "" {
			if !strings.HasPrefix(target, "github:") && !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
				target = "github:" + ghTarget.RepoString()
			}
		} else {
			_, _ = fmt.Fprintf(a.Stderr, "error: invalid registry target %q (expected github:<owner>/<repo> or an existing local directory)\n", target)
			return ExitUsage
		}
	}

	scope := state.ScopeWorkspace
	if globalFlag {
		scope = state.ScopeGlobal
	}
	cfg.SetProfileRepo(target, scope)
	savedPath, err := config.Save(a.Options.Config, cfg)
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: saving config: %v\n", err)
		return ExitGeneral
	}
	a.Config = cfg

	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"status":       "linked",
			"profile_repo": target,
			"scope":        scope,
			"config_path":  savedPath,
		})
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "linked\t%s\t%s\n", sanitizeTSVField(target), scope)
	default:
		_, _ = fmt.Fprintf(a.Stdout, "✓ Linked personal skills registry to %s (scope: %s)\n", target, scope)
		_, _ = fmt.Fprintln(a.Stdout, "  • Run 'dojo profile sync' to bi-directionally synchronize local skills and GitHub")
		_, _ = fmt.Fprintln(a.Stdout, "  • Run 'dojo profile pull' on remote VMs to install all skills from your registry")
	}
	return ExitSuccess
}

func (a *App) runProfileUnlink(args []string) int {
	fs := flag.NewFlagSet("profile unlink", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var jsonFlag, rawFlag bool

	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	if _, err := parseFlagsWithArgs(fs, args); err != nil {
		return ExitUsage
	}

	cfg := a.Config
	if cfg == nil {
		var lErr error
		cfg, lErr = config.Load(a.Options.Config)
		if lErr != nil {
			cfg = config.DefaultConfig()
		}
	}
	prev := cfg.ProfileRepo
	cfg.ProfileRepo = ""
	cfg.ProfileScope = ""
	_, err := config.Save(a.Options.Config, cfg)
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: saving config: %v\n", err)
		return ExitGeneral
	}
	a.Config = cfg

	mode := a.resolveMode(jsonFlag, rawFlag)
	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"status":   "unlinked",
			"previous": prev,
		})
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "unlinked\t%s\n", sanitizeTSVField(prev))
	default:
		if prev != "" {
			_, _ = fmt.Fprintf(a.Stdout, "✓ Unlinked personal skills registry (%s)\n", prev)
		} else {
			_, _ = fmt.Fprintln(a.Stdout, "✓ No personal skills registry was linked")
		}
	}
	return ExitSuccess
}

func (a *App) runProfileExport(args []string) int {
	fs := flag.NewFlagSet("profile export", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var outFlag string
	var globalFlag, stdoutFlag, jsonFlag, rawFlag bool

	fs.StringVar(&outFlag, "out", "", "Output path for skills.json (default: ./skills.json or ~/skills.json with -g)")
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

	effectiveGlobal := a.resolveEffectiveGlobalFlag(globalFlag)
	scope, rootDir := resolveProfileScopeAndDir(effectiveGlobal)
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
		if effectiveGlobal {
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

	effectiveGlobal := a.resolveEffectiveGlobalFlag(globalFlag)
	source := profile.DefaultManifestFilename
	if len(posArgs) > 0 {
		source = posArgs[0]
	} else if a.Config != nil && a.Config.GetProfileRepo() != "" {
		if _, statErr := os.Stat(source); statErr != nil || effectiveGlobal {
			source = a.Config.GetProfileRepo()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manifest, err := profile.LoadManifest(ctx, source, a.getHTTPClient())
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: %v\n", err)
		return MapErrorToExitCode(err)
	}

	installedSkills, projected, applyErr := a.applySkillsManifest(manifest, effectiveGlobal)
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
		spec := profileEntrySpec(entry)

		// If skill already exists locally in .agents/skills/<name>/SKILL.md, no network learn is required
		canPath := filepath.Join(rootDir, ".agents", "skills", entry.Name, "SKILL.md")
		if _, err := os.Stat(canPath); err == nil {
			installed = append(installed, entry.Name)
			continue
		}

		// Check if the manifest came from a local directory containing skills/<name>/SKILL.md
		if manifest.SourceRepo != "" {
			if info, sErr := os.Stat(manifest.SourceRepo); sErr == nil && info.IsDir() {
				localRepoSkillDir := filepath.Join(manifest.SourceRepo, "skills", entry.Name)
				if _, mdErr := os.Stat(filepath.Join(localRepoSkillDir, "SKILL.md")); mdErr == nil {
					destSkillDir := filepath.Join(rootDir, ".agents", "skills", entry.Name)
					if cErr := harness.CopySkillDir(localRepoSkillDir, destSkillDir); cErr == nil {
						installed = append(installed, entry.Name)
						continue
					}
				}
			}
		}

		learnArgs := []string{spec, "--with-deps"}
		if globalFlag {
			learnArgs = append(learnArgs, "-g")
		}
		var lOut, lErr bytes.Buffer
		subApp := *a
		subApp.Stdout = &lOut
		subApp.Stderr = &lErr
		learned := subApp.RunLearn(learnArgs) == ExitSuccess
		if !learned && isGitHubRepoSlug(manifest.SourceRepo) && entry.Repo == "" {
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
			learned = subApp.RunLearn(fallbackArgs) == ExitSuccess
		}
		if learned {
			installed = append(installed, entry.Name)
			continue
		}
		_, _ = fmt.Fprintf(a.Stderr, "warning: could not install %q from profile manifest%s\n", entry.Name, bufferHint(lErr.String()))
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

// profileEntrySpec builds a learn spec for a manifest entry. A catalog URL is
// not a registered catalog name, so the name alone is used instead.
func profileEntrySpec(entry profile.SkillEntry) string {
	if entry.Repo != "" {
		if !strings.Contains(entry.Repo, "/") {
			return entry.Name
		}
		return entry.Repo
	}
	if entry.Catalog != "" && entry.Catalog != "default" && !strings.Contains(entry.Catalog, "://") {
		return fmt.Sprintf("%s@%s", entry.Name, entry.Catalog)
	}
	return entry.Name
}

// isGitHubRepoSlug reports whether a manifest source is an owner/repo slug
// rather than a local directory path.
func isGitHubRepoSlug(source string) bool {
	return source != "" &&
		!strings.Contains(source, ":") &&
		!strings.HasPrefix(source, "/") &&
		strings.Count(source, "/") == 1
}

// bufferHint returns the last non-empty line of a captured stderr buffer.
func bufferHint(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		return ""
	}
	return ": " + last
}

func (a *App) runProfileSync(args []string) int {
	fs := flag.NewFlagSet("profile sync", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var dirFlag, repoFlag string
	var globalFlag, pushFlag, pullOnlyFlag, forceFlag, jsonFlag, rawFlag bool

	fs.StringVar(&dirFlag, "dir", "", "Target directory or Git repository path (default: current directory or ~ with -g)")
	fs.StringVar(&dirFlag, "d", "", "Target directory or Git repository path (shorthand)")
	fs.StringVar(&repoFlag, "repo", "", "Remote registry repository (github:owner/repo) or directory")
	fs.StringVar(&repoFlag, "r", "", "Remote registry repository or directory (shorthand)")
	fs.BoolVar(&globalFlag, "global", false, "Sync global profile (~/.agents/skills/ and ~/skills.json)")
	fs.BoolVar(&globalFlag, "g", false, "Sync global profile (shorthand)")
	fs.BoolVar(&pushFlag, "push", false, "Commit and push skills to the remote Git repository")
	fs.BoolVar(&pullOnlyFlag, "pull", false, "Pull from remote registry without pushing local changes back")
	fs.BoolVar(&forceFlag, "force", false, "Skip pre-push security/secret scan")
	fs.BoolVar(&forceFlag, "f", false, "Skip pre-push security/secret scan (shorthand)")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return ExitUsage
	}

	effectiveGlobal := a.resolveEffectiveGlobalFlag(globalFlag)
	scope, rootDir := resolveProfileScopeAndDir(effectiveGlobal)
	if dirFlag != "" {
		rootDir = dirFlag
	}

	remoteTarget := strings.TrimSpace(repoFlag)
	if remoteTarget == "" && len(posArgs) > 0 {
		remoteTarget = strings.TrimSpace(posArgs[0])
	}
	if remoteTarget == "" && a.Config != nil {
		remoteTarget = a.Config.GetProfileRepo()
	}

	// 1. Bi-directionally reconcile local harnesses first
	reconciled := harness.ReconcileHarnesses(rootDir)

	// 2. Pull / Apply from remote registry if linked or specified
	var pulledSkills []string
	if remoteTarget != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		remoteManifest, rErr := profile.LoadManifest(ctx, remoteTarget, a.getHTTPClient())
		cancel()
		if rErr == nil && remoteManifest != nil {
			if info, sErr := os.Stat(remoteTarget); sErr == nil && info.IsDir() {
				remoteManifest.SourceRepo = remoteTarget
			}
			pulledSkills, _, _ = a.applySkillsManifest(remoteManifest, effectiveGlobal)
		}
	}

	// 3. Also check if skills.json already exists in rootDir and apply any missing skills from it
	manifestPath := filepath.Join(rootDir, profile.DefaultManifestFilename)
	if existing, err := profile.LoadManifest(context.Background(), manifestPath, a.getHTTPClient()); err == nil && existing != nil {
		_, _, _ = a.applySkillsManifest(existing, effectiveGlobal)
	}

	// 4. Export updated skills.json reflecting all currently installed & reconciled skills
	st := a.loadState()
	targets := harness.DetectTargets(rootDir)
	updatedManifest := profile.BuildManifest(st, scope, rootDir, targets)
	if err := profile.SaveManifest(manifestPath, updatedManifest); err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: writing %s: %v\n", manifestPath, err)
		return ExitGeneral
	}

	// 5. Project all canonical skills to all detected harnesses
	discovered := harness.DiscoverInstalledSkillDirs(rootDir)
	var projections []harness.ProjectionResult
	projections = append(projections, reconciled...)
	for name, dir := range discovered {
		projections = append(projections, harness.SyncSkill(name, dir, rootDir, targets)...)
	}

	// 6. Push local skills and updated manifest back to remote registry if configured and not --pull only
	gitPushed := false
	var pushedRemoteSkills []string
	if remoteTarget != "" && !pullOnlyFlag && len(discovered) > 0 {
		pushArgs := []string{remoteTarget, "--all"}
		if effectiveGlobal {
			pushArgs = append(pushArgs, "-g")
		}
		if forceFlag {
			pushArgs = append(pushArgs, "--force")
		}
		var pOut, pErr bytes.Buffer
		subApp := *a
		subApp.Stdout = &pOut
		subApp.Stderr = &pErr
		if code := subApp.runProfilePush(pushArgs); code == ExitSuccess {
			gitPushed = true
			for name := range discovered {
				pushedRemoteSkills = append(pushedRemoteSkills, name)
			}
			sort.Strings(pushedRemoteSkills)
		}
	} else if pushFlag {
		// Commit & push in rootDir if --push is requested on a local git repo without remoteTarget
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
			"status":        "synced",
			"manifest":      manifestPath,
			"remote_target": remoteTarget,
			"skills":        len(updatedManifest.Skills),
			"pulled_skills": pulledSkills,
			"pushed_skills": pushedRemoteSkills,
			"projections":   len(projections),
			"git_pushed":    gitPushed,
		})
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "synced\t%s\t%d\t%d\n",
			sanitizeTSVField(manifestPath), len(updatedManifest.Skills), len(projections))
	default:
		_, _ = fmt.Fprintf(a.Stdout, "✓ Synced profile at %s (%d skill(s), %d harness projection(s))\n",
			manifestPath, len(updatedManifest.Skills), len(projections))
		if remoteTarget != "" {
			if len(pulledSkills) > 0 {
				_, _ = fmt.Fprintf(a.Stdout, "  ✓ Pulled %d skill(s) from %s\n", len(pulledSkills), remoteTarget)
			}
			if gitPushed {
				_, _ = fmt.Fprintf(a.Stdout, "  ✓ Pushed %d skill(s) and updated skills.json in %s\n", len(pushedRemoteSkills), remoteTarget)
			}
		} else if gitPushed {
			_, _ = fmt.Fprintln(a.Stdout, "  ✓ Committed and pushed skills.json to remote Git repository")
		}
	}
	return ExitSuccess
}

// profilePushOptions holds the parsed flags for `dojo profile push`.
type profilePushOptions struct {
	repo   string
	all    bool
	global bool
	force  bool
	dryRun bool
	json   bool
	raw    bool
}

// locatedPushItem is one skill package resolved on disk, with an optional
// cleanup for temporary packages.
type locatedPushItem struct {
	name    string
	dir     string
	cleanup func()
}

// runProfilePush packages local skills, rules (.mdc/.md), or slash commands/workflows
// and pushes them (along with an updated skills.json) to a target directory or GitHub profile repository
// (e.g. `dojo profile push github:jadmadi/agy-vm my-skill my-command` or `dojo profile push --all`).
func (a *App) runProfilePush(args []string) int {
	opts, posArgs, ok := a.parseProfilePushArgs(args)
	if !ok {
		return ExitUsage
	}

	effectiveGlobal := a.resolveEffectiveGlobalFlag(opts.global)
	targetSpec, rawSpecs, allFlag := a.planProfilePushTarget(opts, posArgs)
	if targetSpec == "" {
		_, _ = fmt.Fprintln(a.Stderr, "error: profile push requires a target repository/directory (or a linked registry via 'dojo profile link') and at least one skill/command name (or --all)\nusage: dojo profile push [github:owner/repo | dir] <skill-or-command...>")
		return ExitUsage
	}

	skillSpecs := a.resolveSkillSpecs(rawSpecs)
	searchRoots := profileSearchRoots(effectiveGlobal)
	reconcileProfileRoots(searchRoots)

	if allFlag && len(skillSpecs) == 0 {
		skillSpecs = discoverAllSkillNames(effectiveGlobal)
	}
	if len(skillSpecs) == 0 {
		_, _ = fmt.Fprintln(a.Stderr, "error: specify at least one skill, rule, or command to push (or pass --all)\nexample: dojo profile push github:jadmadi/agy-vm my-skill my-command")
		return ExitUsage
	}

	items, code := a.locatePushItems(skillSpecs, searchRoots)
	if code != ExitSuccess {
		return code
	}
	defer cleanupPushItems(items)

	if !opts.force {
		if code := a.scanPushItems(items); code != ExitSuccess {
			return code
		}
	}

	if opts.dryRun {
		a.writeProfilePushDryRun(opts, targetSpec, items)
		return ExitSuccess
	}

	workTree, isRemoteClone, cleanupTree, code := a.resolvePushWorkTree(targetSpec)
	if code != ExitSuccess {
		return code
	}
	defer cleanupTree()

	pushedNames, code := a.upsertPushedSkills(workTree, items)
	if code != ExitSuccess {
		return code
	}

	gitPushed, code := a.commitAndPushProfile(workTree, pushedNames, targetSpec, isRemoteClone)
	if code != ExitSuccess {
		return code
	}

	a.writeProfilePushReport(opts, targetSpec, pushedNames, gitPushed)
	return ExitSuccess
}

func (a *App) parseProfilePushArgs(args []string) (profilePushOptions, []string, bool) {
	var opts profilePushOptions
	fs := flag.NewFlagSet("profile push", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	fs.StringVar(&opts.repo, "repo", "", "Target GitHub repository (github:owner/repo) or local profile directory")
	fs.StringVar(&opts.repo, "r", "", "Target GitHub repository or directory (shorthand)")
	fs.BoolVar(&opts.all, "all", false, "Push all installed skills in workspace (or global with -g)")
	fs.BoolVar(&opts.all, "a", false, "Push all installed skills (shorthand)")
	fs.BoolVar(&opts.global, "global", false, "Search global scope first")
	fs.BoolVar(&opts.global, "g", false, "Search global scope first (shorthand)")
	fs.BoolVar(&opts.force, "force", false, "Skip pre-push security/secret scan")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "Report the packages that would be pushed without copying, committing, or pushing")
	fs.BoolVar(&opts.force, "f", false, "Skip pre-push security/secret scan (shorthand)")
	fs.BoolVar(&opts.json, "json", false, "Output in JSON format")
	fs.BoolVar(&opts.raw, "raw", false, "Output in raw tab-separated format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return opts, nil, false
	}
	return opts, posArgs, true
}

// planProfilePushTarget decides which argument is the destination and which
// are skill specs, falling back to the linked registry.
func (a *App) planProfilePushTarget(opts profilePushOptions, posArgs []string) (targetSpec string, rawSpecs []string, all bool) {
	all = opts.all
	targetSpec = strings.TrimSpace(opts.repo)

	if targetSpec != "" {
		return targetSpec, posArgs, all
	}

	if len(posArgs) > 0 {
		first := strings.TrimSpace(posArgs[0])
		if isProfilePushTarget(first) {
			return first, posArgs[1:], all
		}
		if a.Config != nil && a.Config.GetProfileRepo() != "" {
			return a.Config.GetProfileRepo(), posArgs, all
		}
		return first, posArgs[1:], all
	}

	if a.Config != nil && a.Config.GetProfileRepo() != "" {
		return a.Config.GetProfileRepo(), nil, true
	}
	return "", nil, all
}

func isProfilePushTarget(first string) bool {
	if strings.HasPrefix(first, "github:") || strings.HasPrefix(first, "http://") || strings.HasPrefix(first, "https://") {
		return true
	}
	if info, statErr := os.Stat(first); statErr == nil && info.IsDir() {
		if _, mdErr := os.Stat(filepath.Join(first, "SKILL.md")); mdErr != nil {
			return true
		}
	}
	if strings.Count(first, "/") == 1 && !strings.ContainsAny(first, ",#") {
		if ghTarget, ok := catalog.ParseGitHubTarget(first); ok && ghTarget.Subpath == "" {
			return true
		}
	}
	return false
}

// profileSearchRoots orders the workspace and home directories by requested scope.
func profileSearchRoots(effectiveGlobal bool) []string {
	wd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()
	if effectiveGlobal {
		return []string{homeDir, wd}
	}
	return []string{wd, homeDir}
}

func reconcileProfileRoots(roots []string) {
	for _, r := range roots {
		if r != "" {
			_ = harness.ReconcileHarnesses(r)
		}
	}
}

func discoverAllSkillNames(effectiveGlobal bool) []string {
	wd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()
	primaryRoot := wd
	if effectiveGlobal && homeDir != "" {
		primaryRoot = homeDir
	}
	var names []string
	for name := range harness.DiscoverInstalledSkillDirs(primaryRoot) {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (a *App) locatePushItems(skillSpecs, searchRoots []string) ([]locatedPushItem, int) {
	var items []locatedPushItem
	for _, spec := range skillSpecs {
		name, dir, cleanup, lErr := harness.LocateOrPackageLocalSkill(spec, searchRoots)
		if lErr == nil {
			items = append(items, locatedPushItem{name: name, dir: dir, cleanup: cleanup})
			continue
		}

		item, code := a.packageCatalogSkill(spec)
		if code != ExitSuccess {
			cleanupPushItems(items)
			_, _ = fmt.Fprintf(a.Stderr, "error: %v\n", lErr)
			return nil, ExitNotFound
		}
		items = append(items, item)
	}
	return items, ExitSuccess
}

// packageCatalogSkill builds a temporary skill package from the catalog when a
// skill is not present in any local harness directory.
func (a *App) packageCatalogSkill(spec string) (locatedPushItem, int) {
	if a.Store == nil {
		return locatedPushItem{}, ExitNotFound
	}
	item, err := a.Store.Get(spec)
	if err != nil || item == nil {
		return locatedPushItem{}, ExitNotFound
	}

	tmpDir, mErr := os.MkdirTemp("", "dojo-cat-pkg-*")
	if mErr != nil {
		return locatedPushItem{}, ExitGeneral
	}
	pkgDir := filepath.Join(tmpDir, item.Name)
	content, cErr := a.loadItemContent(item)
	if cErr != nil {
		_ = os.RemoveAll(tmpDir)
		return locatedPushItem{}, ExitGeneral
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	_, instErr := runner.InstallSkillContext(ctx, *item, pkgDir, content, itemResourceCandidates(a, item), a.getHTTPClient())
	cancel()
	if instErr != nil {
		_ = os.RemoveAll(tmpDir)
		return locatedPushItem{}, ExitGeneral
	}

	return locatedPushItem{
		name:    item.Name,
		dir:     pkgDir,
		cleanup: func() { _ = os.RemoveAll(tmpDir) },
	}, ExitSuccess
}

func cleanupPushItems(items []locatedPushItem) {
	for _, it := range items {
		if it.cleanup != nil {
			it.cleanup()
		}
	}
}

func (a *App) scanPushItems(items []locatedPushItem) int {
	for _, it := range items {
		findings := ScanDirectorySecurity(it.name, it.dir)
		if len(findings) == 0 {
			continue
		}
		for _, f := range findings {
			_, _ = fmt.Fprintf(a.Stderr, "error: pre-push security scan blocked skill %q: %s (use --force to override)\n", it.name, f.Message)
		}
		return ExitGeneral
	}
	return ExitSuccess
}

// resolvePushWorkTree returns a local directory to write into, cloning a GitHub
// repository when a remote target is requested.
func (a *App) resolvePushWorkTree(targetSpec string) (workTree string, isRemoteClone bool, cleanup func(), code int) {
	if info, statErr := os.Stat(targetSpec); statErr == nil && info.IsDir() {
		return targetSpec, false, func() {}, ExitSuccess
	}

	ghTarget, ok := catalog.ParseGitHubTarget(strings.TrimPrefix(targetSpec, "github:"))
	if !ok {
		_, _ = fmt.Fprintf(a.Stderr, "error: target %q is neither an existing local directory nor a valid GitHub repository (github:owner/repo)\n", targetSpec)
		return "", false, nil, ExitUsage
	}

	tmpClone, tErr := os.MkdirTemp("", "dojo-profile-push-*")
	if tErr != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: creating temporary directory: %v\n", tErr)
		return "", false, nil, ExitGeneral
	}
	cleanup = func() { _ = os.RemoveAll(tmpClone) }

	if code := a.cloneProfileTarget(ghTarget, tmpClone); code != ExitSuccess {
		cleanup()
		return "", false, nil, code
	}
	return tmpClone, true, cleanup, ExitSuccess
}

func (a *App) cloneProfileTarget(ghTarget *catalog.GitHubTarget, dest string) int {
	if cloneWithGH(ghTarget, dest) {
		return ExitSuccess
	}

	gitBin, gErr := exec.LookPath("git")
	if gErr != nil {
		_, _ = fmt.Fprintln(a.Stderr, "error: neither 'gh' nor 'git' is available to clone target repository")
		return ExitGeneral
	}
	cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", ghTarget.Owner, ghTarget.Repo)
	cmd := exec.Command(gitBin, "clone", "--depth", "1", cloneURL, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: cloning %s: %v (%s)\n", ghTarget.RepoString(), err, strings.TrimSpace(string(out)))
		return ExitGeneral
	}
	return ExitSuccess
}

func cloneWithGH(ghTarget *catalog.GitHubTarget, dest string) bool {
	ghBin, ghErr := exec.LookPath("gh")
	if ghErr != nil {
		if _, sErr := os.Stat("/opt/antigravity/bin/gh"); sErr == nil {
			ghBin = "/opt/antigravity/bin/gh"
			ghErr = nil
		}
	}
	if ghErr != nil || ghBin == "" {
		return false
	}

	cloneArgs := []string{"repo", "clone", ghTarget.RepoString(), dest, "--", "--depth", "1"}
	if ghTarget.Ref != "" && ghTarget.Ref != "main" {
		cloneArgs = append(cloneArgs, "-b", ghTarget.Ref)
	}
	return exec.Command(ghBin, cloneArgs...).Run() == nil
}

// upsertPushedSkills copies each package into <workTree>/skills/<name>/ and
// rewrites skills.json in sorted order.
func (a *App) upsertPushedSkills(workTree string, items []locatedPushItem) ([]string, int) {
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
			return nil, ExitGeneral
		}

		entry := existingByName[it.name]
		entry.Name = it.name
		if ver := skillManifestVersion(destSkillDir); ver != "" {
			entry.Version = ver
		}
		existingByName[it.name] = entry
		pushedNames = append(pushedNames, it.name)
	}

	allNames := make([]string, 0, len(existingByName))
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
		return nil, ExitGeneral
	}
	return pushedNames, ExitSuccess
}

func skillManifestVersion(skillDir string) string {
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return ""
	}
	fm, _, err := catalog.ParseFrontmatter(data)
	if err != nil || fm == nil || fm.Metadata == nil {
		return ""
	}
	return fm.Metadata["version"]
}

// commitAndPushProfile commits the working tree when it is a git repository.
func (a *App) commitAndPushProfile(workTree string, pushedNames []string, targetSpec string, isRemoteClone bool) (bool, int) {
	if _, gitStatErr := os.Stat(filepath.Join(workTree, ".git")); gitStatErr != nil {
		return false, ExitSuccess
	}
	gitBin, gErr := exec.LookPath("git")
	if gErr != nil || gitBin == "" {
		return false, ExitSuccess
	}

	_ = exec.Command(gitBin, "-C", workTree, "add", "-f", profile.DefaultManifestFilename, "skills").Run()
	statusOut, _ := exec.Command(gitBin, "-C", workTree, "status", "--porcelain").Output()
	if len(bytes.TrimSpace(statusOut)) == 0 {
		return true, ExitSuccess
	}

	commitMsg := fmt.Sprintf("feat(dojo): push skill(s) %s to profile", strings.Join(pushedNames, ", "))
	_ = exec.Command(gitBin, "-C", workTree, "commit", "-m", commitMsg).Run()
	pushOut, pErr := exec.Command(gitBin, "-C", workTree, "push").CombinedOutput()
	if pErr == nil {
		return true, ExitSuccess
	}
	if isRemoteClone {
		_, _ = fmt.Fprintf(a.Stderr, "error: git push to %s failed: %v (%s)\n", targetSpec, pErr, strings.TrimSpace(string(pushOut)))
		return false, ExitGeneral
	}
	return false, ExitSuccess
}

// writeProfilePushDryRun reports the packages and target without writing.
func (a *App) writeProfilePushDryRun(opts profilePushOptions, targetSpec string, items []locatedPushItem) {
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.name)
	}
	sort.Strings(names)

	switch a.resolveMode(opts.json, opts.raw) {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"status":  "planned",
			"target":  targetSpec,
			"skills":  names,
			"scanned": !opts.force,
		})
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "planned\t%s\t%s\n", sanitizeTSVField(targetSpec), strings.Join(names, ","))
	default:
		_, _ = fmt.Fprintf(a.Stdout, "DRY RUN: would push %d skill(s) to %s\n", len(names), targetSpec)
		for _, name := range names {
			_, _ = fmt.Fprintf(a.Stdout, "  • %s\n", name)
		}
	}
}

func (a *App) writeProfilePushReport(opts profilePushOptions, targetSpec string, pushedNames []string, gitPushed bool) {
	switch a.resolveMode(opts.json, opts.raw) {
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
}
