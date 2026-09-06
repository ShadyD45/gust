package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// discoverScenarioPaths returns scenario files for a path.
// A file is returned as a one-element list. A directory yields
// **/scenario.yaml|yml|json (skipping _shared and hidden dirs) plus
// top-level *.yaml|yml|json that parse as scenarios.
func discoverScenarioPaths(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{root}, nil
	}

	root = filepath.Clean(root)
	var named []string
	var topLevel []string

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if skipWalkName(d.Name()) && path != root {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		base := strings.ToLower(d.Name())
		if isNamedScenarioFile(base) {
			named = append(named, path)
			return nil
		}
		if filepath.Dir(path) == root && isScenarioExt(base) {
			topLevel = append(topLevel, path)
		}
		_ = rel
		return nil
	})
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, p := range named {
		add(p)
	}
	for _, p := range topLevel {
		sc, err := parseScenarioFile(p)
		if err != nil || !looksLikeScenario(sc) {
			continue
		}
		add(p)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, os.ErrNotExist
	}
	return out, nil
}

func skipWalkName(name string) bool {
	if name == "_shared" {
		return true
	}
	return strings.HasPrefix(name, ".")
}

func isNamedScenarioFile(name string) bool {
	return name == "scenario.yaml" || name == "scenario.yml" || name == "scenario.json"
}

func isScenarioExt(name string) bool {
	switch filepath.Ext(name) {
	case ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}
