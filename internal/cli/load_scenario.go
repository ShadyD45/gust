package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"gust/pkg/api"
)

// loadScenario parses a scenario file and resolves fixtures_dir, assertion_files, and $ref.
func loadScenario(path string) (api.TestScenario, error) {
	sc, err := parseScenarioFile(path)
	if err != nil {
		return api.TestScenario{}, err
	}
	return resolveScenario(sc, filepath.Dir(path))
}

func parseScenarioFile(path string) (api.TestScenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return api.TestScenario{}, err
	}
	var sc api.TestScenario
	if err := yaml.Unmarshal(data, &sc); err != nil {
		if err2 := json.Unmarshal(data, &sc); err2 != nil {
			return api.TestScenario{}, fmt.Errorf("parse scenario %s: %v / %v", path, err, err2)
		}
	}
	return sc, nil
}

func resolveScenario(sc api.TestScenario, baseDir string) (api.TestScenario, error) {
	fxDir := sc.Environment.FixturesDir
	if fxDir == "" {
		sibling := filepath.Join(baseDir, "fixtures")
		if info, err := os.Stat(sibling); err == nil && info.IsDir() {
			fxDir = "fixtures"
		}
	}
	if fxDir != "" {
		abs := fxDir
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(baseDir, fxDir)
		}
		fromDir, err := loadFixturesDir(abs)
		if err != nil {
			return api.TestScenario{}, fmt.Errorf("fixtures_dir %s: %w", fxDir, err)
		}
		sc.Environment.Fixtures = append(sc.Environment.Fixtures, fromDir...)
	}

	var merged []api.Assertion
	for _, ref := range sc.AssertionFiles {
		pack, err := loadAssertionFile(resolveRel(baseDir, ref))
		if err != nil {
			return api.TestScenario{}, err
		}
		merged = append(merged, pack...)
	}
	for _, a := range sc.Assertions {
		if strings.TrimSpace(a.Ref) != "" {
			pack, err := loadAssertionFile(resolveRel(baseDir, a.Ref))
			if err != nil {
				return api.TestScenario{}, err
			}
			merged = append(merged, pack...)
			continue
		}
		merged = append(merged, a)
	}
	if err := uniqueAssertionIDs(merged); err != nil {
		return api.TestScenario{}, err
	}
	for i := range merged {
		merged[i].Ref = ""
	}
	sc.Assertions = merged
	sc.AssertionFiles = nil
	return sc, nil
}

func loadAssertionFile(path string) ([]api.Assertion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("assertion file %s: %w", path, err)
	}
	var pack []api.Assertion
	if err := yaml.Unmarshal(data, &pack); err != nil {
		if err2 := json.Unmarshal(data, &pack); err2 != nil {
			return nil, fmt.Errorf("parse assertions %s: %v / %v", path, err, err2)
		}
	}
	if pack == nil {
		return nil, fmt.Errorf("assertion file %s: expected an array", path)
	}
	return pack, nil
}

func uniqueAssertionIDs(assertions []api.Assertion) error {
	seen := map[string]struct{}{}
	for _, a := range assertions {
		if a.ID == "" {
			continue
		}
		if _, ok := seen[a.ID]; ok {
			return fmt.Errorf("duplicate assertion id %q after resolving refs", a.ID)
		}
		seen[a.ID] = struct{}{}
	}
	return nil
}

func resolveRel(baseDir, ref string) string {
	if filepath.IsAbs(ref) {
		return ref
	}
	return filepath.Join(baseDir, filepath.FromSlash(ref))
}

func looksLikeScenario(sc api.TestScenario) bool {
	return sc.ID != "" && sc.Task.Input != ""
}
