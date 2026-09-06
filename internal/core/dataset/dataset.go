package dataset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gust/pkg/api"
	"gust/pkg/jcs"
)

// Manifest is a content-addressed dataset description.
type Manifest struct {
	ID           string            `json:"id"`
	CreatedAt    time.Time         `json:"created_at"`
	ScenarioIDs  []string          `json:"scenario_ids"`
	ScenarioHash map[string]string `json:"scenario_hashes"`
	ContentHash  string            `json:"content_hash"`
}

// Bundle writes scenarios as JSON files and a dataset.json manifest with JCS hashes.
func Bundle(dir, datasetID string, scenarios []api.TestScenario) (*Manifest, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	m := &Manifest{
		ID:           datasetID,
		CreatedAt:    time.Now().UTC(),
		ScenarioHash: make(map[string]string),
	}
	for _, sc := range scenarios {
		hash, err := jcs.ContentHash(sc)
		if err != nil {
			return nil, fmt.Errorf("hash scenario %s: %w", sc.ID, err)
		}
		m.ScenarioIDs = append(m.ScenarioIDs, sc.ID)
		m.ScenarioHash[sc.ID] = hash

		path := filepath.Join(dir, sc.ID+".json")
		data, err := json.MarshalIndent(sc, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return nil, err
		}
	}
	sort.Strings(m.ScenarioIDs)
	contentHash, err := jcs.ContentHash(map[string]any{
		"scenario_ids":    m.ScenarioIDs,
		"scenario_hashes": m.ScenarioHash,
	})
	if err != nil {
		return nil, err
	}
	m.ContentHash = contentHash

	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "dataset.json"), raw, 0644); err != nil {
		return nil, err
	}
	return m, nil
}

// Verify checks that on-disk scenarios still match the manifest hashes.
func Verify(dir string) error {
	raw, err := os.ReadFile(filepath.Join(dir, "dataset.json"))
	if err != nil {
		return err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	for id, want := range m.ScenarioHash {
		data, err := os.ReadFile(filepath.Join(dir, id+".json"))
		if err != nil {
			return fmt.Errorf("missing scenario %s: %w", id, err)
		}
		var sc api.TestScenario
		if err := json.Unmarshal(data, &sc); err != nil {
			return err
		}
		got, err := jcs.ContentHash(sc)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("checksum mismatch for scenario %s: want %s got %s", id, want, got)
		}
	}
	return nil
}
