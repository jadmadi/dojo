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

package catalog

import (
	"errors"
	"strings"
	"testing"
)

func TestSlugifyHeading(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Quick Start", "quick-start"},
		{"API Reference (v2)", "api-reference-v2"},
		{"`dojo load` #section", "dojo-load-section"},
		{"Multi-Harness Sync / Sila", "multi-harness-sync-sila"},
		{"   ", "section"},
	}
	for _, tt := range tests {
		if got := SlugifyHeading(tt.in); got != tt.want {
			t.Errorf("SlugifyHeading(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseSections_IgnoresCodeFencesAndExtractsNested(t *testing.T) {
	md := `---
name: sample-skill
description: Sample skill
---
# Sample Skill

Intro text.

## Installation

Run this in bash:
` + "```bash\n# This is a bash comment, NOT a heading!\ndojo learn sample-skill\n```" + `

### Sub-step

Nested sub-step details.

## Troubleshooting

Fix issues here.
`

	sections := ParseSections(md)
	if len(sections) != 4 {
		t.Fatalf("expected 4 sections, got %d: %+v", len(sections), sections)
	}

	if sections[1].Slug != "installation" || sections[1].Level != 2 {
		t.Errorf("unexpected second section: %+v", sections[1])
	}
	// Section "installation" (level 2) should include its nested ### Sub-step (level 3)
	if !strings.Contains(sections[1].Content, "### Sub-step") {
		t.Errorf("expected level-2 section to include nested level-3 subsection, got:\n%s", sections[1].Content)
	}
	if strings.Contains(sections[1].Content, "## Troubleshooting") {
		t.Errorf("expected level-2 section to stop before next level-2 section, got:\n%s", sections[1].Content)
	}

	sec, err := ExtractSection(md, "installation")
	if err != nil {
		t.Fatalf("ExtractSection(installation) unexpected error: %v", err)
	}
	if !strings.Contains(sec.Content, "dojo learn sample-skill") {
		t.Errorf("expected installation code block in extracted section, got: %s", sec.Content)
	}

	if _, err := ExtractSection(md, "nonexistent-section"); err == nil {
		t.Errorf("expected error for nonexistent section")
	}
}

func TestResolveDependencies_TopologicalAndCycle(t *testing.T) {
	cat, err := NewCatalog(
		WithManifest(&Manifest{
			Version: "1.0.0",
			Items: []Item{
				{
					Name:         "app-skill",
					Type:         "skill",
					Description:  "Top level skill",
					Dependencies: []string{"auth-skill", "db-skill"},
				},
				{
					Name:         "auth-skill",
					Type:         "skill",
					Description:  "Auth skill",
					Dependencies: []string{"core-skill"},
				},
				{
					Name:         "db-skill",
					Type:         "skill",
					Description:  "Database skill",
					Dependencies: []string{"core-skill"},
				},
				{
					Name:        "core-skill",
					Type:        "skill",
					Description: "Base core skill",
				},
			},
		}),
		WithRemoteDisabled(),
		WithLocalDiscovery(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveDependencies("app-skill", cat)
	if err != nil {
		t.Fatalf("ResolveDependencies unexpected error: %v", err)
	}
	if len(resolved) != 4 {
		t.Fatalf("expected 4 resolved items, got %d", len(resolved))
	}
	if resolved[0].Name != "core-skill" {
		t.Errorf("expected core-skill first in topological order, got %q", resolved[0].Name)
	}
	if resolved[len(resolved)-1].Name != "app-skill" {
		t.Errorf("expected app-skill last in topological order, got %q", resolved[len(resolved)-1].Name)
	}

	// Test cycle detection
	cyclicCat, err := NewCatalog(
		WithManifest(&Manifest{
			Version: "1.0.0",
			Items: []Item{
				{Name: "skill-a", Dependencies: []string{"skill-b"}},
				{Name: "skill-b", Dependencies: []string{"skill-a"}},
			},
		}),
		WithRemoteDisabled(),
		WithLocalDiscovery(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ResolveDependencies("skill-a", cyclicCat)
	if !errors.Is(err, ErrCircularDependency) {
		t.Fatalf("expected ErrCircularDependency, got %v", err)
	}
}
