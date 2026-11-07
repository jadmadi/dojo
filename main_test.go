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

package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	tempHome, err := os.MkdirTemp("", "dojo-main-test-home-*")
	if err == nil {
		_ = os.Setenv("HOME", tempHome)
		_ = os.Setenv("DOJO_CONFIG_DIR", filepath.Join(tempHome, ".config", "dojo"))
		_ = os.Setenv("DOJO_STATE_PATH", filepath.Join(tempHome, ".config", "dojo", "state.json"))
		_ = os.Setenv("DOJO_CONFIG", "")
		_ = os.Setenv("DOJO_TELEMETRY", "0")
		_ = os.Setenv("DO_NOT_TRACK", "")
	}
	code := m.Run()
	if tempHome != "" {
		_ = os.RemoveAll(tempHome)
	}
	os.Exit(code)
}

func TestRun_CLI_Basics(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantSub  string
	}{
		{"version long", []string{"--version"}, ExitSuccess, "dojo version"},
		{"version short", []string{"-V"}, ExitSuccess, "dojo version"},
		{"version lowercase -v", []string{"-v"}, ExitSuccess, "dojo version"},
		{"verbose flag alone", []string{"--verbose"}, ExitSuccess, "dojo version"},
		{"version command", []string{"version"}, ExitSuccess, "dojo version"},
		{"help long", []string{"--help"}, ExitSuccess, "profile    <link|sync|pull|push|export|apply>"},
		{"help short", []string{"-h"}, ExitSuccess, "USAGE:"},
		{"help command", []string{"help"}, ExitSuccess, "USAGE:"},
		{"no args", []string{}, ExitUsage, "USAGE:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Fatalf("Run(%v) = %d, want %d; stderr: %s", tt.args, code, tt.wantCode, stderr.String())
			}
			out := stdout.String()
			if !strings.Contains(out, tt.wantSub) {
				t.Errorf("Run(%v) output %q does not contain %q", tt.args, out, tt.wantSub)
			}
		})
	}
}

func TestRun_CLI_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"nonexistentcmd"}, &stdout, &stderr)
	if code != ExitUsage {
		t.Fatalf("Run(nonexistentcmd) = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Errorf("expected unknown command error, got: %s", stderr.String())
	}
}

func TestRun_CLI_ListAndShow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"1.0.0","items":[]}`))
	}))
	defer srv.Close()
	t.Setenv("DOJO_URL", srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"list"}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("Run(list) = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "CATEGORY") {
		t.Errorf("expected category listing in stdout: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"show", "dojo"}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("Run(show dojo) = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SKILL: dojo") {
		t.Errorf("expected dojo in show output: %s", stdout.String())
	}
}

func TestRun_CLI_Load(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"1.0.0","items":[]}`))
	}))
	defer srv.Close()
	t.Setenv("DOJO_URL", srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"load", "dojo"}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("Run(load dojo) = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# Dojo") {
		t.Errorf("expected markdown content: %s", stdout.String())
	}
}

func TestRun_CLI_RunBranding(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"run"}, &stdout, &stderr)
	if code != ExitUsage {
		t.Fatalf("Run(run) = %d, want %d", code, ExitUsage)
	}
	errStr := stderr.String()
	if !strings.Contains(errStr, "dojo run <skill> <script>") {
		t.Errorf("expected 'dojo run <skill> <script>' in error, got: %s", errStr)
	}
}

