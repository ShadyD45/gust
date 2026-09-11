package aee

import (
	"context"
	"fmt"
	"time"

	"gust/internal/adapters/evaluators"
	"gust/internal/adapters/mutators"
	"gust/internal/core/analyze"
	"gust/internal/core/mutate"
	"gust/internal/core/stats"
	"gust/internal/ports"
	"gust/pkg/api"
	"gust/pkg/jcs"
)

// Thresholds match spec §23 / §24 and MVP DoD.
const (
	MinDetectionRate  = 0.90
	MaxFalsePositive  = 0.05
	MinEvalThroughput = 1000.0 // deterministic evaluator calls/sec
	MinReproIdentical = 1.0    // fraction of identical hashes
	ReproTrials       = 20
	ThroughputTrials  = 5000
)

// Report is the Agent Evaluation Effectiveness self-score for gust.
type Report struct {
	GeneratedAt time.Time `json:"generated_at"`

	DetectionRate     float64 `json:"detection_rate"`
	FalsePositiveRate float64 `json:"false_positive_rate"`
	MutantsApplied    int     `json:"mutants_applied"`
	MutantsDetected   int     `json:"mutants_detected"`

	EvalThroughputCPS float64 `json:"eval_throughput_cps"`
	AnalyzeDurationNs int64   `json:"analyze_duration_ns"`

	Reproducibility float64 `json:"reproducibility"`
	ReproHash       string  `json:"repro_hash,omitempty"`

	H7 struct {
		PassCase   string `json:"pass_case"`
		FlakyCase  string `json:"flaky_case"`
		FailCase   string `json:"fail_case"`
		AllCorrect bool   `json:"all_correct"`
	} `json:"h7"`

	// Hardening holds issue #4 extras (informational; not part of the composite gate).
	Hardening *HardeningExtras `json:"hardening,omitempty"`

	// Score is a simple documented composite in [0,1].
	Score  float64  `json:"aee_score"`
	Passed bool     `json:"passed"`
	Notes  []string `json:"notes,omitempty"`
}

// Run executes offline self-benchmarks (no network).
func Run(ctx context.Context) (*Report, error) {
	rep := &Report{GeneratedAt: time.Now().UTC()}

	detEvals := deterministicEvaluators()
	mutRunner := mutate.NewRunner(mutators.AllBuiltinMutators(), detEvals)
	mutReport, err := mutRunner.RunBenchmark(ctx, mutate.BuildGoldenSuite())
	if err != nil {
		return nil, fmt.Errorf("mutation benchmark: %w", err)
	}
	rep.DetectionRate = mutReport.DetectionRate
	rep.FalsePositiveRate = mutReport.FalsePositiveRate
	rep.MutantsApplied = mutReport.MutantsApplied
	rep.MutantsDetected = mutReport.MutantsDetected

	if err := measureThroughput(ctx, rep); err != nil {
		return nil, err
	}
	if err := measureRepro(ctx, rep); err != nil {
		return nil, err
	}
	measureH7(rep)
	if err := rep.attachHardening(ctx, mutReport.ClassBreakdown); err != nil {
		return nil, err
	}

	rep.Score = compositeScore(rep)
	rep.Passed = rep.DetectionRate >= MinDetectionRate &&
		rep.FalsePositiveRate <= MaxFalsePositive &&
		rep.EvalThroughputCPS >= MinEvalThroughput &&
		rep.Reproducibility >= MinReproIdentical &&
		rep.H7.AllCorrect

	if !rep.Passed {
		if rep.DetectionRate < MinDetectionRate {
			rep.Notes = append(rep.Notes, fmt.Sprintf("detection_rate %.3f < %.2f", rep.DetectionRate, MinDetectionRate))
		}
		if rep.FalsePositiveRate > MaxFalsePositive {
			rep.Notes = append(rep.Notes, fmt.Sprintf("false_positive_rate %.3f > %.2f", rep.FalsePositiveRate, MaxFalsePositive))
		}
		if rep.EvalThroughputCPS < MinEvalThroughput {
			rep.Notes = append(rep.Notes, fmt.Sprintf("throughput %.0f < %.0f cps", rep.EvalThroughputCPS, MinEvalThroughput))
		}
		if rep.Reproducibility < MinReproIdentical {
			rep.Notes = append(rep.Notes, fmt.Sprintf("reproducibility %.3f < 1", rep.Reproducibility))
		}
		if !rep.H7.AllCorrect {
			rep.Notes = append(rep.Notes, "H7 reliability vectors misclassified")
		}
	}
	return rep, nil
}

func deterministicEvaluators() []ports.Evaluator {
	all := evaluators.AllBuiltinEvaluators()
	out := make([]ports.Evaluator, 0, len(all))
	for _, e := range all {
		switch e.Name() {
		case "llm_judge", "judge_panel":
			continue
		default:
			out = append(out, e)
		}
	}
	return out
}

func measureThroughput(ctx context.Context, rep *Report) error {
	evals := deterministicEvaluators()
	run := mutate.BuildGoldenSuite()[0].Run
	assert := &api.Assertion{ID: "a", Type: api.AssertTaskSuccess}

	const minWall = 50 * time.Millisecond
	n := 0
	start := time.Now()
	for {
		e := evals[n%len(evals)]
		if _, err := e.Evaluate(ctx, run, assert, ports.EvaluationContext{}); err != nil {
			return fmt.Errorf("throughput evaluate: %w", err)
		}
		n++
		if n >= ThroughputTrials && time.Since(start) >= minWall {
			break
		}
		if n > ThroughputTrials*100 {
			break
		}
	}
	elapsed := time.Since(start)
	if elapsed <= 0 {
		elapsed = time.Millisecond
	}
	rep.EvalThroughputCPS = float64(n) / elapsed.Seconds()

	engine := analyze.NewEngine(evals)
	aStart := time.Now()
	_, err := engine.AnalyzeRun(ctx, run, mutate.BuildGoldenSuite()[0].Assertions, ports.EvaluationContext{})
	rep.AnalyzeDurationNs = time.Since(aStart).Nanoseconds()
	return err
}

func measureRepro(ctx context.Context, rep *Report) error {
	evals := deterministicEvaluators()
	engine := analyze.NewEngine(evals)
	gc := mutate.BuildGoldenSuite()[0]
	var first string
	ok := 0
	for i := 0; i < ReproTrials; i++ {
		report, err := engine.AnalyzeRun(ctx, gc.Run, gc.Assertions, ports.EvaluationContext{})
		if err != nil {
			return err
		}
		stable := stableAnalyzeView(report)
		hash, err := jcs.ContentHash(stable)
		if err != nil {
			return err
		}
		if i == 0 {
			first = hash
			rep.ReproHash = hash
		}
		if hash == first {
			ok++
		}
	}
	rep.Reproducibility = float64(ok) / float64(ReproTrials)
	return nil
}

func stableAnalyzeView(report *analyze.AnalysisReport) map[string]any {
	results := make([]map[string]any, 0, len(report.Results))
	for _, r := range report.Results {
		results = append(results, map[string]any{
			"evaluator_name": r.EvaluatorName,
			"passed":         r.Passed,
			"score":          r.Score,
			"message":        r.Message,
		})
	}
	return map[string]any{
		"run_id":  report.RunID,
		"passed":  report.Passed,
		"results": results,
	}
}

func measureH7(rep *Report) {
	const pMin = 0.95
	const conf = 0.95
	const minSamples = 5

	check := func(passes, total int) api.VerdictType {
		iv, err := stats.CalculateWilsonScore(passes, total, conf)
		if err != nil {
			return ""
		}
		return stats.ClassifyVerdict(iv, total, pMin, minSamples)
	}

	passV := check(100, 100)
	flakyV := check(20, 20)
	failV := check(17, 20)

	rep.H7.PassCase = fmt.Sprintf("100/100 -> %s", passV)
	rep.H7.FlakyCase = fmt.Sprintf("20/20 -> %s", flakyV)
	rep.H7.FailCase = fmt.Sprintf("17/20 -> %s", failV)
	rep.H7.AllCorrect = passV == api.VerdictPass && flakyV == api.VerdictFlaky && failV == api.VerdictFail
}

func compositeScore(rep *Report) float64 {
	// Equal weight on the four AEE inputs (spec §24), each clamped to [0,1].
	dr := clamp01(rep.DetectionRate)
	fprGood := clamp01(1.0 - rep.FalsePositiveRate/MaxFalsePositive) // 0 FPR → 1, at cap → 0
	if rep.FalsePositiveRate <= MaxFalsePositive {
		fprGood = 1.0 - (rep.FalsePositiveRate / (2 * MaxFalsePositive))
	} else {
		fprGood = 0
	}
	speed := clamp01(rep.EvalThroughputCPS / MinEvalThroughput)
	if speed > 1 {
		speed = 1
	}
	repro := clamp01(rep.Reproducibility)
	h7 := 0.0
	if rep.H7.AllCorrect {
		h7 = 1.0
	}
	// H7 is reliability-engine proof; fold lightly into score with the four inputs.
	return 0.25*dr + 0.25*fprGood + 0.20*speed + 0.20*repro + 0.10*h7
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
