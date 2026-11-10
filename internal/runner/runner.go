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

package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/version"
)

var (
	ErrScriptNotFound   = errors.New("script not found")
	ErrFileNotFound     = errors.New("file not found")
	ErrBinaryFile       = errors.New("cannot stream binary file as text")
	ErrInvalidItemPath  = errors.New("invalid item path")
	ErrInvalidPath      = errors.New("invalid path or directory traversal detected")
	ErrExecutionTimeout = errors.New("script execution timed out")
	ErrRuntimeNotFound  = errors.New("interpreter runtime not found in PATH")
)

// GetCandidateBaseDirs returns all existing local directories where a skill might be located.
func GetCandidateBaseDirs(item catalog.Item) []string {
	var dirs []string
	add := func(d string) {
		if d == "" {
			return
		}
		d = filepath.Clean(d)
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			if slices.Contains(dirs, d) {
				return
			}
			dirs = append(dirs, d)
		}
	}

	// 1. Workspace .agents/skills/<name> or <id>
	if wd, err := os.Getwd(); err == nil {
		if item.Name != "" {
			add(filepath.Join(wd, ".agents", "skills", item.Name))
		}
		if item.ID != "" && item.ID != item.Name {
			add(filepath.Join(wd, ".agents", "skills", item.ID))
		}
	}

	// 2. User Home ~/.agents/skills/<name> or <id>
	if homeDir, err := os.UserHomeDir(); err == nil {
		if item.Name != "" {
			add(filepath.Join(homeDir, ".agents", "skills", item.Name))
		}
		if item.ID != "" && item.ID != item.Name {
			add(filepath.Join(homeDir, ".agents", "skills", item.ID))
		}
	}

	// 3. Cache directory ~/.cache/dojo/skills/<name> or <id>
	if cacheRoot := resolveCacheRoot(); cacheRoot != "" {
		expectedDigest := item.EffectiveDigest()
		canUseCache := func(skillName string) bool {
			if item.Type == catalog.ItemTypeSkillMD && item.Source != "local" {
				return false
			}
			if catalog.IsArchiveItem(item) && expectedDigest != "" {
				return catalog.IsArchiveCachedWithDigest(cacheRoot, skillName, expectedDigest)
			}
			return true
		}
		if item.Name != "" && canUseCache(item.Name) {
			add(filepath.Join(cacheRoot, "skills", item.Name))
		}
		if item.ID != "" && item.ID != item.Name && canUseCache(item.ID) {
			add(filepath.Join(cacheRoot, "skills", item.ID))
		}
	}

	// 4. Explicit item.Path
	if item.Path != "" {
		if fi, err := os.Stat(item.Path); err == nil {
			if fi.IsDir() {
				add(item.Path)
			} else {
				add(filepath.Dir(item.Path))
			}
		}
	}

	return dirs
}

var (
	cacheDirMu       sync.RWMutex
	cacheDirOverride string
)

// SetCacheDir sets the cache root directory override used by runner operations.
func SetCacheDir(dir string) {
	cacheDirMu.Lock()
	cacheDirOverride = strings.TrimSpace(dir)
	cacheDirMu.Unlock()
}

func resolveCacheRoot() string {
	cacheDirMu.RLock()
	override := cacheDirOverride
	cacheDirMu.RUnlock()
	if override != "" {
		return override
	}
	return catalog.DefaultCacheDir()
}

func skillCacheKey(item catalog.Item) string {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		name = strings.TrimSpace(item.ID)
	}
	if name == "" {
		return ""
	}
	clean := filepath.Base(filepath.Clean(name))
	if clean == "" || clean == "." || clean == ".." {
		return ""
	}
	return clean
}

// CacheSkillFile saves a skill file to ~/.cache/dojo/skills/<skill>/<relPath>.
func CacheSkillFile(skillName, relPath string, data []byte) {
	cacheSkillFile(skillName, relPath, data)
}

func cacheSkillFile(skillName, relPath string, data []byte) {
	if skillName == "" || relPath == "" || len(data) == 0 {
		return
	}
	cleanSkillName := filepath.Base(filepath.Clean(skillName))
	if cleanSkillName == "" || cleanSkillName == "." || cleanSkillName == ".." {
		return
	}
	cleanRel := filepath.ToSlash(filepath.Clean(relPath))
	if strings.HasPrefix(cleanRel, "../") || strings.Contains(cleanRel, "/../") || strings.HasPrefix(cleanRel, "/") {
		return
	}
	cacheRoot := resolveCacheRoot()
	if cacheRoot == "" {
		return
	}
	targetPath := filepath.Join(cacheRoot, "skills", cleanSkillName, filepath.FromSlash(cleanRel))
	_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
	perm := os.FileMode(0644)
	if strings.HasPrefix(cleanRel, "scripts/") || strings.HasSuffix(cleanRel, ".sh") || strings.HasSuffix(cleanRel, ".py") || strings.HasSuffix(cleanRel, ".js") {
		perm = 0755
	}
	if err := os.WriteFile(targetPath, data, perm); err == nil {
		_ = os.Chmod(targetPath, perm)
	}
}

func isContained(baseDir, targetPath string) bool {
	return catalog.IsPathContained(baseDir, targetPath)
}

func detectInterpreter(scriptPath string) (string, []string, error) {
	ext := strings.ToLower(filepath.Ext(scriptPath))

	if f, err := os.Open(scriptPath); err == nil {
		buf := make([]byte, 512)
		n, _ := f.Read(buf)
		_ = f.Close()
		if n > 2 && string(buf[:2]) == "#!" {
			shebang := string(buf[:n])
			if idx := strings.Index(shebang, "\n"); idx != -1 {
				shebang = shebang[:idx]
			}
			shebang = strings.TrimSpace(strings.TrimPrefix(shebang, "#!"))
			parts := strings.Fields(shebang)
			if len(parts) > 0 {
				var interpName string
				var interpFlags []string
				if filepath.Base(parts[0]) == "env" && len(parts) > 1 {
					if parts[1] == "-S" && len(parts) > 2 {
						interpName = parts[2]
						interpFlags = parts[3:]
					} else {
						interpName = parts[1]
						interpFlags = parts[2:]
					}
				} else {
					interpName = parts[0]
					interpFlags = parts[1:]
				}

				base := filepath.Base(interpName)
				switch {
				case strings.HasPrefix(base, "python"):
					return resolvePythonCommand(scriptPath, interpFlags...)
				case strings.HasPrefix(base, "node") || strings.HasPrefix(base, "bun") || strings.HasPrefix(base, "deno") || strings.HasPrefix(base, "tsx"):
					var bin string
					if found, err := exec.LookPath(base); err == nil {
						bin = found
					} else if found, err := exec.LookPath(interpName); err == nil {
						bin = found
					} else {
						return "", nil, fmt.Errorf("%w: shebang interpreter %q not found in PATH", ErrRuntimeNotFound, interpName)
					}
					return bin, append(append([]string{}, interpFlags...), scriptPath), nil
				case strings.HasPrefix(base, "bash") || strings.HasPrefix(base, "sh") || strings.HasPrefix(base, "zsh"):
					var bin string
					if found, err := exec.LookPath(base); err == nil {
						bin = found
					} else if found, err := exec.LookPath(interpName); err == nil {
						bin = found
					} else {
						return "", nil, fmt.Errorf("%w: shebang interpreter %q not found in PATH", ErrRuntimeNotFound, interpName)
					}
					return bin, append(append([]string{}, interpFlags...), scriptPath), nil
				default:
					var bin string
					if found, err := exec.LookPath(base); err == nil {
						bin = found
					} else if found, err := exec.LookPath(interpName); err == nil {
						bin = found
					} else {
						return "", nil, fmt.Errorf("%w: shebang interpreter %q not found in PATH", ErrRuntimeNotFound, interpName)
					}
					return bin, append(append([]string{}, interpFlags...), scriptPath), nil
				}
			}
		}
	}

	switch ext {
	case ".py":
		return resolvePythonCommand(scriptPath)
	case ".js", ".mjs", ".cjs":
		if bin, err := exec.LookPath("node"); err == nil {
			return bin, []string{scriptPath}, nil
		}
		if bin, err := exec.LookPath("bun"); err == nil {
			return bin, []string{"run", scriptPath}, nil
		}
		if bin, err := exec.LookPath("deno"); err == nil {
			return bin, []string{"run", scriptPath}, nil
		}
		return "", nil, fmt.Errorf("%w: JavaScript runtime not found (checked node, bun, deno in PATH)", ErrRuntimeNotFound)
	case ".ts":
		if bin, err := exec.LookPath("bun"); err == nil {
			return bin, []string{"run", scriptPath}, nil
		}
		if bin, err := exec.LookPath("deno"); err == nil {
			return bin, []string{"run", scriptPath}, nil
		}
		if bin, err := exec.LookPath("tsx"); err == nil {
			return bin, []string{scriptPath}, nil
		}
		if bin, err := exec.LookPath("node"); err == nil {
			return bin, []string{scriptPath}, nil
		}
		return "", nil, fmt.Errorf("%w: TypeScript runtime not found (checked bun, deno, tsx, node in PATH)", ErrRuntimeNotFound)
	case ".sh", ".bash":
		if bin, err := exec.LookPath("bash"); err == nil {
			return bin, []string{scriptPath}, nil
		}
		if bin, err := exec.LookPath("sh"); err == nil {
			return bin, []string{scriptPath}, nil
		}
		return "", nil, fmt.Errorf("%w: shell runtime not found (checked bash, sh in PATH)", ErrRuntimeNotFound)
	default:
		return scriptPath, nil, nil
	}
}

func resolvePythonCommand(scriptPath string, interpFlags ...string) (string, []string, error) {
	if bin, err := exec.LookPath("uv"); err == nil {
		return bin, []string{"run", scriptPath}, nil
	}
	if bin, err := exec.LookPath("python3"); err == nil {
		return bin, append(append([]string{}, interpFlags...), scriptPath), nil
	}
	if bin, err := exec.LookPath("python"); err == nil {
		return bin, append(append([]string{}, interpFlags...), scriptPath), nil
	}
	return "", nil, fmt.Errorf("%w: Python runtime not found (checked uv, python3, python in PATH)", ErrRuntimeNotFound)
}

// RunScriptContext executes a bundled script for a catalog item with context cancellation and custom I/O.
// It searches local candidate directories first; if not found, it downloads/caches the script on demand.
func RunScriptContext(ctx context.Context, item catalog.Item, scriptName string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cleanName := filepath.Clean(strings.TrimSpace(scriptName))
	if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, "..") {
		return fmt.Errorf("%w: %s", ErrInvalidPath, scriptName)
	}

	var scriptPath string
	var baseDir string

	// 1. Search local candidate directories
	baseDirs := GetCandidateBaseDirs(item)
	for _, dir := range baseDirs {
		candidates := []string{
			filepath.Join(dir, "scripts", cleanName),
			filepath.Join(dir, cleanName),
		}
		for _, cand := range candidates {
			candClean := filepath.Clean(cand)
			if !isContained(dir, candClean) {
				continue
			}
			info, err := os.Stat(candClean)
			if err == nil && !info.IsDir() {
				scriptPath = candClean
				baseDir = dir
				break
			}
		}
		if scriptPath != "" {
			break
		}
	}

	// 2. If not found locally, fetch JIT via ReadBytesContext and cache it
	if scriptPath == "" {
		var fetchTarget string
		if strings.HasPrefix(cleanName, "scripts/") {
			fetchTarget = cleanName
		} else {
			fetchTarget = "scripts/" + cleanName
		}

		data, err := ReadBytesContext(ctx, item, fetchTarget, nil)
		if err != nil && fetchTarget != cleanName {
			data, err = ReadBytesContext(ctx, item, cleanName, nil)
			if err == nil {
				fetchTarget = cleanName
			}
		}

		if err == nil && len(data) > 0 {
			cacheRoot := resolveCacheRoot()
			sKey := skillCacheKey(item)
			if cacheRoot == "" || sKey == "" {
				return fmt.Errorf("runner: unable to determine cache directory")
			}
			cachedPath := filepath.Join(cacheRoot, "skills", sKey, filepath.FromSlash(fetchTarget))
			if err := os.MkdirAll(filepath.Dir(cachedPath), 0755); err != nil {
				return fmt.Errorf("runner: creating cache directory for %s: %w", cachedPath, err)
			}
			if err := os.WriteFile(cachedPath, data, 0755); err != nil {
				return fmt.Errorf("runner: writing cached script %s: %w", cachedPath, err)
			}
			if err := os.Chmod(cachedPath, 0755); err != nil {
				return fmt.Errorf("runner: setting permissions on %s: %w", cachedPath, err)
			}
			scriptPath = cachedPath
			baseDir = filepath.Join(cacheRoot, "skills", sKey)
		}
	}

	if scriptPath == "" {
		return fmt.Errorf("%w: script %q not found in skill %q", ErrScriptNotFound, scriptName, item.Name)
	}

	bin, binArgs, err := detectInterpreter(scriptPath)
	if err != nil {
		return fmt.Errorf("runner: %w", err)
	}
	allArgs := append(binArgs, args...)

	cmd := exec.CommandContext(ctx, bin, allArgs...)
	cmd.WaitDelay = 2 * time.Second
	configureProcessGroup(cmd)

	// Set execution working directory: prefer workspace cwd so tools operate on user files
	if wd, err := os.Getwd(); err == nil && wd != "" {
		cmd.Dir = wd
	} else if baseDir != "" {
		cmd.Dir = baseDir
	}

	// Build environment variables with PYTHONPATH / NODE_PATH pointing to script and skill dir
	env := os.Environ()
	scriptDir := filepath.Dir(scriptPath)
	env = appendEnvPath(env, "PYTHONPATH", scriptDir, baseDir)
	env = appendEnvPath(env, "NODE_PATH", scriptDir, baseDir)
	cmd.Env = env

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if stdin != nil {
		cmd.Stdin = stdin
	}

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("runner: %w: %v", ErrExecutionTimeout, err)
		}
		return fmt.Errorf("runner: execution failed: %w", err)
	}
	return nil
}

func appendEnvPath(env []string, key string, dirs ...string) []string {
	var validDirs []string
	for _, d := range dirs {
		if d != "" {
			validDirs = append(validDirs, d)
		}
	}
	if len(validDirs) == 0 {
		return env
	}
	extra := strings.Join(validDirs, string(os.PathListSeparator))

	prefix := key + "="
	for i, e := range env {
		if after, ok := strings.CutPrefix(e, prefix); ok {
			current := after
			if current != "" {
				env[i] = prefix + extra + string(os.PathListSeparator) + current
			} else {
				env[i] = prefix + extra
			}
			return env
		}
	}
	return append(env, prefix+extra)
}

// RunScript executes a bundled script for a catalog item.
func RunScript(item catalog.Item, scriptName string, args []string, stdout, stderr io.Writer) error {
	return RunScriptContext(context.Background(), item, scriptName, args, os.Stdin, stdout, stderr)
}

func deriveBaseURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}

	// GitHub blob / tree handling:
	// https://github.com/owner/repo/blob/branch/path/SKILL.md -> https://raw.githubusercontent.com/owner/repo/branch/path
	if strings.HasPrefix(trimmed, "https://github.com/") || strings.HasPrefix(trimmed, "http://github.com/") {
		u, err := url.Parse(trimmed)
		if err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 4 && (parts[2] == "blob" || parts[2] == "tree") {
				owner := parts[0]
				repo := parts[1]
				branch := parts[3]
				rest := parts[4:]
				cleanRest := strings.Join(rest, "/")
				if strings.HasSuffix(strings.ToLower(cleanRest), ".md") {
					cleanRest = path.Dir(cleanRest)
					if cleanRest == "." {
						cleanRest = ""
					}
				}
				if cleanRest != "" {
					return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, cleanRest)
				}
				return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", owner, repo, branch)
			}
		}
	}

	// raw.githubusercontent.com handling:
	if strings.HasPrefix(trimmed, "https://raw.githubusercontent.com/") || strings.HasPrefix(trimmed, "http://raw.githubusercontent.com/") {
		if u, err := url.Parse(trimmed); err == nil {
			if strings.HasSuffix(strings.ToLower(u.Path), ".md") {
				u.Path = path.Dir(u.Path)
			}
			u.RawQuery = ""
			u.Fragment = ""
			return strings.TrimSuffix(u.String(), "/")
		}
	}

	if u, err := url.Parse(trimmed); err == nil {
		if strings.HasSuffix(strings.ToLower(u.Path), ".md") {
			u.Path = path.Dir(u.Path)
		}
		u.RawQuery = ""
		u.Fragment = ""
		return strings.TrimSuffix(u.String(), "/")
	}
	baseURL := strings.TrimSuffix(trimmed, "/SKILL.md")
	baseURL = strings.TrimSuffix(baseURL, "/skill.md")
	return strings.TrimSuffix(baseURL, "/")
}

// ReadBytesContext reads any skill file (text or binary) from local dirs, cache, or remote URL.
func ReadBytesContext(ctx context.Context, item catalog.Item, filePath string, client *http.Client) ([]byte, error) {
	return readBytesInternal(ctx, item, filePath, client, "", false)
}

// readBytesInternal resolves any skill file from local dirs, the archive cache,
// or the remote origin.
func readBytesInternal(ctx context.Context, item catalog.Item, filePath string, client *http.Client, skipDir string, preferRemote bool) ([]byte, error) {
	cleanPath, err := validateReadPath(filePath)
	if err != nil {
		return nil, err
	}
	cleanSkip := cleanSkipDir(skipDir)

	if !preferRemote {
		if data, ok := readFromLocalDirs(item, cleanPath, cleanSkip); ok {
			return data, nil
		}
	}

	sKey := skillCacheKey(item)
	if remoteURL := itemRemoteURL(item); remoteURL != "" {
		if catalog.IsArchiveItem(item) {
			return readFromArchive(ctx, item, remoteURL, sKey, cleanPath, client)
		}
		if baseURL := deriveBaseURL(remoteURL); baseURL != "" {
			if client == nil {
				client = &http.Client{Timeout: 15 * time.Second}
			}
			if data, ok := readRemoteFileCandidates(ctx, client, baseURL, cleanPath, sKey); ok {
				return data, nil
			}
		}
	}

	if preferRemote {
		if data, ok := readFromLocalDirs(item, cleanPath, cleanSkip); ok {
			return data, nil
		}
	}

	return nil, fmt.Errorf("runner: %w: %s in skill %s", ErrFileNotFound, filePath, item.Name)
}

func validateReadPath(filePath string) (string, error) {
	trimmed := strings.TrimSpace(filePath)
	if trimmed == "" || trimmed == "." || trimmed == ".." {
		return "", fmt.Errorf("runner: %w: %s", ErrInvalidPath, filePath)
	}

	cleanPath := filepath.ToSlash(filepath.Clean(trimmed))
	cleanPath = strings.TrimPrefix(cleanPath, "./")
	if strings.Contains(cleanPath, "..") ||
		filepath.IsAbs(trimmed) ||
		strings.HasPrefix(cleanPath, "/") ||
		strings.HasPrefix(trimmed, "\\") ||
		strings.Contains(trimmed, ":") {
		return "", fmt.Errorf("runner: %w: %s", ErrInvalidPath, filePath)
	}
	return cleanPath, nil
}

func cleanSkipDir(skipDir string) string {
	if skipDir == "" {
		return ""
	}
	return filepath.Clean(skipDir)
}

func itemRemoteURL(item catalog.Item) string {
	if item.URL != "" {
		return item.URL
	}
	if item.Metadata != nil {
		return item.Metadata["canonical"]
	}
	return ""
}

// fileCandidates returns the direct path plus the scripts/ and references/
// fallbacks used for both local and cached lookups.
func fileCandidates(baseDir, cleanPath string) []string {
	candidates := []string{
		filepath.Join(baseDir, filepath.FromSlash(cleanPath)),
	}
	if !strings.HasPrefix(cleanPath, "scripts/") {
		candidates = append(candidates, filepath.Join(baseDir, "scripts", filepath.FromSlash(cleanPath)))
	}
	if !strings.HasPrefix(cleanPath, "references/") {
		candidates = append(candidates, filepath.Join(baseDir, "references", filepath.FromSlash(cleanPath)))
	}
	return candidates
}

func readFromLocalDirs(item catalog.Item, cleanPath, cleanSkip string) ([]byte, bool) {
	for _, baseDir := range GetCandidateBaseDirs(item) {
		if cleanSkip != "" && filepath.Clean(baseDir) == cleanSkip {
			// Only allow reading from cleanSkip if item.Path explicitly points inside cleanSkip.
			if item.Path == "" || (!catalog.IsPathContained(cleanSkip, item.Path) && filepath.Clean(item.Path) != cleanSkip) {
				continue
			}
		}

		for _, candPath := range fileCandidates(baseDir, cleanPath) {
			candClean := filepath.Clean(candPath)
			if !isContained(baseDir, candClean) {
				continue
			}
			if data, err := os.ReadFile(candClean); err == nil {
				return data, true
			}
		}
	}
	return nil, false
}

// readFromArchive downloads and unpacks the skill archive into the cache, then
// reads the requested file from it.
func readFromArchive(ctx context.Context, item catalog.Item, remoteURL, sKey, cleanPath string, client *http.Client) ([]byte, error) {
	cacheRoot := resolveCacheRoot()
	if cacheRoot == "" || sKey == "" {
		return nil, fmt.Errorf("runner: %w: %s in skill %s", ErrFileNotFound, cleanPath, item.Name)
	}
	cacheDir := filepath.Join(cacheRoot, "skills", sKey)
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	data, err := downloadArchive(ctx, client, remoteURL, sKey)
	if err != nil {
		return nil, err
	}

	expectedDigest := item.EffectiveDigest()
	if expectedDigest != "" {
		if vErr := catalog.VerifyDigest(data, expectedDigest); vErr != nil {
			return nil, fmt.Errorf("runner: %w: %v", catalog.ErrDigestMismatch, vErr)
		}
	}
	if _, uErr := catalog.UnpackArchiveToCache(data, cacheRoot, sKey, expectedDigest); uErr != nil {
		return nil, fmt.Errorf("runner: unpacking archive for %s: %w", sKey, uErr)
	}

	for _, cachedTarget := range fileCandidates(cacheDir, cleanPath) {
		if !isContained(cacheDir, cachedTarget) {
			continue
		}
		if fileBytes, readErr := os.ReadFile(cachedTarget); readErr == nil {
			return fileBytes, nil
		}
	}
	return nil, fmt.Errorf("runner: %w: %s in skill %s", ErrFileNotFound, cleanPath, item.Name)
}

func downloadArchive(ctx context.Context, client *http.Client, remoteURL, sKey string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("runner: creating archive request for %s: %w", sKey, err)
	}
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("runner: downloading archive for %s: %w", sKey, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("runner: downloading archive for %s: HTTP %d", sKey, resp.StatusCode)
	}

	data, readErr := catalog.ReadBounded(resp.Body, catalog.DefaultMaxUnpackedBytes)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("runner: reading archive for %s: %w", sKey, readErr)
	}
	return data, nil
}

// readRemoteFileCandidates tries the direct path and the scripts/ and
// references/ fallbacks against the remote base URL.
func readRemoteFileCandidates(ctx context.Context, client *http.Client, baseURL, cleanPath, sKey string) ([]byte, bool) {
	for _, rp := range remotePathCandidates(cleanPath) {
		targetURL := baseURL + "/" + strings.TrimPrefix(rp, "/")
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", version.UserAgent())

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			continue
		}
		data, readErr := catalog.ReadBounded(resp.Body, 10<<20)
		_ = resp.Body.Close()
		if readErr == nil && len(data) > 0 {
			cacheSkillFile(sKey, rp, data)
			return data, true
		}
	}
	return nil, false
}

func remotePathCandidates(cleanPath string) []string {
	remotePaths := []string{cleanPath}
	if !strings.HasPrefix(cleanPath, "scripts/") && isScriptPath(cleanPath) {
		remotePaths = append([]string{"scripts/" + cleanPath}, remotePaths...)
	}
	if !strings.HasPrefix(cleanPath, "references/") && strings.HasSuffix(cleanPath, ".md") && cleanPath != "SKILL.md" {
		remotePaths = append([]string{"references/" + cleanPath}, remotePaths...)
	}
	return remotePaths
}

func isScriptPath(path string) bool {
	switch {
	case strings.HasSuffix(path, ".py"),
		strings.HasSuffix(path, ".sh"),
		strings.HasSuffix(path, ".js"),
		strings.HasSuffix(path, ".ts"),
		strings.HasSuffix(path, ".bash"):
		return true
	default:
		return false
	}
}

// ReadFileContext reads any skill file verbatim without magic path resolution.
func ReadFileContext(ctx context.Context, item catalog.Item, filePath string, client *http.Client) (string, error) {
	data, err := ReadBytesContext(ctx, item, filePath, client)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) != -1 {
		return "", fmt.Errorf("runner: %w: %s", ErrBinaryFile, filePath)
	}
	return string(data), nil
}

// ReadFile reads any skill file verbatim without magic path resolution.
func ReadFile(item catalog.Item, filePath string) (string, error) {
	return ReadFileContext(context.Background(), item, filePath, nil)
}

var (
	markdownLinkRegex  = regexp.MustCompile(`\[[^\]]*\]\(([^)#?]+)(?:[#?][^)]*)?\)`)
	markdownImgRegex   = regexp.MustCompile(`!\[[^\]]*\]\(([^)#?]+)(?:[#?][^)]*)?\)`)
	backtickPathRegex  = regexp.MustCompile("`((?:scripts|references|assets|evals)/[^`\\s]+)`")
	prosePathRegex     = regexp.MustCompile(`(?:^|[\s"'` + "`" + `\(\)\[\]])((?:scripts|references|assets|evals)/[a-zA-Z0-9_\-\./]+\.[a-zA-Z0-9_]+)`)
	dojoRunRegex       = regexp.MustCompile(`dojo\s+run\s+[a-zA-Z0-9_\-]+\s+([a-zA-Z0-9_\-\./]+)`)
	runnerCommandRegex = regexp.MustCompile(`(?:uv\s+run|python3?|bash|sh|node|bun(?:\s+run)?|deno(?:\s+run)?)\s+(?:\./)?scripts/([a-zA-Z0-9_\-\./]+\.[a-zA-Z0-9_]+)`)
)

// ExtractCandidates scans markdown text and extracts all referenced script, reference, asset, and evaluation paths.
func ExtractCandidates(content string) []string {
	var candidates []string
	seen := make(map[string]bool)

	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || p == "." || p == ".." || strings.HasPrefix(p, "#") || strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "mailto:") {
			return
		}
		if idx := strings.IndexAny(p, "?#"); idx != -1 {
			p = p[:idx]
		}
		if strings.ContainsAny(p, ":<>{}*") || strings.HasPrefix(p, "\\") || filepath.IsAbs(p) {
			return
		}
		clean := filepath.ToSlash(filepath.Clean(p))
		clean = strings.TrimPrefix(clean, "./")
		if clean == "" || clean == "." || clean == ".." || clean == "SKILL.md" || clean == "skill.md" || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") || strings.HasPrefix(clean, "/") {
			return
		}
		if !seen[clean] {
			seen[clean] = true
			candidates = append(candidates, clean)
		}
	}

	for _, m := range markdownLinkRegex.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range markdownImgRegex.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range backtickPathRegex.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range prosePathRegex.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range dojoRunRegex.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			scr := strings.TrimPrefix(m[1], "./")
			scr = strings.TrimPrefix(scr, "scripts/")
			if scr != "" {
				add("scripts/" + scr)
			}
		}
	}
	for _, m := range runnerCommandRegex.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			scr := strings.TrimPrefix(m[1], "./")
			scr = strings.TrimPrefix(scr, "scripts/")
			if scr != "" {
				add("scripts/" + scr)
			}
		}
	}

	sort.Strings(candidates)
	return candidates
}

// installTracker records the files an install produced, de-duplicated and
// normalized to slash-separated relative paths.
type installTracker struct {
	seen  map[string]bool
	files []string
}

func newInstallTracker() *installTracker {
	return &installTracker{seen: make(map[string]bool)}
}

func (t *installTracker) record(rel string) {
	cleanRel := filepath.ToSlash(filepath.Clean(rel))
	cleanRel = strings.TrimPrefix(cleanRel, "./")
	if t.seen[cleanRel] {
		return
	}
	t.seen[cleanRel] = true
	t.files = append(t.files, cleanRel)
}

func (t *installTracker) sorted() []string {
	sort.Strings(t.files)
	return t.files
}

// InstallSkillContext installs all skill files (SKILL.md, references, scripts, auxiliary docs) into destDir.
func InstallSkillContext(ctx context.Context, item catalog.Item, destDir string, rawContent string, extraCandidates []string, client *http.Client) ([]string, error) {
	if destDir == "" {
		return nil, fmt.Errorf("runner: destination directory is empty")
	}

	cleanDest := filepath.Clean(destDir)
	if err := os.MkdirAll(cleanDest, 0755); err != nil {
		return nil, fmt.Errorf("runner: creating destination directory %s: %w", cleanDest, err)
	}

	tracker := newInstallTracker()
	remoteURL := itemRemoteURL(item)

	// 0. Archive items unpack straight into the destination.
	if catalog.IsArchiveItem(item) && remoteURL != "" {
		if handled, files, err := installFromArchive(ctx, item, remoteURL, cleanDest, tracker, client); handled {
			return files, err
		}
	}

	// 1. Ensure SKILL.md is written.
	content, err := ensureInstalledSkillMD(ctx, item, cleanDest, rawContent, remoteURL, client, tracker)
	if err != nil {
		return nil, err
	}

	// 2. Install referenced files breadth-first, following links in Markdown.
	queue := collectInstallCandidates(item, content, extraCandidates)
	if err := installCandidateFiles(ctx, item, cleanDest, queue, remoteURL, client, tracker); err != nil {
		return nil, err
	}

	pruneStaleInstalledFiles(cleanDest, tracker.seen)
	return tracker.sorted(), nil
}

// installFromArchive materializes an archive item from the digest-verified
// cache when possible, otherwise downloads and unpacks it.
func installFromArchive(ctx context.Context, item catalog.Item, remoteURL, cleanDest string, tracker *installTracker, client *http.Client) (bool, []string, error) {
	expectedDigest := item.EffectiveDigest()
	cacheRoot := resolveCacheRoot()
	sKey := skillCacheKey(item)

	if expectedDigest != "" && cacheRoot != "" && sKey != "" && catalog.IsArchiveCachedWithDigest(cacheRoot, sKey, expectedDigest) {
		cacheSkillDir := filepath.Join(cacheRoot, "skills", sKey)
		if extracted, err := copyCachedArchiveDir(cacheSkillDir, cleanDest); err == nil && len(extracted) > 0 {
			for _, rel := range extracted {
				tracker.record(rel)
			}
			pruneStaleInstalledFiles(cleanDest, tracker.seen)
			return true, tracker.sorted(), nil
		}
	}

	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	data, err := downloadArchive(ctx, client, remoteURL, item.Name)
	if err != nil {
		return true, nil, err
	}
	if expectedDigest != "" {
		if vErr := catalog.VerifyDigest(data, expectedDigest); vErr != nil {
			return true, nil, fmt.Errorf("runner: %w: %v", catalog.ErrDigestMismatch, vErr)
		}
	}

	extracted, err := catalog.UnpackArchive(data, cleanDest, catalog.DefaultMaxUnpackedBytes)
	if err != nil {
		return true, nil, fmt.Errorf("runner: unpacking archive for %s: %w", item.Name, err)
	}
	for _, rel := range extracted {
		tracker.record(rel)
	}
	pruneStaleInstalledFiles(cleanDest, tracker.seen)
	return true, tracker.sorted(), nil
}

// ensureInstalledSkillMD writes the supplied body, or fetches it when absent.
func ensureInstalledSkillMD(ctx context.Context, item catalog.Item, cleanDest, rawContent, remoteURL string, client *http.Client, tracker *installTracker) (string, error) {
	skillFilePath := filepath.Join(cleanDest, "SKILL.md")

	if rawContent != "" {
		if err := os.WriteFile(skillFilePath, []byte(rawContent), 0644); err != nil {
			return "", fmt.Errorf("runner: writing %s: %w", skillFilePath, err)
		}
		tracker.record("SKILL.md")
		return rawContent, nil
	}
	if tracker.seen["SKILL.md"] {
		return rawContent, nil
	}

	data, err := readBytesInternal(ctx, item, "SKILL.md", client, cleanDest, remoteURL != "")
	if err == nil {
		if writeErr := os.WriteFile(skillFilePath, data, 0644); writeErr != nil {
			return "", fmt.Errorf("runner: writing %s: %w", skillFilePath, writeErr)
		}
		tracker.record("SKILL.md")
		return string(data), nil
	}
	if existing, readErr := os.ReadFile(skillFilePath); readErr == nil {
		tracker.record("SKILL.md")
		return string(existing), nil
	}
	return "", fmt.Errorf("runner: fetching SKILL.md for %s: %w", item.Name, err)
}

// collectInstallCandidates merges Markdown references and caller-supplied paths
// into a de-duplicated queue.
func collectInstallCandidates(item catalog.Item, rawContent string, extraCandidates []string) []string {
	var queue []string
	seen := make(map[string]bool)

	add := func(p string) {
		clean, ok := normalizeCandidatePath(p)
		if !ok || seen[clean] {
			return
		}
		seen[clean] = true
		queue = append(queue, clean)
	}

	if rawContent != "" {
		for _, c := range ExtractCandidates(rawContent) {
			add(c)
		}
	}
	if item.Body != "" && item.Body != rawContent {
		for _, c := range ExtractCandidates(item.Body) {
			add(c)
		}
	}
	for _, cand := range extraCandidates {
		add(cand)
	}
	return queue
}

// normalizeCandidatePath rejects URLs, anchors, absolute paths, and traversal.
func normalizeCandidatePath(p string) (string, bool) {
	p = strings.TrimSpace(p)
	if p == "" || p == "." || p == ".." || strings.HasPrefix(p, "#") ||
		strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "mailto:") {
		return "", false
	}
	if idx := strings.IndexAny(p, "?#"); idx != -1 {
		p = p[:idx]
	}
	if strings.Contains(p, ":") || strings.HasPrefix(p, "\\") || filepath.IsAbs(p) {
		return "", false
	}

	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(p)), "./")
	if clean == "" || clean == "." || clean == ".." || clean == "SKILL.md" || clean == "skill.md" ||
		strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") || strings.HasPrefix(clean, "/") {
		return "", false
	}
	return clean, true
}

// installCandidateFiles writes each queued file and follows Markdown links.
func installCandidateFiles(ctx context.Context, item catalog.Item, cleanDest string, queue []string, remoteURL string, client *http.Client, tracker *installTracker) error {
	preferRemote := remoteURL != "" && item.Source != "local"
	queued := make(map[string]bool, len(queue))
	for _, q := range queue {
		queued[q] = true
	}

	for i := 0; i < len(queue); i++ {
		target := queue[i]
		if tracker.seen[target] {
			continue
		}

		data, err := readBytesInternal(ctx, item, target, client, cleanDest, preferRemote)
		if err != nil {
			continue
		}

		destFilePath := filepath.Join(cleanDest, filepath.FromSlash(target))
		if !isContained(cleanDest, destFilePath) {
			return fmt.Errorf("runner: candidate %q escapes destination directory %q: %w", target, cleanDest, ErrInvalidPath)
		}
		if err := os.MkdirAll(filepath.Dir(destFilePath), 0755); err != nil {
			return fmt.Errorf("runner: creating directory for %s: %w", destFilePath, err)
		}

		perm := installedFilePerm(target)
		if err := os.WriteFile(destFilePath, data, perm); err != nil {
			return fmt.Errorf("runner: writing %s: %w", destFilePath, err)
		}
		if err := os.Chmod(destFilePath, perm); err != nil {
			return fmt.Errorf("runner: setting permissions on %s: %w", destFilePath, err)
		}
		tracker.record(target)

		if strings.HasSuffix(strings.ToLower(target), ".md") && utf8.Valid(data) {
			for _, next := range ExtractCandidates(string(data)) {
				clean, ok := normalizeCandidatePath(next)
				if !ok || queued[clean] || tracker.seen[clean] {
					continue
				}
				queued[clean] = true
				queue = append(queue, clean)
			}
		}
	}
	return nil
}

// installedFilePerm marks scripts executable and everything else readable.
func installedFilePerm(target string) os.FileMode {
	if strings.HasPrefix(target, "scripts/") ||
		strings.HasSuffix(target, ".sh") ||
		strings.HasSuffix(target, ".py") ||
		strings.HasSuffix(target, ".js") ||
		strings.HasSuffix(target, ".ts") {
		return 0755
	}
	return 0644
}

func pruneStaleInstalledFiles(cleanDest string, installedMap map[string]bool) {
	if len(installedMap) == 0 {
		return
	}
	var dirs []string
	_ = filepath.WalkDir(cleanDest, func(p string, d os.DirEntry, err error) error {
		if err != nil || p == cleanDest {
			return nil
		}
		rel, rErr := filepath.Rel(cleanDest, p)
		if rErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(filepath.Clean(rel))
		if d.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		if !installedMap[relSlash] {
			_ = os.Remove(p)
		}
		return nil
	})
	for i := len(dirs) - 1; i >= 0; i-- {
		if entries, err := os.ReadDir(dirs[i]); err == nil && len(entries) == 0 {
			_ = os.Remove(dirs[i])
		}
	}
}

func copyCachedArchiveDir(cacheSkillDir, cleanDest string) ([]string, error) {
	var extracted []string
	err := filepath.WalkDir(cacheSkillDir, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == cacheSkillDir {
			return nil
		}
		rel, err := filepath.Rel(cacheSkillDir, p)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(filepath.Clean(rel))
		if relSlash == ".digest" {
			return nil
		}
		destPath := filepath.Join(cleanDest, rel)
		if !isContained(cleanDest, destPath) {
			return ErrInvalidPath
		}
		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(p)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}
			_ = os.Remove(destPath)
			if err := os.Symlink(linkTarget, destPath); err != nil {
				return err
			}
			extracted = append(extracted, relSlash)
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		perm := info.Mode().Perm()
		if perm == 0 {
			perm = 0644
		}
		if err := os.WriteFile(destPath, data, perm); err != nil {
			return err
		}
		_ = os.Chmod(destPath, perm)
		extracted = append(extracted, relSlash)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return extracted, nil
}

// InstallSkill installs all skill files (SKILL.md, references, scripts, auxiliary docs) into destDir.
func InstallSkill(item catalog.Item, destDir string, rawContent string, extraCandidates []string) ([]string, error) {
	return InstallSkillContext(context.Background(), item, destDir, rawContent, extraCandidates, nil)
}
