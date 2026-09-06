package ports

import (
	"context"
	"errors"

	"gust/pkg/api"
)

var (
	ErrNotFound = errors.New("entity not found in store")
	ErrConflict = errors.New("entity already exists in store")
)

// ScenarioStore defines persistence operations for TestScenario artifacts.
type ScenarioStore interface {
	SaveScenario(ctx context.Context, scenario api.TestScenario) error
	GetScenario(ctx context.Context, id string) (api.TestScenario, error)
	ListScenarios(ctx context.Context) ([]api.TestScenario, error)
	DeleteScenario(ctx context.Context, id string) error
}

// FixtureStore defines persistence operations for Fixture mocks.
type FixtureStore interface {
	SaveFixture(ctx context.Context, fixture api.Fixture) error
	GetFixture(ctx context.Context, id string) (api.Fixture, error)
	FindByHash(ctx context.Context, tool, inputHash string) (api.Fixture, bool, error)
	ListByTool(ctx context.Context, tool string) ([]api.Fixture, error)
	ListAllFixtures(ctx context.Context) ([]api.Fixture, error)
	DeleteFixture(ctx context.Context, id string) error
}

// RunStore defines persistence operations for captured AgentRun execution traces.
type RunStore interface {
	SaveRun(ctx context.Context, run api.AgentRun) error
	GetRun(ctx context.Context, id string) (api.AgentRun, error)
	ListRuns(ctx context.Context) ([]api.AgentRun, error)
	DeleteRun(ctx context.Context, id string) error
}
