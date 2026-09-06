package registry

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"gust/internal/ports"
	"gust/pkg/api"
)

type mockEvaluator struct {
	name string
}

func (m *mockEvaluator) Name() string    { return m.name }
func (m *mockEvaluator) Version() string { return "1.0" }
func (m *mockEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	return ports.EvaluationResult{Passed: true}, nil
}

func TestRegistryOperations(t *testing.T) {
	reg := New()

	e := &mockEvaluator{name: "test_eval"}
	if err := reg.RegisterEvaluator(e); err != nil {
		t.Fatalf("RegisterEvaluator failed: %v", err)
	}

	// Duplicate should fail
	if err := reg.RegisterEvaluator(e); err == nil {
		t.Errorf("expected error on duplicate registration, got nil")
	}

	// Get
	found, ok := reg.GetEvaluator("test_eval")
	if !ok || found.Name() != "test_eval" {
		t.Errorf("failed to get registered evaluator")
	}

	// List
	all := reg.ListEvaluators()
	if len(all) != 1 {
		t.Errorf("expected 1 evaluator, got %d", len(all))
	}
}

func TestRegistryConcurrency(t *testing.T) {
	reg := New()
	var wg sync.WaitGroup
	workers := 50

	// Concurrent register
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			e := &mockEvaluator{name: fmt.Sprintf("eval_%d", id)}
			_ = reg.RegisterEvaluator(e)
		}(i)
	}

	// Concurrent read
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, _ = reg.GetEvaluator(fmt.Sprintf("eval_%d", id))
			_ = reg.ListEvaluators()
		}(i)
	}

	wg.Wait()
}
