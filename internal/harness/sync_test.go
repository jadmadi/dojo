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

package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTargets(t *testing.T) {
	root := t.TempDir()
	targets, err := ParseTargets("all", root)
	if err != nil {
		t.Fatalf("ParseTargets(all) error: %v", err)
	}
	if len(targets) != len(AllTargets) {
		t.Errorf("expected %d targets for 'all', got %d", len(AllTargets), len(targets))
	}

	subset, err := ParseTargets("claude,cursor,sila", root)
	if err != nil {
		t.Fatalf("ParseTargets(claude,cursor,sila) error: %v", err)
	}
	if len(subset) != 3 {
		t.Fatalf("expected 3 targets, got %v", subset)
	}

	if _, err := ParseTargets("invalid-harness", root); err == nil {
		t.Errorf("expected error for invalid harness target")
	}
}

func TestSyncSkill_AllHarnesses(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, ".agents", "skills", "demo-skill")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: demo-skill
description: Demo multi-harness skill for testing
---

# Demo Skill

Always run tests with race detector.
`
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatal(err)
	}

	results := SyncSkill("demo-skill", srcDir, root, AllTargets)
	for _, r := range results {
		if r.Status != "synced" {
			t.Errorf("expected status synced for %s, got %+v", r.Harness, r)
		}
	}

	// Verify Cursor .mdc conversion
	mdcBytes, err := os.ReadFile(filepath.Join(root, ".cursor", "rules", "demo-skill.mdc"))
	if err != nil {
		t.Fatalf("expected .cursor/rules/demo-skill.mdc to exist: %v", err)
	}
	if !strings.Contains(string(mdcBytes), "alwaysApply: false") || !strings.Contains(string(mdcBytes), "Demo multi-harness skill") {
		t.Errorf("unexpected Cursor .mdc content:\n%s", string(mdcBytes))
	}

	// Verify Windsurf rule file
	wsBytes, err := os.ReadFile(filepath.Join(root, ".windsurf", "rules", "demo-skill.md"))
	if err != nil {
		t.Fatalf("expected .windsurf/rules/demo-skill.md to exist: %v", err)
	}
	if !strings.Contains(string(wsBytes), "# Demo Skill") {
		t.Errorf("unexpected Windsurf rule content:\n%s", string(wsBytes))
	}

	// Verify Claude, Gemini, OpenCode, and Sila projections
	for _, dir := range []string{
		filepath.Join(root, ".claude", "skills", "demo-skill", "SKILL.md"),
		filepath.Join(root, ".gemini", "skills", "demo-skill", "SKILL.md"),
		filepath.Join(root, ".opencode", "skills", "demo-skill", "SKILL.md"),
		filepath.Join(root, ".sila", "skills", "demo-skill", "SKILL.md"),
	} {
		data, err := os.ReadFile(dir)
		if err != nil {
			t.Errorf("expected projected SKILL.md at %s: %v", dir, err)
			continue
		}
		if !strings.Contains(string(data), "# Demo Skill") {
			t.Errorf("expected # Demo Skill in %s", dir)
		}
	}
}

func TestReconcileHarnesses_BiDirectional(t *testing.T) {
	root := t.TempDir()

	// 1. Create a brand new skill inside .gemini/skills/antigravity-authored/SKILL.md
	agySkillDir := filepath.Join(root, ".gemini", "skills", "antigravity-authored")
	if err := os.MkdirAll(agySkillDir, 0755); err != nil {
		t.Fatal(err)
	}
	agyContent := "---\nname: antigravity-authored\ndescription: Authored directly in Antigravity\n---\n\n# Antigravity Authored\n\nUse dojo sila.\n"
	if err := os.WriteFile(filepath.Join(agySkillDir, "SKILL.md"), []byte(agyContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Create a Cursor rule .cursor/rules/cursor-authored.mdc
	cursorRulesDir := filepath.Join(root, ".cursor", "rules")
	if err := os.MkdirAll(cursorRulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	mdcContent := "---\ndescription: \"Rule created in Cursor\"\nglobs:\nalwaysApply: false\n---\n\n# Cursor Rule\n\nFollow strict Go linting.\n"
	if err := os.WriteFile(filepath.Join(cursorRulesDir, "cursor-authored.mdc"), []byte(mdcContent), 0644); err != nil {
		t.Fatal(err)
	}

	reconciled := ReconcileHarnesses(root)
	if len(reconciled) != 2 {
		t.Fatalf("expected 2 reconciled skills, got %d: %+v", len(reconciled), reconciled)
	}

	// Verify both exist in canonical .agents/skills/
	canonAgy, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "antigravity-authored", "SKILL.md"))
	if err != nil {
		t.Fatalf("expected canonical antigravity-authored SKILL.md: %v", err)
	}
	if !strings.Contains(string(canonAgy), "# Antigravity Authored") {
		t.Errorf("unexpected canonical antigravity-authored content: %s", string(canonAgy))
	}

	canonCursor, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "cursor-authored", "SKILL.md"))
	if err != nil {
		t.Fatalf("expected canonical cursor-authored SKILL.md: %v", err)
	}
	if !strings.Contains(string(canonCursor), "name: cursor-authored") || !strings.Contains(string(canonCursor), "# Cursor Rule") {
		t.Errorf("unexpected reverse-transpiled SKILL.md from .mdc:\n%s", string(canonCursor))
	}
}
