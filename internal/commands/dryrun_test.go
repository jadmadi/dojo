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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestSkill(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, ".agents", "skills", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating test skill: %v", err)
	}
	body := "---\nname: " + name + "\ndescription: test skill\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0644); err != nil {
		t.Fatalf("writing test skill: %v", err)
	}
	return dir
}

func TestLearnDryRunWritesNothing(t *testing.T) {
	server := setupEmptyRemoteCatalogServer(t)
	defer server.Close()

	workDir := t.TempDir()
	origWD, _ := os.Getwd()
	_ = os.Chdir(workDir)
	defer func() { _ = os.Chdir(origWD) }()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"learn", "dojo", "--dry-run", "--json"}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("Run(learn --dry-run) = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "planned"`) {
		t.Errorf("expected planned result, got:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(workDir, ".agents", "skills", "dojo", "SKILL.md")); err == nil {
		t.Errorf("learn --dry-run wrote a skill directory")
	}
}

func TestSyncDryRunPlansWithoutWriting(t *testing.T) {
	workDir := t.TempDir()
	writeTestSkill(t, workDir, "dry-skill")

	origWD, _ := os.Getwd()
	_ = os.Chdir(workDir)
	defer func() { _ = os.Chdir(origWD) }()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--dry-run", "--json", "-t", "all"}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("Run(sync --dry-run) = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "planned"`) {
		t.Errorf("expected planned projection, got:\n%s", stdout.String())
	}
	for _, harnessDir := range []string{".claude", ".cursor", ".windsurf", ".gemini", ".opencode"} {
		if _, err := os.Stat(filepath.Join(workDir, harnessDir)); err == nil {
			t.Errorf("sync --dry-run wrote %s", harnessDir)
		}
	}
}

func TestProfilePushDryRunDoesNotWrite(t *testing.T) {
	server := setupEmptyRemoteCatalogServer(t)
	defer server.Close()

	workDir := t.TempDir()
	targetDir := t.TempDir()
	writeTestSkill(t, workDir, "local-skill")

	origWD, _ := os.Getwd()
	_ = os.Chdir(workDir)
	defer func() { _ = os.Chdir(origWD) }()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"profile", "push", targetDir, "local-skill", "--dry-run", "--json"}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("Run(profile push --dry-run) = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "planned"`) {
		t.Errorf("expected planned push, got:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(targetDir, "skills")); err == nil {
		t.Errorf("profile push --dry-run wrote a skills directory")
	}
	if _, err := os.Stat(filepath.Join(targetDir, "skills.json")); err == nil {
		t.Errorf("profile push --dry-run wrote skills.json")
	}
}
