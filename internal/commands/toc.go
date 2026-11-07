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
	"strings"
	"time"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/runner"
)

// RunTOC parses and displays the Markdown table of contents (headings, slugs, line ranges, and byte sizes)
// for a skill or skill reference file.
func (a *App) RunTOC(args []string) int {
	fs := flag.NewFlagSet("toc", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var localFlag, jsonFlag, rawFlag bool

	fs.BoolVar(&localFlag, "local", false, "Use locally cached or installed copy without checking remote upstream")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw tab-separated format")

	posArgs, err := parseFlagsWithArgs(fs, args)
	if err != nil {
		return ExitUsage
	}
	if len(posArgs) == 0 {
		_, _ = fmt.Fprintln(a.Stderr, "error: toc requires a skill name or subpath (e.g. 'dojo toc godoctor')")
		return ExitUsage
	}

	mode := a.resolveMode(jsonFlag, rawFlag)
	spec := posArgs[0]
	if idx := strings.Index(spec, "#"); idx > 0 {
		spec = spec[:idx]
	}

	parts := strings.SplitN(spec, "/", 2)
	skillName := parts[0]
	subpath := ""
	if len(parts) == 2 {
		subpath = parts[1]
	}

	item, err := a.Store.Get(skillName)
	if err != nil {
		a.printNotFoundWithSuggestions("toc", skillName)
		return ExitNotFound
	}

	var content string
	if subpath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		subItem := *item
		if localFlag {
			subItem.URL = ""
			subItem.Metadata = nil
		}
		content, err = runner.ReadFileContext(ctx, subItem, subpath, a.getHTTPClient())
		if err != nil {
			_, _ = fmt.Fprintf(a.Stderr, "error: loading %s/%s: %v\n", item.Name, subpath, err)
			return MapErrorToExitCode(err)
		}
	} else {
		rawContent, err := a.loadItemContentWithOpts(item, localFlag, false, false)
		if err != nil {
			_, _ = fmt.Fprintf(a.Stderr, "error: loading skill %q: %v\n", item.Name, err)
			return MapErrorToExitCode(err)
		}
		content = stripFrontmatter(rawContent)
	}

	sections := catalog.ParseSections(content)

	type TOCResult struct {
		Name       string            `json:"name"`
		Subpath    string            `json:"subpath,omitempty"`
		TotalBytes int               `json:"total_bytes"`
		Sections   []catalog.Section `json:"sections"`
	}

	res := TOCResult{
		Name:       item.Name,
		Subpath:    subpath,
		TotalBytes: len(content),
		Sections:   sections,
	}

	switch mode {
	case OutputModeJSON:
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return ExitSuccess
	case OutputModeRaw:
		for _, s := range sections {
			_, _ = fmt.Fprintf(a.Stdout, "%d\t%s\t%s\t%d-%d\t%d\n",
				s.Level,
				sanitizeTSVField(s.Slug),
				sanitizeTSVField(s.Heading),
				s.StartLine,
				s.EndLine,
				s.Bytes,
			)
		}
		return ExitSuccess
	default:
		_, _ = fmt.Fprintf(a.Stdout, "TABLE OF CONTENTS: %s (%d bytes, %d sections)\n\n", spec, len(content), len(sections))
		if len(sections) == 0 {
			_, _ = fmt.Fprintln(a.Stdout, "  (No markdown headings found)")
			return ExitSuccess
		}
		for _, s := range sections {
			indent := strings.Repeat("  ", max(0, s.Level-1))
			_, _ = fmt.Fprintf(a.Stdout, "  %s• %-34s [lines %d-%d, %dB]\n    %s  ↳ dojo load %s#%s\n",
				indent, s.Heading, s.StartLine, s.EndLine, s.Bytes, indent, spec, s.Slug)
		}
		return ExitSuccess
	}
}
