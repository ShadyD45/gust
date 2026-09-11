package cli

import (
	"fmt"
	"os"
	"strings"

	"gust/internal/core/analyze"
	"gust/internal/core/mutate"
	"gust/internal/core/policy"
	"gust/internal/domain/evidence"
	"gust/pkg/api"
)

// PrintAnalyzeTerminal renders Mode 1 results.
func PrintAnalyzeTerminal(report *analyze.AnalysisReport) {
	badge := "PASS"
	if !report.Passed {
		badge = "FAIL"
	}
	fmt.Printf("Analyze %s → %s (%d assertions, %d ns)\n", report.RunID, badge, len(report.Results), report.TotalDurationNs)
	for _, r := range report.Results {
		mark := "✓"
		if !r.Passed {
			mark = "✗"
		}
		fmt.Printf("  %s %s: %s\n", mark, r.EvaluatorName, r.Message)
		printEvidenceDiffs(r.Evidence)
	}
}

// PrintReplayTerminal renders Mode 2 output summary.
func PrintReplayTerminal(run *api.AgentRun) {
	fmt.Printf("Replay → %s (%d spans, outcome=%s)\n", run.RunID, len(run.Trace), run.Outcome.Status)
}

// PrintReliabilityTerminal renders Mode 3 with distinct FLAKY badge.
func PrintReliabilityTerminal(res *api.ReliabilityResult) {
	completed := res.SamplesCompleted
	if completed == 0 && res.ExecutionErrors == 0 {
		completed = res.Samples
	}
	requested := res.SamplesRequested
	if requested == 0 {
		requested = res.Samples
	}
	fmt.Printf("\nScenario: %s\n", res.ScenarioID)
	if res.EvaluationID != "" {
		fmt.Printf("  evaluation: %s\n", res.EvaluationID)
	}
	infra := res.InfrastructureErrors
	if infra == 0 {
		infra = res.ExecutionErrors
	}
	fmt.Printf("  samples requested: %d\n", requested)
	fmt.Printf("  agent passes: %d\n", res.Passes)
	fmt.Printf("  agent failures: %d\n", res.BehavioralFailures)
	fmt.Printf("  execution errors: %d\n", infra)
	fmt.Printf("  observed reliability: %d/%d = %.1f%%\n",
		res.Passes, completed, res.ObservedPassRate*100)
	fmt.Printf("  95%% confidence interval: [%.1f%%, %.1f%%]\n",
		res.ConfidenceInterval[0]*100, res.ConfidenceInterval[1]*100)
	if infra > 0 {
		fmt.Printf("  note: execution errors are excluded from the Wilson denominator\n")
	}

	switch res.Verdict {
	case api.VerdictPass:
		fmt.Printf("\nVERDICT: [+] PASS\n")
	case api.VerdictFail:
		fmt.Printf("\nVERDICT: [-] FAIL\n")
	case api.VerdictFlaky:
		fmt.Printf("\nVERDICT: [?] FLAKY (Inconclusive)\n")
	default:
		fmt.Printf("\nVERDICT: %s\n", res.Verdict)
	}
}

// PrintSuiteVerdict renders the aggregated policy decision after a directory run.
func PrintSuiteVerdict(n int, v policy.Verdict) {
	fmt.Printf("\nSuite: %d scenarios → %s (exit %d)\n", n, v.OverallVerdict, v.ExitCode)
	for _, viol := range v.Violations {
		fmt.Printf("  - %s: %s\n", viol.Severity, viol.Message)
	}
}

// PrintMutateTerminal renders mutation benchmark with an explicit mutation score.
func PrintMutateTerminal(report *mutate.MutationBenchmarkReport) {
	escaped := report.MutantsApplied - report.MutantsDetected
	if escaped < 0 {
		escaped = 0
	}
	fmt.Printf("Mutation testing\n")
	fmt.Printf("  Mutants: %d | Detected: %d | Escaped: %d | Skipped: %d\n",
		report.MutantsApplied, report.MutantsDetected, escaped, report.MutantsSkipped)
	fmt.Printf("  Evaluator mutation score: %.0f%%\n", report.DetectionRate*100)
	fmt.Printf("  FPR: %.1f%%\n", report.FalsePositiveRate*100)
}

// PrintRegressionTerminal renders compare output.
func PrintRegressionTerminal(reg *policy.RegressionResult) {
	status := "PASS"
	if reg.Regressed {
		status = "REGRESSION"
	}
	fmt.Printf("Compare → %s\n", status)
	if reg.BaselineSamples > 0 || reg.CandidateSamples > 0 {
		fmt.Printf("  Baseline:  %d/%d = %.1f%%\n",
			reg.BaselinePasses, reg.BaselineSamples, reg.BaselinePassRate*100)
		fmt.Printf("  Candidate: %d/%d = %.1f%%\n",
			reg.CandidatePasses, reg.CandidateSamples, reg.CandidatePassRate*100)
		fmt.Printf("  Delta:     %+.1f pp (policy max_drop=%.1f pp)\n",
			-reg.PassRateDrop*100, reg.MaxPassRateDrop*100)
	}
	lat := "n/a"
	if reg.LatencyIncreaseRatio != nil {
		lat = fmt.Sprintf("%.4f", *reg.LatencyIncreaseRatio)
	}
	fmt.Printf("  Significance: p=%.4f significant=%v\n", reg.PValue, reg.Significant)
	fmt.Printf("  Effect size:  Cohen's d=%.3f (%s)\n", reg.CohensD, reg.EffectSize)
	if reg.BootstrapReplicates > 0 {
		fmt.Printf("  Bootstrap Δ CI [%d reps]: [%.4f, %.4f]\n",
			reg.BootstrapReplicates, reg.BootstrapDiffLower, reg.BootstrapDiffUpper)
	}
	fmt.Printf("  Latency increase ratio: %s\n", lat)
	fmt.Printf("  Verdict: %s\n", status)
	fmt.Printf("  %s\n", reg.Message)
}

// WriteGitHubSummary appends a markdown table when GITHUB_STEP_SUMMARY is set.
func WriteGitHubSummary(results []*api.ReliabilityResult) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" || len(results) == 0 {
		return
	}
	var b strings.Builder
	b.WriteString("## gust Test Results\n\n")
	b.WriteString("| Scenario | Samples | Pass Rate | 95% CI | Verdict |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, r := range results {
		badge := string(r.Verdict)
		switch r.Verdict {
		case api.VerdictFlaky:
			badge = "⚠️ **FLAKY**"
		case api.VerdictPass:
			badge = "✅ **PASS**"
		case api.VerdictFail:
			badge = "❌ **FAIL**"
		}
		fmt.Fprintf(&b, "| `%s` | %d | %.1f%% | `[%.1f%%, %.1f%%]` | %s |\n",
			r.ScenarioID, r.Samples, r.ObservedPassRate*100,
			r.ConfidenceInterval[0]*100, r.ConfidenceInterval[1]*100, badge)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(b.String())
}

func printEvidenceDiffs(ev map[string]any) {
	if ev == nil {
		return
	}
	if raw, ok := ev["diffs"]; ok {
		printDiffValue("    ", raw)
	}
	if raw, ok := ev["discrepancies"]; ok {
		printDiffValue("    ", raw)
	}
}

func printDiffValue(indent string, raw any) {
	switch v := raw.(type) {
	case []evidence.DiffItem:
		fmt.Print(evidence.FormatDiffs(v))
	case evidence.DiffItem:
		fmt.Printf("%s%s: expected %v, actual %v\n", indent, v.Path, v.Expected, v.Actual)
	case []any:
		for _, item := range v {
			printDiffValue(indent, item)
		}
	case map[string]any:
		if diffs, ok := v["diffs"]; ok {
			if sid, ok := v["span_id"]; ok {
				fmt.Printf("%sspan %v:\n", indent, sid)
			}
			printDiffValue(indent+"  ", diffs)
			return
		}
		if path, ok := v["path"].(string); ok {
			fmt.Printf("%s%s: expected %v, actual %v\n", indent, path, v["expected"], v["actual"])
		}
	}
}
