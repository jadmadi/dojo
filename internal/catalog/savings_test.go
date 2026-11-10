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
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// maxLargestLeafPercent is the published ceiling this corpus enforces. A
// change that makes the largest leaf section take more of a skill body fails.
const maxLargestLeafPercent = 45.0

var updateSavings = flag.Bool("update", false, "rewrite docs/benchmarks/context-savings.json")

type savingsRow struct {
	Skill              string  `json:"skill"`
	BodyBytes          int     `json:"body_bytes"`
	LeafSections       int     `json:"leaf_sections"`
	LargestLeafBytes   int     `json:"largest_leaf_bytes"`
	LargestLeafPercent float64 `json:"largest_leaf_percent"`
}

type savingsReport struct {
	Corpus                   string       `json:"corpus"`
	Fixtures                 int          `json:"fixtures"`
	CeilingPercent           float64      `json:"ceiling_percent"`
	MaxLargestLeafPercent    float64      `json:"max_largest_leaf_percent"`
	MedianLargestLeafPercent float64      `json:"median_largest_leaf_percent"`
	Note                     string       `json:"note"`
	Skills                   []savingsRow `json:"skills"`
}

func TestContextSavings(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "bench-skills", "*", "SKILL.md"))
	if err != nil {
		t.Fatalf("globbing fixtures: %v", err)
	}
	if len(paths) < 4 {
		t.Fatalf("expected at least 4 benchmark skills, found %d", len(paths))
	}
	sort.Strings(paths)

	report := savingsReport{
		Corpus:         "internal/catalog/testdata/bench-skills",
		Fixtures:       len(paths),
		CeilingPercent: maxLargestLeafPercent,
		Note:           "Synthetic fixtures, regenerated with: go test ./internal/catalog -run TestContextSavings -update",
	}

	var percents []float64
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("reading %s: %v", path, readErr)
		}
		row := measureSavings(filepath.Base(filepath.Dir(path)), data)
		if row.LeafSections < 5 {
			t.Errorf("%s: expected at least 5 leaf sections, found %d", row.Skill, row.LeafSections)
		}
		if row.LargestLeafPercent > maxLargestLeafPercent {
			t.Errorf("%s: largest leaf section is %.2f%% of the body, ceiling is %.2f%%",
				row.Skill, row.LargestLeafPercent, maxLargestLeafPercent)
		}
		percents = append(percents, row.LargestLeafPercent)
		report.Skills = append(report.Skills, row)
	}

	report.MaxLargestLeafPercent = round2(maxOf(percents))
	report.MedianLargestLeafPercent = round2(medianOf(percents))

	const publishedPath = "../../docs/benchmarks/context-savings.json"
	if *updateSavings {
		if err := os.MkdirAll(filepath.Dir(publishedPath), 0755); err != nil {
			t.Fatalf("creating benchmark dir: %v", err)
		}
		data, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr != nil {
			t.Fatalf("marshalling report: %v", marshalErr)
		}
		if err := os.WriteFile(publishedPath, append(data, '\n'), 0644); err != nil {
			t.Fatalf("writing report: %v", err)
		}
		return
	}

	publishedData, err := os.ReadFile(publishedPath)
	if err != nil {
		t.Fatalf("reading %s: %v (run with -update to create it)", publishedPath, err)
	}
	var published savingsReport
	if err := json.Unmarshal(publishedData, &published); err != nil {
		t.Fatalf("parsing %s: %v", publishedPath, err)
	}
	if published.Fixtures != report.Fixtures ||
		published.CeilingPercent != report.CeilingPercent ||
		published.MaxLargestLeafPercent != report.MaxLargestLeafPercent ||
		published.MedianLargestLeafPercent != report.MedianLargestLeafPercent {
		t.Fatalf("published benchmark drift: got %+v, published %+v; run with -update", report, published)
	}
}

// measureSavings computes the byte share of the largest leaf section.
func measureSavings(name string, data []byte) savingsRow {
	body := data
	if _, stripped, hasFM, err := ExtractFrontmatter(data); err == nil && hasFM {
		body = stripped
	}

	leaves := leafSections(ParseSections(string(data)))
	row := savingsRow{
		Skill:        name,
		BodyBytes:    len(body),
		LeafSections: len(leaves),
	}
	for _, s := range leaves {
		if s.Bytes > row.LargestLeafBytes {
			row.LargestLeafBytes = s.Bytes
		}
	}
	if row.BodyBytes > 0 {
		row.LargestLeafPercent = round2(float64(row.LargestLeafBytes) / float64(row.BodyBytes) * 100)
	}
	return row
}

// leafSections keeps sections that contain no child heading.
func leafSections(sections []Section) []Section {
	var leaves []Section
	for i, s := range sections {
		hasChild := false
		for j, other := range sections {
			if i == j {
				continue
			}
			if other.Level > s.Level && other.StartLine > s.StartLine && other.StartLine <= s.EndLine {
				hasChild = true
				break
			}
		}
		if !hasChild {
			leaves = append(leaves, s)
		}
	}
	return leaves
}

func maxOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	best := values[0]
	for _, v := range values[1:] {
		if v > best {
			best = v
		}
	}
	return best
}

func medianOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
