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

// RunStore persists evaluation artifacts — mapped AgentRun traces and, later,
// verdicts — not raw OTLP spans. A SQLite/Postgres adapter can sit behind this
// port for historical eval data. This is not an observability backend; keep
// Phoenix or Langfuse for span search and live tail. Unused by the CLI until
// a --store flag is added; Receiver.OnRun is the ingest hook that will call
// SaveRun.
type RunStore interface {
	SaveRun(ctx context.Context, run api.AgentRun) error
	GetRun(ctx context.Context, id string) (api.AgentRun, error)
	ListRuns(ctx context.Context) ([]api.AgentRun, error)
	DeleteRun(ctx context.Context, id string) error
}
