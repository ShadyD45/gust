package cli

import (
	"os"
	"path/filepath"

	"gust/internal/adapters/fixtures"
	"gust/pkg/api"
)

func loadFixturesDir(dir string) ([]api.Fixture, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var loaded []api.Fixture
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		fx, err := loadJSON[api.Fixture](filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, fx)
	}
	return loaded, nil
}

func newFixtureProvider(scenarioFixtures []api.Fixture, fixturesDir string) (*fixtures.MemoryFixtureProvider, error) {
	provider := fixtures.NewMemoryFixtureProvider()
	if len(scenarioFixtures) > 0 {
		if err := provider.LoadFixtures(scenarioFixtures); err != nil {
			return nil, err
		}
	}
	fromDir, err := loadFixturesDir(fixturesDir)
	if err != nil {
		return nil, err
	}
	if len(fromDir) > 0 {
		if err := provider.LoadFixtures(fromDir); err != nil {
			return nil, err
		}
	}
	return provider, nil
}
