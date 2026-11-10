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

package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNegotiateProtocolVersion(t *testing.T) {
	tests := []struct {
		name   string
		params string
		want   string
	}{
		{name: "supported older version", params: `{"protocolVersion":"2024-11-05"}`, want: "2024-11-05"},
		{name: "supported current version", params: `{"protocolVersion":"2025-06-18"}`, want: "2025-06-18"},
		{name: "unknown version falls back to newest", params: `{"protocolVersion":"1999-01-01"}`, want: SupportedProtocolVersions[0]},
		{name: "missing version falls back to newest", params: `{}`, want: SupportedProtocolVersions[0]},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := negotiateProtocolVersion(json.RawMessage(tc.params)); got != tc.want {
				t.Errorf("negotiateProtocolVersion(%s) = %q, want %q", tc.params, got, tc.want)
			}
		})
	}
}

func TestToolsCarryAgentMetadata(t *testing.T) {
	for _, tool := range Tools() {
		if tool.Annotations == nil {
			t.Errorf("tool %q has no annotations", tool.Name)
			continue
		}
		if tool.Annotations.Title == "" {
			t.Errorf("tool %q has no annotation title", tool.Name)
		}
		if tool.Name != "dojo_run" && tool.OutputSchema == nil {
			t.Errorf("tool %q has no outputSchema", tool.Name)
		}
	}
}

func TestWriteToolsExposeDryRun(t *testing.T) {
	for _, name := range []string{"dojo_learn", "dojo_sync", "dojo_profile"} {
		tool := findTool(t, name)
		props, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("tool %q has no properties", name)
		}
		if _, ok := props["dry_run"]; !ok {
			t.Errorf("tool %q does not expose dry_run", name)
		}
	}
}

func findTool(t *testing.T, name string) Tool {
	t.Helper()
	for _, tool := range Tools() {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found", name)
	return Tool{}
}

func TestServeReturnsStructuredContentAndInstructions(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"fake","arguments":{}}}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	err := Serve(strings.NewReader(input), &out, func(name string, args map[string]any) ToolResult {
		return ToolResult{Text: `{"ok":true}`, Data: map[string]any{"ok": true}}
	})
	if err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		`"protocolVersion":"2024-11-05"`,
		`"instructions"`,
		`"structuredContent":{"ok":true}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %s in output, got:\n%s", want, got)
		}
	}
}
