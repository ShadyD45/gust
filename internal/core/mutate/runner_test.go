package mutate

import (
	"context"
	"testing"

	"gust/internal/adapters/evaluators"
	"gust/internal/adapters/mutators"
	"gust/internal/ports"
)

func TestMutationBenchmarkTargetMetrics(t *testing.T) {
	evalList := []ports.Evaluator{
		&evaluators.TaskSuccessEvaluator{},
		&evaluators.ToolSelectionEvaluator{},
		&evaluators.ToolArgumentsEvaluator{},
		&evaluators.ToolSequenceEvaluator{},
		&evaluators.ForbiddenToolEvaluator{},
		&evaluators.RequiredToolEvaluator{},
		&evaluators.MaxStepsEvaluator{},
		&evaluators.MaxLatencyEvaluator{},
		&evaluators.ErrorRecoveryEvaluator{},
		&evaluators.SchemaValidationEvaluator{},
	}

	mutList := mutators.AllBuiltinMutators()
	runner := NewRunner(mutList, evalList)
	goldenSuite := BuildGoldenSuite()

	report, err := runner.RunBenchmark(context.Background(), goldenSuite)
	if err != nil {
		t.Fatalf("RunBenchmark failed: %v", err)
	}

	t.Logf("Generated %d mutants, %d applied, %d detected", report.TotalMutantsGenerated, report.MutantsApplied, report.MutantsDetected)
	t.Logf("Detection Rate: %.2f%%", report.DetectionRate*100)
	t.Logf("False Positive Rate: %.2f%%", report.FalsePositiveRate*100)

	for name, stats := range report.ClassBreakdown {
		t.Logf("  [%s]: %d/%d detected (%.1f%%)", name, stats.Detected, stats.Applied, stats.Rate*100)
	}

	if report.FalsePositiveRate > 0.05 {
		t.Errorf("False Positive Rate exceeded 5%% threshold: %.2f%%", report.FalsePositiveRate*100)
	}

	if report.DetectionRate < 0.90 {
		t.Errorf("Detection Rate below 90%% target: %.2f%%", report.DetectionRate*100)
	}
}
