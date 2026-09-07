package evaluators

import (
	"context"
	"encoding/json"
	"testing"

	"gust/internal/ports"
	"gust/pkg/api"
)

func FuzzTaskSuccessEvaluate(f *testing.F) {
	seed, _ := json.Marshal(api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "fuzz",
		Agent:         api.AgentInfo{Name: "a", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "x"},
		Outcome:       api.RunOutcome{Status: "completed", Output: "ok"},
	})
	f.Add(seed)
	f.Fuzz(func(t *testing.T, data []byte) {
		var run api.AgentRun
		if json.Unmarshal(data, &run) != nil {
			return
		}
		eval := &TaskSuccessEvaluator{}
		_, _ = eval.Evaluate(context.Background(), run, &api.Assertion{ID: "a", Type: api.AssertTaskSuccess}, ports.EvaluationContext{})
	})
}
