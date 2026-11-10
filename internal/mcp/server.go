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
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jadmadi/dojo/internal/version"
)

// Tool represents an MCP tool definition.
type Tool struct {
	Name         string           `json:"name"`
	Title        string           `json:"title,omitempty"`
	Description  string           `json:"description"`
	InputSchema  map[string]any   `json:"inputSchema"`
	OutputSchema map[string]any   `json:"outputSchema,omitempty"`
	Annotations  *ToolAnnotations `json:"annotations,omitempty"`
}

// ToolAnnotations carries MCP behaviour hints for clients.
type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    bool   `json:"readOnlyHint"`
	DestructiveHint bool   `json:"destructiveHint"`
	IdempotentHint  bool   `json:"idempotentHint"`
	OpenWorldHint   bool   `json:"openWorldHint"`
}

// ToolResult is the executor's return value: display text plus optional
// structured data that matches the tool's OutputSchema.
type ToolResult struct {
	Text    string
	Data    map[string]any
	IsError bool
}

// ToolCallParams represents parameters for tools/call.
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ContentBlock represents an MCP text content block.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolCallResult represents the result of tools/call.
type ToolCallResult struct {
	Content           []ContentBlock `json:"content"`
	StructuredContent map[string]any `json:"structuredContent,omitempty"`
	IsError           bool           `json:"isError,omitempty"`
}

// Request represents a JSON-RPC 2.0 request or notification.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolExecutor executes a Dojo tool call.
type ToolExecutor func(name string, args map[string]any) ToolResult

// SupportedProtocolVersions lists the MCP protocol versions this server accepts,
// newest first.
var SupportedProtocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// Instructions tells the client how to use the tools in one place.
const Instructions = "Dojo manages Agent Skills. Workflow: dojo_find to locate a skill, dojo_toc to inspect section sizes, dojo_load to read only the needed section, dojo_run to execute a bundled script, dojo_learn to install, dojo_sync to project across harnesses. Read-only tools are find, toc, load, and doctor. Tools that execute or write are run, learn, sync, and profile; pass dry_run where available before committing."

// Tools returns the standard list of Dojo MCP tools.
func Tools() []Tool {
	tools := []Tool{
		{
			Name:        "dojo_find",
			Description: "Search Agent Skills by keyword across names, descriptions, categories, and tags.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search keywords (e.g. 'go testing', 'react', 'docker')"},
					"limit": map[string]any{"type": "integer", "description": "Max results to return (default 10)"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "dojo_toc",
			Description: "List Markdown section headings, #slugs, line ranges, and byte sizes for a skill before loading.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"skill": map[string]any{"type": "string", "description": "Skill name or skill/subpath"},
				},
				"required": []string{"skill"},
			},
		},
		{
			Name:        "dojo_load",
			Description: "Load skill instructions, a specific #section, or a reference file on demand.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"skill":     map[string]any{"type": "string", "description": "Skill name, skill/subpath, or skill#section"},
					"section":   map[string]any{"type": "string", "description": "Optional section slug or title to extract"},
					"page":      map[string]any{"type": "integer", "description": "Page number (1-based, default 1)"},
					"with_deps": map[string]any{"type": "boolean", "description": "Include transitive skill dependencies"},
				},
				"required": []string{"skill"},
			},
		},
		{
			Name:        "dojo_run",
			Description: "Execute a bundled script from a skill package.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"skill":  map[string]any{"type": "string", "description": "Skill name"},
					"script": map[string]any{"type": "string", "description": "Script filename inside scripts/"},
					"args": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional arguments passed to the script",
					},
				},
				"required": []string{"skill", "script"},
			},
		},
		{
			Name:        "dojo_learn",
			Description: "Install a skill into workspace or global scope and optionally sync across agent harnesses.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"skill":     map[string]any{"type": "string", "description": "Skill name or github:owner/repo"},
					"global":    map[string]any{"type": "boolean", "description": "Install to global ~/.agents/skills"},
					"with_deps": map[string]any{"type": "boolean", "description": "Install transitive skill dependencies"},
					"target":    map[string]any{"type": "string", "description": "Harnesses to sync (all, claude, cursor, gemini, windsurf, opencode, sila)"},
				},
				"required": []string{"skill"},
			},
		},
		{
			Name:        "dojo_doctor",
			Description: "Run Dojo environment diagnostics, frontmatter linting, token budget analysis, and security scan.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"skills":   map[string]any{"type": "boolean", "description": "Audit installed skills for frontmatter and token budget compliance"},
					"security": map[string]any{"type": "boolean", "description": "Scan installed skills and scripts for prompt injection, unsafe pipes, and secrets"},
					"global":   map[string]any{"type": "boolean", "description": "Include global skills (~/.agents/skills/)"},
				},
			},
		},
		{
			Name:        "dojo_sync",
			Description: "Bi-directionally synchronize and project installed skills across AI coding harnesses (Claude, Cursor, Antigravity, Gemini, Windsurf, OpenCode, Sila).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"skills": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional list of specific skill names to sync (defaults to all installed skills)",
					},
					"target":  map[string]any{"type": "string", "description": "Target harnesses (all, auto, claude, cursor, antigravity, gemini, windsurf, opencode, sila)"},
					"global":  map[string]any{"type": "boolean", "description": "Sync global scope (~/.agents/skills/)"},
					"one_way": map[string]any{"type": "boolean", "description": "Disable reverse reconciliation before projecting"},
				},
			},
		},
		{
			Name:        "dojo_profile",
			Description: "Manage the Personal Skills Registry and traveling skills.json manifest (link, unlink, pull, push, sync, export).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"description": "Profile action: link, unlink, pull, push, sync, or export",
						"enum":        []string{"link", "unlink", "pull", "push", "sync", "export"},
					},
					"source": map[string]any{"type": "string", "description": "Optional repository target (github:owner/repo) or manifest path"},
					"skills": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional skill/command names when action is 'push'",
					},
					"global": map[string]any{"type": "boolean", "description": "Operate on global profile scope"},
				},
				"required": []string{"action"},
			},
		},
	}
	enrichTools(tools)
	return tools
}

// toolMeta carries the MCP annotations and output contract for one tool.
type toolMeta struct {
	title       string
	annotations ToolAnnotations
	output      map[string]any
	extraInput  map[string]any
}

func enrichTools(tools []Tool) {
	resultsArray := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"results": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
		},
		"required": []string{"results"},
	}

	meta := map[string]toolMeta{
		"dojo_find": {
			title: "Search skills",
			annotations: ToolAnnotations{
				ReadOnlyHint: true, IdempotentHint: true,
			},
			output: resultsArray,
		},
		"dojo_toc": {
			title: "Inspect skill sections",
			annotations: ToolAnnotations{
				ReadOnlyHint: true, IdempotentHint: true,
			},
			output: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":        map[string]any{"type": "string"},
					"total_bytes": map[string]any{"type": "integer"},
					"sections":    map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				},
			},
			extraInput: map[string]any{
				"local":  map[string]any{"type": "boolean", "description": "Use the cached or installed copy only"},
				"update": map[string]any{"type": "boolean", "description": "Refresh the cached skill before listing"},
				"force":  map[string]any{"type": "boolean", "description": "Bypass digest verification when refreshing"},
			},
		},
		"dojo_load": {
			title: "Load skill content",
			annotations: ToolAnnotations{
				ReadOnlyHint: true, IdempotentHint: true,
			},
			output: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":        map[string]any{"type": "string"},
					"section":     map[string]any{"type": "string"},
					"page":        map[string]any{"type": "integer"},
					"total_pages": map[string]any{"type": "integer"},
					"content":     map[string]any{"type": "string"},
				},
			},
			extraInput: map[string]any{
				"local": map[string]any{"type": "boolean", "description": "Use the cached or installed copy only"},
				"full":  map[string]any{"type": "boolean", "description": "Return the complete unpaginated content"},
			},
		},
		"dojo_run": {
			title: "Run a skill script",
			annotations: ToolAnnotations{
				DestructiveHint: true, OpenWorldHint: true,
			},
		},
		"dojo_learn": {
			title: "Install a skill",
			annotations: ToolAnnotations{
				IdempotentHint: true, OpenWorldHint: true,
			},
			output: resultsArray,
			extraInput: map[string]any{
				"dry_run": map[string]any{"type": "boolean", "description": "Resolve and report planned installs without writing"},
			},
		},
		"dojo_doctor": {
			title: "Audit skills",
			annotations: ToolAnnotations{
				ReadOnlyHint: true, IdempotentHint: true,
			},
			output: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"healthy":  map[string]any{"type": "boolean"},
					"warnings": map[string]any{"type": "integer"},
					"findings": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				},
			},
		},
		"dojo_sync": {
			title: "Sync skills across harnesses",
			annotations: ToolAnnotations{
				IdempotentHint: true, OpenWorldHint: true,
			},
			output: resultsArray,
			extraInput: map[string]any{
				"dry_run": map[string]any{"type": "boolean", "description": "Report the planned projections without writing"},
			},
		},
		"dojo_profile": {
			title: "Manage the personal skill registry",
			annotations: ToolAnnotations{
				DestructiveHint: true, OpenWorldHint: true,
			},
			output: map[string]any{
				"type":       "object",
				"properties": map[string]any{"result": map[string]any{}},
			},
			extraInput: map[string]any{
				"dry_run": map[string]any{"type": "boolean", "description": "Supported for the push action: report packages without writing or pushing"},
			},
		},
	}

	for i := range tools {
		m, ok := meta[tools[i].Name]
		if !ok {
			continue
		}
		tools[i].Title = m.title
		tools[i].OutputSchema = m.output
		ann := m.annotations
		if ann.Title == "" {
			ann.Title = m.title
		}
		tools[i].Annotations = &ann
		addToolInputProperties(&tools[i], m.extraInput)
	}
}

func addToolInputProperties(tool *Tool, extra map[string]any) {
	if len(extra) == 0 {
		return
	}
	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		props = map[string]any{}
		tool.InputSchema["properties"] = props
	}
	for k, v := range extra {
		props[k] = v
	}
}

// negotiateProtocolVersion echoes the client's requested version when this
// server supports it, otherwise it returns the newest supported version.
func negotiateProtocolVersion(params json.RawMessage) string {
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &init)
	for _, v := range SupportedProtocolVersions {
		if v == init.ProtocolVersion {
			return v
		}
	}
	return SupportedProtocolVersions[0]
}

func Serve(r io.Reader, w io.Writer, execTool ToolExecutor) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	writeResp := func(resp Response) {
		data, err := json.Marshal(resp)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(w, "%s\n", data)
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeResp(Response{
				JSONRPC: "2.0",
				ID:      json.RawMessage("null"),
				Error:   &RPCError{Code: -32700, Message: "Parse error"},
			})
			continue
		}

		// Notifications have no ID
		if len(req.ID) == 0 || string(req.ID) == "null" {
			if strings.HasPrefix(req.Method, "notifications/") {
				continue
			}
		}

		switch req.Method {
		case "initialize":
			writeResp(Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"protocolVersion": negotiateProtocolVersion(req.Params),
					"capabilities": map[string]any{
						"tools": map[string]any{},
					},
					"serverInfo": map[string]any{
						"name":    "dojo",
						"version": version.Get(),
					},
					"instructions": Instructions,
				},
			})
		case "ping":
			writeResp(Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  map[string]any{},
			})
		case "tools/list":
			writeResp(Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"tools": Tools(),
				},
			})
		case "tools/call":
			var params ToolCallParams
			if err := json.Unmarshal(req.Params, &params); err != nil || params.Name == "" {
				writeResp(Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   &RPCError{Code: -32602, Message: "Invalid tool call params"},
				})
				continue
			}
			res := execTool(params.Name, params.Arguments)
			text := res.Text
			if text == "" && res.IsError {
				text = "tool call failed"
			}
			result := ToolCallResult{
				Content: []ContentBlock{{Type: "text", Text: text}},
				IsError: res.IsError,
			}
			if res.Data != nil {
				result.StructuredContent = res.Data
			}
			writeResp(Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			})
		default:
			if len(req.ID) > 0 && string(req.ID) != "null" {
				writeResp(Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
				})
			}
		}
	}
	return scanner.Err()
}
