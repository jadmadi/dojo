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

	err := mcp.Serve(Stdin, a.Stdout, func(name string, toolArgs map[string]any) (string, bool) {
		var outBuf, errBuf bytes.Buffer
		subApp := *a
		subApp.Stdout = &outBuf
		subApp.Stderr = &errBuf

		var code int
		switch name {
		case "dojo_find":
			q := fmt.Sprint(toolArgs["query"])
			cmdArgs := []string{q}
			if lim, ok := toolArgs["limit"]; ok && lim != nil {
				cmdArgs = append(cmdArgs, "-n", fmt.Sprint(lim))
			}
			code = subApp.RunFind(cmdArgs)
		case "dojo_toc":
			sk := fmt.Sprint(toolArgs["skill"])
			code = subApp.RunTOC([]string{sk})
		case "dojo_load":
			sk := fmt.Sprint(toolArgs["skill"])
			cmdArgs := []string{sk}
			if sec, ok := toolArgs["section"].(string); ok && sec != "" {
				cmdArgs = append(cmdArgs, "--section", sec)
			}
			if page, ok := toolArgs["page"]; ok && page != nil {
				cmdArgs = append(cmdArgs, "-p", fmt.Sprint(page))
			}
			if wd, ok := toolArgs["with_deps"].(bool); ok && wd {
				cmdArgs = append(cmdArgs, "--with-deps")
			}
			code = subApp.RunLoad(cmdArgs)
		case "dojo_run":
			sk := fmt.Sprint(toolArgs["skill"])
			scr := fmt.Sprint(toolArgs["script"])
			cmdArgs := []string{sk, scr}
			if rawArgs, ok := toolArgs["args"].([]any); ok {
				for _, ra := range rawArgs {
					cmdArgs = append(cmdArgs, fmt.Sprint(ra))
				}
			}
			code = subApp.RunRun(cmdArgs)
		case "dojo_learn":
			sk := fmt.Sprint(toolArgs["skill"])
			cmdArgs := []string{sk}
			if g, ok := toolArgs["global"].(bool); ok && g {
				cmdArgs = append(cmdArgs, "-g")
			}
			if wd, ok := toolArgs["with_deps"].(bool); ok && wd {
				cmdArgs = append(cmdArgs, "--with-deps")
			}
			if t, ok := toolArgs["target"].(string); ok && t != "" {
				cmdArgs = append(cmdArgs, "--target", t)
			}
			code = subApp.RunLearn(cmdArgs)
		case "dojo_doctor":
			var cmdArgs []string
			if sk, ok := toolArgs["skills"].(bool); ok && sk {
				cmdArgs = append(cmdArgs, "--skills")
			}
			if sec, ok := toolArgs["security"].(bool); ok && sec {
				cmdArgs = append(cmdArgs, "--security")
			}
			if g, ok := toolArgs["global"].(bool); ok && g {
				cmdArgs = append(cmdArgs, "-g")
			}
			code = subApp.RunDoctor(cmdArgs)
		case "dojo_sync":
			var cmdArgs []string
			if rawSkills, ok := toolArgs["skills"].([]any); ok {
				for _, rs := range rawSkills {
					if s := strings.TrimSpace(fmt.Sprint(rs)); s != "" {
						cmdArgs = append(cmdArgs, s)
					}
				}
			}
			if t, ok := toolArgs["target"].(string); ok && t != "" {
				cmdArgs = append(cmdArgs, "--target", t)
			}
			if g, ok := toolArgs["global"].(bool); ok && g {
				cmdArgs = append(cmdArgs, "-g")
			}
			if ow, ok := toolArgs["one_way"].(bool); ok && ow {
				cmdArgs = append(cmdArgs, "--one-way")
			}
			code = subApp.RunSync(cmdArgs)
		case "dojo_profile":
			action := "export"
			if act, ok := toolArgs["action"].(string); ok && strings.TrimSpace(act) != "" {
				action = strings.TrimSpace(act)
			}
			cmdArgs := []string{action}
			if src, ok := toolArgs["source"].(string); ok && strings.TrimSpace(src) != "" {
				cmdArgs = append(cmdArgs, strings.TrimSpace(src))
			}
			if rawSkills, ok := toolArgs["skills"].([]any); ok {
				for _, rs := range rawSkills {
					if s := strings.TrimSpace(fmt.Sprint(rs)); s != "" {
						cmdArgs = append(cmdArgs, s)
					}
				}
			}
			if g, ok := toolArgs["global"].(bool); ok && g {
				cmdArgs = append(cmdArgs, "-g")
			}
			code = subApp.RunProfile(cmdArgs)
		default:
			return fmt.Sprintf("unknown tool %q", name), true
		}

		combined := strings.TrimSpace(outBuf.String())
		if errBuf.Len() > 0 {
			if combined != "" {
				combined += "\n" + strings.TrimSpace(errBuf.String())
			} else {
				combined = strings.TrimSpace(errBuf.String())
			}
		}
		return combined, code != ExitSuccess
	})
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: mcp server error: %v\n", err)
		return ExitGeneral
	}
	return ExitSuccess
}
