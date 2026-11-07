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
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
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
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
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

// ToolExecutor executes a Dojo tool call and returns (output, isError).
type ToolExecutor func(name string, args map[string]any) (string, bool)

// Tools returns the standard list of Dojo MCP tools.
func Tools() []Tool {
	return []Tool{
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
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// Serve runs the JSON-RPC 2.0 MCP server loop over r and w.
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
					"protocolVersion": "2024-11-05",
					"capabilities": map[string]any{
						"tools": map[string]any{},
					},
					"serverInfo": map[string]any{
						"name":    "dojo",
						"version": version.Get(),
					},
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
			text, isErr := execTool(params.Name, params.Arguments)
			writeResp(Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: ToolCallResult{
					Content: []ContentBlock{{Type: "text", Text: text}},
					IsError: isErr,
				},
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
