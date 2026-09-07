package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gust/internal/ports"
	"gust/pkg/api"
)

// Compile-time interface checks.
var (
	_ ports.ScenarioStore = (*FileStore)(nil)
	_ ports.FixtureStore  = (*FileStore)(nil)
	_ ports.RunStore      = (*FileStore)(nil)
)

// FileStore implements ScenarioStore, FixtureStore, and RunStore using local filesystem JSON files.
// Writes are intentionally mutable: re-saving the same ID overwrites. This is a local
// results/run cache, not the content-addressed immutability contract of dataset.Bundle.
type FileStore struct {
	baseDir      string
	scenariosDir string
	fixturesDir  string
	runsDir      string
	mu           sync.RWMutex
}

// NewFileStore creates a new filesystem-backed store initialized at rootDir.
func NewFileStore(rootDir string) (*FileStore, error) {
	s := &FileStore{
		baseDir:      rootDir,
		scenariosDir: filepath.Join(rootDir, "scenarios"),
		fixturesDir:  filepath.Join(rootDir, "fixtures"),
		runsDir:      filepath.Join(rootDir, "runs"),
	}

	for _, dir := range []string{s.scenariosDir, s.fixturesDir, s.runsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return s, nil
}

// --- ScenarioStore Implementation ---

func (s *FileStore) SaveScenario(ctx context.Context, scenario api.TestScenario) error {
	if err := scenario.Validate(); err != nil {
		return fmt.Errorf("scenario validation failed: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.scenariosDir, scenario.ID+".json")
	data, err := json.MarshalIndent(scenario, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal scenario: %w", err)
	}
	return os.WriteFile(filePath, data, 0644)
}

func (s *FileStore) GetScenario(ctx context.Context, id string) (api.TestScenario, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.scenariosDir, id+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return api.TestScenario{}, ports.ErrNotFound
		}
		return api.TestScenario{}, err
	}

	var scenario api.TestScenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		return api.TestScenario{}, fmt.Errorf("failed to decode scenario: %w", err)
	}
	return scenario, nil
}

func (s *FileStore) ListScenarios(ctx context.Context) ([]api.TestScenario, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.scenariosDir)
	if err != nil {
		return nil, err
	}

	var list []api.TestScenario
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.scenariosDir, entry.Name()))
		if err != nil {
			continue
		}
		var sc api.TestScenario
		if err := json.Unmarshal(data, &sc); err == nil {
			list = append(list, sc)
		}
	}
	return list, nil
}

func (s *FileStore) DeleteScenario(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.scenariosDir, id+".json")
	err := os.Remove(filePath)
	if err != nil && os.IsNotExist(err) {
		return ports.ErrNotFound
	}
	return err
}

// --- FixtureStore Implementation ---

func (s *FileStore) SaveFixture(ctx context.Context, fixture api.Fixture) error {
	if err := fixture.Validate(); err != nil {
		return fmt.Errorf("fixture validation failed: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.fixturesDir, fixture.FixtureID+".json")
	data, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal fixture: %w", err)
	}
	return os.WriteFile(filePath, data, 0644)
}

func (s *FileStore) GetFixture(ctx context.Context, id string) (api.Fixture, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.fixturesDir, id+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return api.Fixture{}, ports.ErrNotFound
		}
		return api.Fixture{}, err
	}

	var fx api.Fixture
	if err := json.Unmarshal(data, &fx); err != nil {
		return api.Fixture{}, fmt.Errorf("failed to decode fixture: %w", err)
	}
	return fx, nil
}

func (s *FileStore) FindByHash(ctx context.Context, tool, inputHash string) (api.Fixture, bool, error) {
	fixtures, err := s.ListByTool(ctx, tool)
	if err != nil {
		return api.Fixture{}, false, err
	}
	for _, fx := range fixtures {
		if fx.InputHash == inputHash {
			return fx, true, nil
		}
	}
	return api.Fixture{}, false, nil
}

func (s *FileStore) ListByTool(ctx context.Context, tool string) ([]api.Fixture, error) {
	all, err := s.ListAllFixtures(ctx)
	if err != nil {
		return nil, err
	}
	var filtered []api.Fixture
	for _, fx := range all {
		if fx.Tool == tool {
			filtered = append(filtered, fx)
		}
	}
	return filtered, nil
}

func (s *FileStore) ListAllFixtures(ctx context.Context) ([]api.Fixture, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.fixturesDir)
	if err != nil {
		return nil, err
	}

	var list []api.Fixture
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.fixturesDir, entry.Name()))
		if err != nil {
			continue
		}
		var fx api.Fixture
		if err := json.Unmarshal(data, &fx); err == nil {
			list = append(list, fx)
		}
	}
	return list, nil
}

func (s *FileStore) DeleteFixture(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.fixturesDir, id+".json")
	err := os.Remove(filePath)
	if err != nil && os.IsNotExist(err) {
		return ports.ErrNotFound
	}
	return err
}

// --- RunStore Implementation ---

func (s *FileStore) SaveRun(ctx context.Context, run api.AgentRun) error {
	if err := run.Validate(); err != nil {
		return fmt.Errorf("agent run validation failed: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.runsDir, run.RunID+".json")
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal agent run: %w", err)
	}
	return os.WriteFile(filePath, data, 0644)
}

func (s *FileStore) GetRun(ctx context.Context, id string) (api.AgentRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.runsDir, id+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return api.AgentRun{}, ports.ErrNotFound
		}
		return api.AgentRun{}, err
	}

	var run api.AgentRun
	if err := json.Unmarshal(data, &run); err != nil {
		return api.AgentRun{}, fmt.Errorf("failed to decode agent run: %w", err)
	}
	return run, nil
}

func (s *FileStore) ListRuns(ctx context.Context) ([]api.AgentRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.runsDir)
	if err != nil {
		return nil, err
	}

	var list []api.AgentRun
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.runsDir, entry.Name()))
		if err != nil {
			continue
		}
		var run api.AgentRun
		if err := json.Unmarshal(data, &run); err == nil {
			list = append(list, run)
		}
	}
	return list, nil
}

func (s *FileStore) DeleteRun(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.runsDir, id+".json")
	err := os.Remove(filePath)
	if err != nil && os.IsNotExist(err) {
		return ports.ErrNotFound
	}
	return err
}
