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
	"errors"
	"fmt"
	"strings"
)

// ErrCircularDependency is returned when a skill dependency graph contains a cycle.
var ErrCircularDependency = errors.New("circular dependency detected")

// ResolveDependencies resolves the transitive dependency graph for rootSkill in topological order
// (dependencies first, ending with rootSkill). Returns ErrCircularDependency if a cycle exists.
func ResolveDependencies(rootSkill string, store Store) ([]Item, error) {
	if store == nil {
		return nil, fmt.Errorf("catalog store is nil")
	}

	visited := make(map[string]int) // 0=unvisited, 1=visiting, 2=visited
	var order []Item
	var path []string

	var dfs func(name string) error
	dfs = func(name string) error {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			return nil
		}
		switch visited[key] {
		case 1:
			cyclePath := append(append([]string(nil), path...), key)
			return fmt.Errorf("%w: %s", ErrCircularDependency, strings.Join(cyclePath, " -> "))
		case 2:
			return nil
		}

		visited[key] = 1
		path = append(path, key)

		item, err := store.Get(name)
		if err != nil || item == nil {
			return fmt.Errorf("dependency %q not found: %w", name, ErrItemNotFound)
		}

		// Ensure frontmatter dependencies are enriched if body is loaded or available
		deps := item.Dependencies
		if len(deps) == 0 && item.Metadata != nil && item.Metadata["dependencies"] != "" {
			deps = ParseTags(item.Metadata["dependencies"])
		}
		if len(deps) == 0 && item.Body != "" {
			EnrichItemFromSkillMD(item, []byte(item.Body))
			deps = item.Dependencies
		}

		for _, dep := range deps {
			if err := dfs(dep); err != nil {
				return err
			}
		}

		path = path[:len(path)-1]
		visited[key] = 2
		order = append(order, *item)
		return nil
	}

	if err := dfs(rootSkill); err != nil {
		return nil, err
	}

	return order, nil
}
