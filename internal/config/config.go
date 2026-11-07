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

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultCatalogURL is the default remote catalog URL.
const DefaultCatalogURL = "https://skills.danicat.dev/.well-known/agent-skills/index.json"

// DefaultDojoGitHubCatalog is the primary Dojo GitHub catalog repository.
const DefaultDojoGitHubCatalog = "jadmadi/dojo"

// DefaultGitHubCatalog is the legacy Kungfu GitHub catalog repository.
const DefaultGitHubCatalog = "danicat/kungfu"

// DefaultGeminiSkillsCatalog is the official Google Gemini Skills catalog repository.
const DefaultGeminiSkillsCatalog = "google-gemini/gemini-skills"

// LoadConfig holds configuration options for dojo load behavior.
type LoadConfig struct {
	AutoUpdate *bool `json:"auto_update,omitempty" yaml:"auto_update,omitempty"`
}

// TelemetryConfig holds configuration options for opt-in community usage telemetry.
type TelemetryConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// Config represents the user-level configuration (~/.config/dojo/config.yaml or config.json).
type Config struct {
	Load      LoadConfig        `json:"load" yaml:"load,omitempty"`
	Telemetry TelemetryConfig   `json:"telemetry" yaml:"telemetry,omitempty"`
	Catalogs  map[string]string `json:"catalogs,omitempty" yaml:"catalogs,omitempty"`
}

// DefaultConfig returns the default configuration (telemetry strictly opt-in / false by default).
func DefaultConfig() *Config {
	autoUpdate := true
	telemetryEnabled := false
	return &Config{
		Load: LoadConfig{
			AutoUpdate: &autoUpdate,
		},
		Telemetry: TelemetryConfig{
			Enabled: &telemetryEnabled,
		},
		Catalogs: map[string]string{
			"default":       DefaultCatalogURL,
			"dojo":          DefaultDojoGitHubCatalog,
			"kungfu":        DefaultGitHubCatalog,
			"gemini-skills": DefaultGeminiSkillsCatalog,
		},
	}
}

// IsAutoUpdate returns true if auto-update is enabled (default true).
func (c *Config) IsAutoUpdate() bool {
	if c == nil || c.Load.AutoUpdate == nil {
		return true
	}
	return *c.Load.AutoUpdate
}

// IsTelemetryEnabled returns true only if community telemetry is explicitly enabled via config or DOJO_TELEMETRY / KUNGFU_TELEMETRY=1/true,
// and DO_NOT_TRACK is not set to 1/true.
func (c *Config) IsTelemetryEnabled() bool {
	if dnt := strings.ToLower(strings.TrimSpace(os.Getenv("DO_NOT_TRACK"))); dnt == "1" || dnt == "true" {
		return false
	}
	if env := strings.ToLower(strings.TrimSpace(os.Getenv("DOJO_TELEMETRY"))); env != "" {
		return env == "1" || env == "true" || env == "on" || env == "yes"
	}
	if env := strings.ToLower(strings.TrimSpace(os.Getenv("KUNGFU_TELEMETRY"))); env != "" {
		return env == "1" || env == "true" || env == "on" || env == "yes"
	}
	if c == nil || c.Telemetry.Enabled == nil {
		return false
	}
	return *c.Telemetry.Enabled
}

// SetTelemetryEnabled updates the telemetry enabled flag on the Config struct.
func (c *Config) SetTelemetryEnabled(enabled bool) {
	if c == nil {
		return
	}
	v := enabled
	c.Telemetry.Enabled = &v
}

// GetCatalogURL returns the URL for a catalog name, or empty string if not found.
func (c *Config) GetCatalogURL(name string) string {
	if c != nil && c.Catalogs != nil {
		if u, ok := c.Catalogs[name]; ok && u != "" {
			return u
		}
	}
	switch name {
	case "default":
		return DefaultCatalogURL
	case "dojo":
		return DefaultDojoGitHubCatalog
	case "kungfu":
		return DefaultGitHubCatalog
	case "gemini-skills":
		return DefaultGeminiSkillsCatalog
	default:
		return ""
	}
}

// UserConfigDir returns ~/.config/dojo (or ~/.config/kungfu fallback), or respects DOJO_CONFIG_DIR / KUNGFU_CONFIG_DIR.
func UserConfigDir() string {
	if dir := os.Getenv("DOJO_CONFIG_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("KUNGFU_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		panic(fmt.Sprintf("config: unable to resolve user home directory: %v", err))
	}
	dojoDir := filepath.Join(home, ".config", "dojo")
	kungfuDir := filepath.Join(home, ".config", "kungfu")
	if _, err := os.Stat(dojoDir); err != nil {
		if fi, kErr := os.Stat(kungfuDir); kErr == nil && fi.IsDir() {
			return kungfuDir
		}
	}
	return dojoDir
}

// ResolveConfigPath locates the active configuration file.
// Precedence:
// 1. Explicit path parameter (e.g. from --config flag)
// 2. DOJO_CONFIG or KUNGFU_CONFIG env var
// 3. ~/.config/dojo/config.yaml
// 4. ~/.config/dojo/config.yml
// 5. ~/.config/dojo/config.json
func ResolveConfigPath(customPath string) string {
	if customPath != "" {
		return customPath
	}
	if envPath := os.Getenv("DOJO_CONFIG"); envPath != "" {
		return envPath
	}
	if envPath := os.Getenv("KUNGFU_CONFIG"); envPath != "" {
		return envPath
	}

	dir := UserConfigDir()
	candidates := []string{
		filepath.Join(dir, "config.yaml"),
		filepath.Join(dir, "config.yml"),
		filepath.Join(dir, "config.json"),
	}

	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}

	return filepath.Join(dir, "config.yaml")
}

// Load reads and parses user configuration from the resolved path.
// If no explicit customPath or DOJO_CONFIG/KUNGFU_CONFIG is set and the default file does not exist, DefaultConfig is returned.
func Load(customPath string) (*Config, error) {
	explicit := customPath != "" || os.Getenv("DOJO_CONFIG") != "" || os.Getenv("KUNGFU_CONFIG") != ""
	path := ResolveConfigPath(customPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !explicit {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("config: reading %s: %w", path, err)
	}

	cfg := DefaultConfig()

	if strings.HasSuffix(path, ".json") {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config: parsing JSON %s: %w", path, err)
		}
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config: parsing YAML %s: %w", path, err)
		}
	}

	if cfg.Catalogs == nil {
		cfg.Catalogs = make(map[string]string)
	}
	if _, ok := cfg.Catalogs["default"]; !ok || cfg.Catalogs["default"] == "" {
		cfg.Catalogs["default"] = DefaultCatalogURL
	}
	if _, ok := cfg.Catalogs["dojo"]; !ok || cfg.Catalogs["dojo"] == "" {
		cfg.Catalogs["dojo"] = DefaultDojoGitHubCatalog
	}
	if _, ok := cfg.Catalogs["kungfu"]; !ok || cfg.Catalogs["kungfu"] == "" {
		cfg.Catalogs["kungfu"] = DefaultGitHubCatalog
	}
	if _, ok := cfg.Catalogs["gemini-skills"]; !ok || cfg.Catalogs["gemini-skills"] == "" {
		cfg.Catalogs["gemini-skills"] = DefaultGeminiSkillsCatalog
	}

	return cfg, nil
}

// Save writes the given configuration atomically to the resolved config path and returns the path written.
func Save(customPath string, cfg *Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config: cannot save nil config")
	}
	path := ResolveConfigPath(customPath)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("config: creating directory %s: %w", dir, err)
	}

	var data []byte
	var err error
	if strings.HasSuffix(path, ".json") {
		data, err = json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return "", fmt.Errorf("config: encoding JSON: %w", err)
		}
		data = append(data, '\n')
	} else {
		data, err = yaml.Marshal(cfg)
		if err != nil {
			return "", fmt.Errorf("config: encoding YAML: %w", err)
		}
	}

	tmpFile, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return "", fmt.Errorf("config: creating temp file in %s: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("config: writing temp file %s: %w", tmpPath, err)
	}
	if err := tmpFile.Chmod(0644); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("config: setting permissions on %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("config: closing temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("config: renaming %s -> %s: %w", tmpPath, path, err)
	}
	return path, nil
}
