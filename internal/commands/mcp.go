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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jadmadi/dojo/internal/mcp"
)

// Stdin allows tests to override standard input for RunMCP.
var Stdin io.Reader = os.Stdin

// RunMCP starts the native Dojo MCP (Model Context Protocol) server over stdio,
// or prints the MCP tool schema when --print-schema is passed.
func (a *App) RunMCP(args []string) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	var printSchema bool

	fs.BoolVar(&printSchema, "print-schema", false, "Print the MCP tools JSON schema and exit")

	if _, err := parseFlagsWithArgs(fs, args); err != nil {
		return ExitUsage
	}

	if printSchema {
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"name":  "dojo",
			"tools": mcp.Tools(),
		})
		return ExitSuccess
	}

	err := mcp.Serve(Stdin, a.Stdout, func(name string, toolArgs map[string]any) mcp.ToolResult {
		return a.executeMCPTool(name, toolArgs)
	})
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: mcp server error: %v\n", err)
		return ExitGeneral
	}
	return ExitSuccess
}

// executeMCPTool runs one tool as a sub-command and returns JSON text plus
// structured data for clients that support it.
func (a *App) executeMCPTool(name string, toolArgs map[string]any) mcp.ToolResult {
	cmdArgs, wrapKey, err := buildMCPToolArgs(name, toolArgs)
	if err != nil {
		return mcp.ToolResult{Text: err.Error(), IsError: true}
	}

	var outBuf, errBuf bytes.Buffer
	subApp := *a
	subApp.Stdout = &outBuf
	subApp.Stderr = &errBuf
	code := dispatchMCPTool(&subApp, name, cmdArgs)

	combined := strings.TrimSpace(outBuf.String())
	if errBuf.Len() > 0 {
		if combined != "" {
			combined += "\n" + strings.TrimSpace(errBuf.String())
		} else {
			combined = strings.TrimSpace(errBuf.String())
		}
	}
	if code != ExitSuccess {
		return mcp.ToolResult{Text: combined, IsError: true}
	}
	if combined == "" {
		return mcp.ToolResult{Text: "ok"}
	}

	data, ok := structuredMCPResult(combined, wrapKey)
	if !ok {
		return mcp.ToolResult{Text: combined}
	}
	return mcp.ToolResult{Text: combined, Data: data}
}

// structuredMCPResult converts the command's JSON output into the object shape
// declared by the tool's outputSchema.
func structuredMCPResult(text, wrapKey string) (map[string]any, bool) {
	var decoded any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		return nil, false
	}
	if wrapKey != "" {
		items, ok := decoded.([]any)
		if !ok {
			return nil, false
		}
		return map[string]any{wrapKey: items}, true
	}
	if obj, ok := decoded.(map[string]any); ok {
		return obj, true
	}
	if items, ok := decoded.([]any); ok {
		return map[string]any{"results": items}, true
	}
	return map[string]any{"result": decoded}, true
}

// buildMCPToolArgs maps tool arguments to CLI flags and requests JSON output
// wherever the command supports it.
func buildMCPToolArgs(name string, args map[string]any) ([]string, string, error) {
	switch name {
	case "dojo_find":
		cmdArgs := []string{"--json", fmt.Sprint(args["query"])}
		if lim, ok := intArg(args, "limit"); ok {
			cmdArgs = append(cmdArgs, "-n", fmt.Sprint(lim))
		}
		return cmdArgs, "results", nil

	case "dojo_toc":
		cmdArgs := []string{"--json", fmt.Sprint(args["skill"])}
		cmdArgs = appendBoolFlag(cmdArgs, args, "local", "--local")
		cmdArgs = appendBoolFlag(cmdArgs, args, "update", "--update")
		cmdArgs = appendBoolFlag(cmdArgs, args, "force", "--force")
		return cmdArgs, "", nil

	case "dojo_load":
		cmdArgs := []string{"--json", fmt.Sprint(args["skill"])}
		if sec, ok := stringArg(args, "section"); ok {
			cmdArgs = append(cmdArgs, "--section", sec)
		}
		if page, ok := intArg(args, "page"); ok {
			cmdArgs = append(cmdArgs, "-p", fmt.Sprint(page))
		}
		cmdArgs = appendBoolFlag(cmdArgs, args, "with_deps", "--with-deps")
		cmdArgs = appendBoolFlag(cmdArgs, args, "local", "--local")
		cmdArgs = appendBoolFlag(cmdArgs, args, "full", "--full")
		return cmdArgs, "", nil

	case "dojo_run":
		cmdArgs := []string{fmt.Sprint(args["skill"]), fmt.Sprint(args["script"])}
		if rawArgs, ok := args["args"].([]any); ok {
			for _, ra := range rawArgs {
				cmdArgs = append(cmdArgs, fmt.Sprint(ra))
			}
		}
		return cmdArgs, "", nil

	case "dojo_learn":
		cmdArgs := []string{"--json", fmt.Sprint(args["skill"])}
		cmdArgs = appendBoolFlag(cmdArgs, args, "global", "-g")
		cmdArgs = appendBoolFlag(cmdArgs, args, "with_deps", "--with-deps")
		cmdArgs = appendBoolFlag(cmdArgs, args, "dry_run", "--dry-run")
		if target, ok := stringArg(args, "target"); ok {
			cmdArgs = append(cmdArgs, "--target", target)
		}
		return cmdArgs, "results", nil

	case "dojo_doctor":
		cmdArgs := []string{"--json"}
		cmdArgs = appendBoolFlag(cmdArgs, args, "skills", "--skills")
		cmdArgs = appendBoolFlag(cmdArgs, args, "security", "--security")
		cmdArgs = appendBoolFlag(cmdArgs, args, "global", "-g")
		return cmdArgs, "", nil

	case "dojo_sync":
		cmdArgs := []string{"--json"}
		if rawSkills, ok := args["skills"].([]any); ok {
			for _, rs := range rawSkills {
				if s := strings.TrimSpace(fmt.Sprint(rs)); s != "" {
					cmdArgs = append(cmdArgs, s)
				}
			}
		}
		if target, ok := stringArg(args, "target"); ok {
			cmdArgs = append(cmdArgs, "--target", target)
		}
		cmdArgs = appendBoolFlag(cmdArgs, args, "global", "-g")
		cmdArgs = appendBoolFlag(cmdArgs, args, "one_way", "--one-way")
		cmdArgs = appendBoolFlag(cmdArgs, args, "dry_run", "--dry-run")
		return cmdArgs, "results", nil

	case "dojo_profile":
		action := "export"
		if act, ok := stringArg(args, "action"); ok {
			action = act
		}
		if dryRun, _ := boolArg(args, "dry_run"); dryRun && action != "push" {
			return nil, "", fmt.Errorf("dry_run is supported for the profile push action only")
		}

		cmdArgs := []string{"--json", action}
		if src, ok := stringArg(args, "source"); ok {
			cmdArgs = append(cmdArgs, src)
		}
		if rawSkills, ok := args["skills"].([]any); ok {
			for _, rs := range rawSkills {
				if s := strings.TrimSpace(fmt.Sprint(rs)); s != "" {
					cmdArgs = append(cmdArgs, s)
				}
			}
		}
		cmdArgs = appendBoolFlag(cmdArgs, args, "global", "-g")
		cmdArgs = appendBoolFlag(cmdArgs, args, "dry_run", "--dry-run")
		return cmdArgs, "", nil

	default:
		return nil, "", fmt.Errorf("unknown tool %q", name)
	}
}

func dispatchMCPTool(app *App, name string, cmdArgs []string) int {
	switch name {
	case "dojo_find":
		return app.RunFind(cmdArgs)
	case "dojo_toc":
		return app.RunTOC(cmdArgs)
	case "dojo_load":
		return app.RunLoad(cmdArgs)
	case "dojo_run":
		return app.RunRun(cmdArgs)
	case "dojo_learn":
		return app.RunLearn(cmdArgs)
	case "dojo_doctor":
		return app.RunDoctor(cmdArgs)
	case "dojo_sync":
		return app.RunSync(cmdArgs)
	case "dojo_profile":
		return app.RunProfile(cmdArgs)
	default:
		return ExitUsage
	}
}

func stringArg(args map[string]any, key string) (string, bool) {
	value, ok := args[key].(string)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func boolArg(args map[string]any, key string) (bool, bool) {
	value, ok := args[key].(bool)
	return value, ok
}

func intArg(args map[string]any, key string) (int, bool) {
	value, ok := args[key].(float64)
	if !ok {
		return 0, false
	}
	return int(value), true
}

func appendBoolFlag(cmdArgs []string, args map[string]any, key, flagName string) []string {
	if enabled, ok := boolArg(args, key); ok && enabled {
		return append(cmdArgs, flagName)
	}
	return cmdArgs
}
