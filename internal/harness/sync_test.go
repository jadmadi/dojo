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

	legacy, err := ParseTargets("windsurf", root)
	if err != nil || len(legacy) != 1 || legacy[0] != TargetDevin {
		t.Errorf("windsurf should alias devin, got %v (err %v)", legacy, err)
	}

	rules, err := ParseTargets("cursor-rules", root)
	if err != nil || len(rules) != 1 || rules[0] != TargetCursorRules {
		t.Errorf("cursor-rules should be an opt-in target, got %v (err %v)", rules, err)
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

	// Verify Cursor projects a skill, not a rule.
	for _, dir := range []string{
		filepath.Join(root, ".claude", "skills", "demo-skill", "SKILL.md"),
		filepath.Join(root, ".cursor", "skills", "demo-skill", "SKILL.md"),
		filepath.Join(root, ".devin", "skills", "demo-skill", "SKILL.md"),
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

	if _, err := os.Stat(filepath.Join(root, ".cursor", "rules", "demo-skill.mdc")); err == nil {
		t.Errorf("cursor-rules projection must be opt-in, found .cursor/rules/demo-skill.mdc")
	}
	if _, err := os.Stat(filepath.Join(root, ".windsurf")); err == nil {
		t.Errorf("legacy windsurf rules must not be written for the devin target")
	}

	// Antigravity reads workspace skills from the canonical store, so gemini
	// resolves to .agents and must not duplicate the skill under .gemini.
	if _, err := os.Stat(filepath.Join(root, ".gemini", "skills")); err == nil {
		t.Errorf("gemini workspace projection should use .agents/skills, found .gemini/skills")
	}
	var geminiResult *ProjectionResult
	for i := range results {
		if results[i].Harness == "gemini" {
			geminiResult = &results[i]
			break
		}
	}
	if geminiResult == nil || geminiResult.Method != "canonical" || filepath.Clean(geminiResult.Path) != filepath.Clean(srcDir) {
		t.Errorf("expected gemini to resolve to the canonical store, got %+v", geminiResult)
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

func TestSyncSkill_CursorRulesOptIn(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, ".agents", "skills", "rule-skill")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: rule-skill\ndescription: Rule opt in\n---\n\n# Rule Skill\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results := SyncSkill("rule-skill", srcDir, root, []Target{TargetCursorRules})
	if len(results) != 1 || results[0].Status != "synced" || results[0].Method != "mdc" {
		t.Fatalf("unexpected cursor-rules result: %+v", results)
	}
	mdcBytes, err := os.ReadFile(filepath.Join(root, ".cursor", "rules", "rule-skill.mdc"))
	if err != nil {
		t.Fatalf("expected .cursor/rules/rule-skill.mdc: %v", err)
	}
	if !strings.Contains(string(mdcBytes), "alwaysApply: false") {
		t.Errorf("unexpected mdc content:\n%s", string(mdcBytes))
	}
}

func TestSyncSkill_GeminiGlobalPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srcDir := filepath.Join(home, ".agents", "skills", "global-skill")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: global-skill\ndescription: Global skill\n---\n\n# Global Skill\n"
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results := SyncSkill("global-skill", srcDir, home, []Target{TargetGemini})
	if len(results) != 1 || results[0].Status != "synced" {
		t.Fatalf("unexpected gemini global result: %+v", results)
	}
	want := filepath.Join(home, ".gemini", "config", "skills", "global-skill", "SKILL.md")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected Antigravity global projection at %s: %v", want, err)
	}
}
