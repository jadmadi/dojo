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

	if !checkFlag && (hasUpdate || forceFlag) {
		if upgradedErr := a.performSelfUpgrade(ctx, rel, latestVer); upgradedErr != nil {
			_, _ = fmt.Fprintf(a.Stderr, "error: self-upgrade failed: %v\n", upgradedErr)
			return ExitGeneral
		}
		status = "upgraded"
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
			!strings.HasSuffix(lower, ".tar.gz") && !strings.HasSuffix(lower, ".zip") && !strings.HasSuffix(lower, ".txt") {
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
			binData, err := catalog.ReadBounded(resp.Body, 100<<20)
			_ = resp.Body.Close()
			if err != nil || len(binData) == 0 {
				continue
			}
			exePath, err := os.Executable()
			if err != nil {
				return err
			}
			tmpPath := filepath.Join(filepath.Dir(exePath), ".dojo-upgrade.tmp")
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
		cmd := exec.CommandContext(ctx, goBin, "install", "github.com/jadmadi/dojo/cmd/dojo@"+ref)
		cmd.Env = append(os.Environ(), "GOPROXY=direct")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go install failed: %v (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	return fmt.Errorf("no prebuilt binary asset found for %s/%s and 'go' toolchain not in PATH", runtime.GOOS, runtime.GOARCH)
}
