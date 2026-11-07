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
	"fmt"
	"strings"
	"unicode"
)

// Section represents a parsed Markdown heading section inside a SKILL.md document.
type Section struct {
	Heading   string `json:"heading"`
	Slug      string `json:"slug"`
	Level     int    `json:"level"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Bytes     int    `json:"bytes"`
	Content   string `json:"content,omitempty"`
}

// SlugifyHeading converts a Markdown heading string into a GitHub-compatible anchor slug.
func SlugifyHeading(heading string) string {
	s := strings.TrimSpace(heading)
	// Strip inline code backticks or markdown links if simple
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ToLower(s)

	var sb strings.Builder
	lastHyphen := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			lastHyphen = false
		} else if r == ' ' || r == '-' || r == '_' || r == '/' || r == '.' {
			if sb.Len() > 0 && !lastHyphen {
				sb.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	res := strings.Trim(sb.String(), "-")
	if res == "" {
		return "section"
	}
	return res
}

// ParseSections parses Markdown content into hierarchical sections while ignoring headings inside fenced code blocks.
func ParseSections(markdown string) []Section {
	// Strip YAML frontmatter first if present
	_, bodyBytes, hasFM, err := ExtractFrontmatter([]byte(markdown))
	content := markdown
	if err == nil && hasFM {
		content = string(bodyBytes)
	}

	lines := strings.Split(content, "\n")
	type rawHeading struct {
		title string
		slug  string
		level int
		line  int // 0-indexed into lines
	}

	var headings []rawHeading
	slugCounts := make(map[string]int)
	inFence := false
	fenceChar := byte(0)
	fenceLen := 0

	for i, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)

		// Detect fenced code blocks (``` or ~~~)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			ch := trimmed[0]
			cnt := 0
			for cnt < len(trimmed) && trimmed[cnt] == ch {
				cnt++
			}
			if !inFence {
				inFence = true
				fenceChar = ch
				fenceLen = cnt
				continue
			} else if ch == fenceChar && cnt >= fenceLen && strings.TrimSpace(trimmed[cnt:]) == "" {
				inFence = false
				fenceChar = 0
				fenceLen = 0
				continue
			}
		}

		if inFence {
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			lvl := 0
			for lvl < len(trimmed) && trimmed[lvl] == '#' {
				lvl++
			}
			if lvl >= 1 && lvl <= 6 && lvl < len(trimmed) && (trimmed[lvl] == ' ' || trimmed[lvl] == '\t') {
				title := strings.TrimSpace(strings.TrimRight(trimmed[lvl:], "# \t"))
				if title != "" {
					baseSlug := SlugifyHeading(title)
					count := slugCounts[baseSlug]
					slugCounts[baseSlug] = count + 1
					finalSlug := baseSlug
					if count > 0 {
						finalSlug = fmt.Sprintf("%s-%d", baseSlug, count)
					}
					headings = append(headings, rawHeading{
						title: title,
						slug:  finalSlug,
						level: lvl,
						line:  i,
					})
				}
			}
		}
	}

	if len(headings) == 0 {
		return nil
	}

	var sections []Section
	for idx, h := range headings {
		endLine := len(lines) - 1
		// A section extends until the next heading of equal or higher (smaller number) level
		for nextIdx := idx + 1; nextIdx < len(headings); nextIdx++ {
			if headings[nextIdx].level <= h.level {
				endLine = headings[nextIdx].line - 1
				break
			}
		}
		secLines := lines[h.line : endLine+1]
		secContent := strings.TrimRight(strings.Join(secLines, "\n"), "\n") + "\n"
		sections = append(sections, Section{
			Heading:   h.title,
			Slug:      h.slug,
			Level:     h.level,
			StartLine: h.line + 1,
			EndLine:   endLine + 1,
			Bytes:     len(secContent),
			Content:   secContent,
		})
	}

	return sections
}

// ExtractSection finds a section by slug or heading substring and returns its full content (including nested subsections).
func ExtractSection(markdown, query string) (*Section, error) {
	sections := ParseSections(markdown)
	if len(sections) == 0 {
		return nil, fmt.Errorf("no markdown sections found in document")
	}

	cleanQuery := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(query, "#")))
	slugQuery := SlugifyHeading(cleanQuery)

	// 1. Exact slug match
	for i := range sections {
		if sections[i].Slug == cleanQuery || sections[i].Slug == slugQuery {
			return &sections[i], nil
		}
	}

	// 2. Exact case-insensitive heading title match
	for i := range sections {
		if strings.EqualFold(sections[i].Heading, cleanQuery) {
			return &sections[i], nil
		}
	}

	// 3. Substring match on slug or heading
	for i := range sections {
		if strings.Contains(sections[i].Slug, slugQuery) || strings.Contains(strings.ToLower(sections[i].Heading), cleanQuery) {
			return &sections[i], nil
		}
	}

	var available []string
	for _, s := range sections {
		available = append(available, "#"+s.Slug)
	}
	return nil, fmt.Errorf("section %q not found (available sections: %s)", query, strings.Join(available, ", "))
}
