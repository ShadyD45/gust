package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"gust/pkg/api"
)

// SampleRuns loads AgentRun JSON files from a directory (non-recursive).
// Used by continuous-eval batch sampling.
func SampleRuns(dir string, limit int) ([]api.AgentRun, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var runs []api.AgentRun
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var run api.AgentRun
		if err := json.Unmarshal(raw, &run); err != nil {
			return nil, err
		}
		if err := run.Validate(); err != nil {
			continue // skip malformed staging dumps
		}
		runs = append(runs, run)
		if limit > 0 && len(runs) >= limit {
			break
		}
	}
	return runs, nil
}
