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
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestFindInstalledGoBin_ToolchainDir(t *testing.T) {
	tempDir := t.TempDir()
	goBinDir := filepath.Join(tempDir, "mise", "installs", "go", "bin")
	if err := os.MkdirAll(goBinDir, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	binName := "dojo"
	if runtime.GOOS == "windows" {
		binName = "dojo.exe"
	}

	fakeGo := filepath.Join(goBinDir, "go")
	if runtime.GOOS == "windows" {
		fakeGo += ".exe"
	}
	if err := os.WriteFile(fakeGo, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("failed to write fake go: %v", err)
	}

	start := time.Now().Add(-1 * time.Second)
	fakeDojo := filepath.Join(goBinDir, binName)
	if err := os.WriteFile(fakeDojo, []byte("binary-content"), 0755); err != nil {
		t.Fatalf("failed to write fake dojo: %v", err)
	}

	found := findInstalledGoBin(fakeGo, start)
	expectedClean := filepath.Clean(fakeDojo)
	if filepath.Clean(found) != expectedClean {
		t.Errorf("findInstalledGoBin() = %v, want %v", found, expectedClean)
	}
}

func TestFindInstalledGoBin_PrefersNewer(t *testing.T) {
	tempDir := t.TempDir()
	dirA := filepath.Join(tempDir, "old_bin")
	dirB := filepath.Join(tempDir, "new_bin")
	_ = os.MkdirAll(dirA, 0755)
	_ = os.MkdirAll(dirB, 0755)

	binName := "dojo"
	if runtime.GOOS == "windows" {
		binName = "dojo.exe"
	}

	fileA := filepath.Join(dirA, binName)
	fileB := filepath.Join(dirB, binName)

	_ = os.WriteFile(fileA, []byte("old"), 0755)
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	time.Sleep(50 * time.Millisecond)
	_ = os.WriteFile(fileB, []byte("new"), 0755)

	fakeGo := filepath.Join(dirB, "go")
	_ = os.WriteFile(fakeGo, []byte("#!/bin/sh\n"), 0755)

	t.Setenv("PATH", dirA+string(os.PathListSeparator)+dirB)

	found := findInstalledGoBin(fakeGo, start)
	if filepath.Clean(found) != filepath.Clean(fileB) {
		t.Errorf("findInstalledGoBin() = %v, want %v", found, fileB)
	}
}
