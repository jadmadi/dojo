// Copyright 2026 The Dojo Authors
// Licensed under the Apache License, Version 2.0

package commands

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/jadmadi/dojo/internal/catalog"
)

type mockStore struct {
	catalog.Store
	items []catalog.Item
}

func (m *mockStore) All() []catalog.Item {
	return m.items
}

func TestResolveSkillSpecs(t *testing.T) {
	store := &mockStore{
		items: []catalog.Item{
			{Name: "abstract"},
			{Name: "buffer"},
			{Name: "cli-reviewer"},
			{Name: "godoctor"},
		},
	}

	var stderr bytes.Buffer
	app := &App{
		Store:  store,
		Stderr: &stderr,
	}

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "plain names",
			input:    []string{"buffer", "godoctor"},
			expected: []string{"buffer", "godoctor"},
		},
		{
			name:     "comma-separated names",
			input:    []string{"abstract,cli-reviewer"},
			expected: []string{"abstract", "cli-reviewer"},
		},
		{
			name:     "comma-separated numbers",
			input:    []string{"1,3"},
			expected: []string{"abstract", "cli-reviewer"},
		},
		{
			name:     "mixed numbers and names and out of bounds",
			input:    []string{"2", "99", "cli-reviewer,1"},
			expected: []string{"buffer", "cli-reviewer", "abstract"},
		},
		{
			name:     "deduplication",
			input:    []string{"1", "abstract", "1"},
			expected: []string{"abstract"},
		},
		{
			name:     "compound numeric section",
			input:    []string{"1#architecture", "3#quick-start"},
			expected: []string{"abstract#architecture", "cli-reviewer#quick-start"},
		},
		{
			name:     "compound numeric subpath",
			input:    []string{"2/references/guide.md"},
			expected: []string{"buffer/references/guide.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := app.resolveSkillSpecs(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("resolveSkillSpecs(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
