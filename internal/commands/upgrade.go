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
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/version"
)

// DefaultReleasesURL is the GitHub Releases API endpoint for Dojo self-upgrades.
var DefaultReleasesURL = "https://api.github.com/repos/jadmadi/dojo/releases/latest"

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// RunUpgrade checks for a newer Dojo release on GitHub and optionally self-upgrades the binary.
func (a *App) RunUpgrade(args []string) int {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var checkFlag, forceFlag, jsonFlag, rawFlag bool

	fs.BoolVar(&checkFlag, "check", false, "Only check if a newer version of dojo is available")
	fs.BoolVar(&checkFlag, "c", false, "Only check if a newer version is available (shorthand)")
	fs.BoolVar(&forceFlag, "force", false, "Force upgrade even if current version is up to date")
	fs.BoolVar(&forceFlag, "f", false, "Force upgrade (shorthand)")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	if _, err := parseFlagsWithArgs(fs, args); err != nil {
		return ExitUsage
	}

	mode := a.resolveMode(jsonFlag, rawFlag)
	currentVer := version.Get()

	releasesURL := DefaultReleasesURL
	if envURL := os.Getenv("DOJO_RELEASES_URL"); envURL != "" {
		releasesURL = envURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: %v\n", err)
		return ExitGeneral
	}
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Accept", "application/vnd.github+json")

	var latestVer string
	var rel githubRelease
	resp, err := a.getHTTPClient().Do(req)
	if err == nil && resp != nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == http.StatusOK {
			data, rErr := catalog.ReadBounded(resp.Body, 2<<20)
			if rErr == nil {
				_ = json.Unmarshal(data, &rel)
				latestVer = strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
			}
		}
	}

	if latestVer == "" {
		latestVer = currentVer
	}

	hasUpdate := catalog.CompareSemver(latestVer, currentVer) > 0
	status := "up_to_date"
	if hasUpdate {
		status = "update_available"
	}

	var manualSyncMsg string
	if !checkFlag && (hasUpdate || forceFlag) {
		if upgradedErr := a.performSelfUpgrade(ctx, rel, latestVer); upgradedErr != nil {
			if strings.HasPrefix(upgradedErr.Error(), "manual_sync:") {
				manualSyncMsg = strings.TrimPrefix(upgradedErr.Error(), "manual_sync:")
				status = "sync_required"
			} else {
				_, _ = fmt.Fprintf(a.Stderr, "error: self-upgrade failed: %v\n", upgradedErr)
				return ExitGeneral
			}
		} else {
			status = "upgraded"
		}
	}

	switch mode {
	case OutputModeJSON:
		res := struct {
			CurrentVersion string `json:"current_version"`
			LatestVersion  string `json:"latest_version"`
			HasUpdate      bool   `json:"has_update"`
			Status         string `json:"status"`
		}{
			CurrentVersion: currentVer,
			LatestVersion:  latestVer,
			HasUpdate:      hasUpdate,
			Status:         status,
		}
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	case OutputModeRaw:
		_, _ = fmt.Fprintf(a.Stdout, "%s\t%s\t%s\n",
			sanitizeTSVField(status),
			sanitizeTSVField(currentVer),
			sanitizeTSVField(latestVer),
		)
	default:
		switch status {
		case "upgraded":
			_, _ = fmt.Fprintf(a.Stdout, "✓ Upgraded dojo %s -> %s\n", currentVer, latestVer)
		case "sync_required":
			_, _ = fmt.Fprintln(a.Stdout, manualSyncMsg)
		case "update_available":
			_, _ = fmt.Fprintf(a.Stdout, "↑ Update available: dojo %s -> %s (run 'dojo upgrade' to install)\n", currentVer, latestVer)
		default:
			_, _ = fmt.Fprintf(a.Stdout, "✓ dojo %s is up to date.\n", currentVer)
		}
	}
	return ExitSuccess
}

func (a *App) performSelfUpgrade(ctx context.Context, rel githubRelease, targetVer string) error {
	osArch := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	osArchDash := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, asset := range rel.Assets {
		lower := strings.ToLower(asset.Name)
		if (strings.Contains(lower, osArch) || strings.Contains(lower, osArchDash)) &&
			!strings.HasSuffix(lower, ".txt") && !strings.HasSuffix(lower, ".sha256") && !strings.HasSuffix(lower, ".sig") {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
			if err != nil {
				continue
			}
			req.Header.Set("User-Agent", version.UserAgent())
			resp, err := a.getHTTPClient().Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				if resp != nil {
					_ = resp.Body.Close()
				}
				continue
			}
			rawData, err := catalog.ReadBounded(resp.Body, 100<<20)
			_ = resp.Body.Close()
			if err != nil || len(rawData) == 0 {
				continue
			}
			binData := rawData
			if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
				extracted, exErr := extractBinaryFromTarGz(rawData)
				if exErr != nil || len(extracted) == 0 {
					continue
				}
				binData = extracted
			} else if strings.HasSuffix(lower, ".zip") {
				extracted, exErr := extractBinaryFromZip(rawData)
				if exErr != nil || len(extracted) == 0 {
					continue
				}
				binData = extracted
			}
			exePath, err := os.Executable()
			if err != nil {
				return err
			}
			cleanExe, symErr := filepath.EvalSymlinks(exePath)
			if symErr == nil && cleanExe != "" {
				exePath = cleanExe
			}
			tmpPath := filepath.Join(filepath.Dir(exePath), fmt.Sprintf(".dojo-upgrade.%d.tmp", os.Getpid()))
			if err := os.WriteFile(tmpPath, binData, 0755); err != nil {
				return err
			}
			if err := os.Rename(tmpPath, exePath); err != nil {
				_ = os.Remove(tmpPath)
				return err
			}
			return nil
		}
	}

	if goBin, err := exec.LookPath("go"); err == nil && goBin != "" {
		ref := "latest"
		if targetVer != "" {
			ref = "v" + strings.TrimPrefix(targetVer, "v")
		}
		installStart := time.Now().Add(-2 * time.Second)
		cmd := exec.CommandContext(ctx, goBin, "install", "github.com/jadmadi/dojo/cmd/dojo@"+ref)
		cmd.Env = append(os.Environ(), "GOPROXY=direct")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go install failed: %v (%s)", err, strings.TrimSpace(string(out)))
		}

		installedBin := findInstalledGoBin(goBin, installStart)
		if installedBin == "" {
			return nil
		}

		if exePath, exeErr := os.Executable(); exeErr == nil && exePath != "" {
			cleanExe, _ := filepath.EvalSymlinks(exePath)
			cleanInstalled, _ := filepath.EvalSymlinks(installedBin)
			if cleanExe != "" && cleanInstalled != "" && cleanExe != cleanInstalled {
				copied := false
				if binData, rErr := os.ReadFile(installedBin); rErr == nil {
					tmpPath := filepath.Join(filepath.Dir(cleanExe), fmt.Sprintf(".dojo-upgrade.%d.tmp", os.Getpid()))
					if wErr := os.WriteFile(tmpPath, binData, 0755); wErr == nil {
						if renErr := os.Rename(tmpPath, cleanExe); renErr == nil {
							copied = true
						} else {
							_ = os.Remove(tmpPath)
						}
					}
				}
				if !copied {
					return fmt.Errorf("manual_sync:Notice: upgraded binary installed to %s.\nActive executable %s could not be automatically replaced (insufficient permissions).\nTo use the upgraded version, update your PATH or run:\n  sudo cp %s %s && hash -r", installedBin, exePath, installedBin, exePath)
				}
			}
		}
		return nil
	}

	return fmt.Errorf("no prebuilt binary asset found for %s/%s and 'go' toolchain not in PATH", runtime.GOOS, runtime.GOARCH)
}

func extractBinaryFromTarGz(archiveBytes []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(archiveBytes))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base == "dojo" || base == "dojo.exe" {
			return catalog.ReadBounded(tr, 100<<20)
		}
	}
	return nil, fmt.Errorf("dojo binary not found in tar.gz archive")
}

func extractBinaryFromZip(archiveBytes []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		if base == "dojo" || base == "dojo.exe" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			data, rErr := catalog.ReadBounded(rc, 100<<20)
			_ = rc.Close()
			if rErr != nil {
				return nil, rErr
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("dojo binary not found in zip archive")
}

func findInstalledGoBin(goBin string, installStart time.Time) string {
	binName := "dojo"
	if runtime.GOOS == "windows" {
		binName = "dojo.exe"
	}

	var candidateDirs []string
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		candidateDirs = append(candidateDirs, gobin)
	}
	if out, err := exec.Command(goBin, "env", "GOBIN").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			candidateDirs = append(candidateDirs, s)
		}
	}
	if goBinDir := filepath.Dir(goBin); goBinDir != "" {
		candidateDirs = append(candidateDirs, goBinDir)
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		for _, p := range filepath.SplitList(gopath) {
			candidateDirs = append(candidateDirs, filepath.Join(p, "bin"))
		}
	}
	if out, err := exec.Command(goBin, "env", "GOPATH").Output(); err == nil {
		for _, p := range filepath.SplitList(strings.TrimSpace(string(out))) {
			candidateDirs = append(candidateDirs, filepath.Join(p, "bin"))
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidateDirs = append(candidateDirs, filepath.Join(home, "go", "bin"))
	}
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		candidateDirs = append(candidateDirs, p)
	}

	seen := make(map[string]bool)
	var newestPath string
	var newestMod time.Time

	for _, dir := range candidateDirs {
		cand := filepath.Clean(filepath.Join(dir, binName))
		if seen[cand] {
			continue
		}
		seen[cand] = true

		fi, err := os.Stat(cand)
		if err != nil || fi.IsDir() {
			continue
		}

		if fi.ModTime().After(installStart) {
			return cand
		}
		if fi.ModTime().After(newestMod) {
			newestMod = fi.ModTime()
			newestPath = cand
		}
	}

	if newestPath != "" {
		return newestPath
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "go", "bin", binName)
	}
	return filepath.Join(filepath.Dir(goBin), binName)
}
