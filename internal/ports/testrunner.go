package ports

import (
	"context"
	"gust/pkg/api"
)

// TestRunner executes a live agent against a controlled fixture environment.
type TestRunner interface {
	Name() string
	Run(ctx context.Context, scenario api.TestScenario, fixtureEndpoint string) (api.AgentRun, error)
}
