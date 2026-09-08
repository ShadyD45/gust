package report_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gust/internal/adapters/report"
	"gust/internal/core/policy"
	"gust/pkg/api"
)

func TestWriteHTML_SummaryAndFailures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gust-report.html")
	err := report.WriteHTML(path, report.SuiteReport{
		EvaluationID: "eval-1",
		GeneratedAt:  time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC),
		PolicyName:   "ci",
		Overall: policy.Verdict{
			OverallVerdict: api.VerdictFail,
			ExitCode:       api.ExitFailure,
			Violations: []policy.PolicyViolation{{
				Severity: "fatal",
				Message:  "scenario demo reliability verdict FAIL",
			}},
		},
		Results: []*api.ReliabilityResult{{
			EvaluationID:         "eval-1",
			ScenarioID:           "demo",
			Samples:              5,
			SamplesRequested:     5,
			SamplesCompleted:     4,
			Passes:               3,
			BehavioralFailures:   1,
			InfrastructureErrors: 1,
			ObservedPassRate:     0.75,
			ConfidenceInterval:   [2]float64{0.3, 0.95},
			Verdict:              api.VerdictFail,
			SampleResults: []api.SampleResult{
				{
					SampleID: "demo-ok",
					Status:   api.SampleStatusPassed,
					Passed:   true,
					Evaluations: []api.EvaluationResult{{
						EvaluatorName: "task_success",
						Passed:        true,
						Message:       "task completed successfully",
					}},
				},
				{
					SampleID:        "demo-fail",
					Status:          api.SampleStatusFailed,
					FailureCategory: api.FailureAssertion,
					Message:         "tool missing",
					Passed:          false,
					Evaluations: []api.EvaluationResult{{
						EvaluatorName: "required_tool",
						Passed:        false,
						Message:       "required tool lookup missing",
						Evidence:      map[string]any{"tool": "lookup"},
					}},
				},
				{
					SampleID:        "demo-infra",
					Status:          api.SampleStatusInfraError,
					FailureCategory: api.FailureTraceIngest,
					Message:         "timed out waiting for sample",
				},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(raw)
	for _, want := range []string{
		"Gust reliability report",
		"eval-1",
		"demo-fail",
		"required tool lookup missing",
		"demo-ok",
		"data-filter=\"fail\"",
		"data-action=\"expand\"",
		"✗",
		"✓",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("report missing %q", want)
		}
	}
	if strings.Contains(html, "<script src=") {
		t.Fatal("report must not embed remote scripts")
	}
	if strings.Contains(html, "<link rel=") {
		t.Fatal("report must not load remote stylesheets")
	}
}

func TestRenderHTML_EscapesContent(t *testing.T) {
	html, err := report.RenderHTML(report.SuiteReport{
		GeneratedAt: time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC),
		Overall: policy.Verdict{
			OverallVerdict: api.VerdictFail,
			ExitCode:       1,
			Violations: []policy.PolicyViolation{{
				Severity: "fatal",
				Message:  "<script>alert(1)</script>",
			}},
		},
		Results: []*api.ReliabilityResult{{
			ScenarioID: "esc",
			Verdict:    api.VerdictFail,
			SampleResults: []api.SampleResult{{
				SampleID: "s1",
				Status:   api.SampleStatusFailed,
				Passed:   false,
				Message:  "<b>bad</b>",
				Evaluations: []api.EvaluationResult{{
					EvaluatorName: "x",
					Passed:        false,
					Message:       "saw <img>",
					Evidence:      map[string]any{"raw": "<script>"},
				}},
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<script>alert") {
		t.Fatal("unescaped script in violations")
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatal("expected escaped violation text")
	}
	if strings.Contains(html, "<b>bad</b>") {
		t.Fatal("unescaped message html")
	}
}
