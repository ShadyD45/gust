package aee

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const defaultMarkdownFooter = "_Self-benchmark only - methodology: [`docs/usage/aee-methodology.md`](../docs/usage/aee-methodology.md)._"

const siteMarkdownFooter = "_Self-benchmark only — [metrics explained]({% link benchmarks/metrics.md %}) · [methodology]({% link usage/aee-methodology.md %}) · [repo RESULTS.md](https://github.com/ShadyD45/gust/blob/main/benchmarks/RESULTS.md)._"

// FormatMarkdown returns a GitHub-flavored markdown report for step summaries and docs.
func FormatMarkdown(rep *Report) string {
	return formatMarkdown(rep, defaultMarkdownFooter)
}

func formatMarkdown(rep *Report, footer string) string {
	var b strings.Builder
	gate := "**PASS**"
	if !rep.Passed {
		gate = "**FAIL**"
	}
	b.WriteString("## gust AEE self-report\n\n")
	fmt.Fprintf(&b, "**Gate:** %s | **Score:** `%.3f` | **Generated:** `%s`\n\n",
		gate, rep.Score, rep.GeneratedAt.UTC().Format(time.RFC3339))

	b.WriteString("### Metrics\n\n")
	b.WriteString("| Metric | Value | Gate |\n")
	b.WriteString("| --- | --- | --- |\n")
	fmt.Fprintf(&b, "| Detection rate | **%.1f%%** (%d/%d mutants) | >= %.0f%% |\n",
		rep.DetectionRate*100, rep.MutantsDetected, rep.MutantsApplied, MinDetectionRate*100)
	fmt.Fprintf(&b, "| False positive rate (golden suite) | **%.1f%%** | <= %.0f%% |\n",
		rep.FalsePositiveRate*100, MaxFalsePositive*100)
	fmt.Fprintf(&b, "| Deterministic evaluator throughput | **%s** evaluator calls/sec | >= %.0f |\n",
		FormatThroughput(rep.EvalThroughputCPS), MinEvalThroughput)
	fmt.Fprintf(&b, "| Reproducibility | **%.0f%%** identical (%d trials) | 100%% |\n",
		rep.Reproducibility*100, ReproTrials)

	b.WriteString("\n### Reliability engine (H7)\n\n")
	b.WriteString("| Vector | Result |\n")
	b.WriteString("| --- | --- |\n")
	fmt.Fprintf(&b, "| PASS case | `%s` |\n", rep.H7.PassCase)
	fmt.Fprintf(&b, "| FLAKY case | `%s` |\n", rep.H7.FlakyCase)
	fmt.Fprintf(&b, "| FAIL case | `%s` |\n", rep.H7.FailCase)
	h7 := "all correct"
	if !rep.H7.AllCorrect {
		h7 = "misclassified"
	}
	fmt.Fprintf(&b, "| Overall | **%s** |\n", h7)

	if rep.Hardening != nil {
		b.WriteString("\n### Hardening extras (informational)\n\n")
		fmt.Fprintf(&b, "- Regression noise test: **%v**\n", rep.Hardening.RegressionNoiseOK)
		fmt.Fprintf(&b, "- Replay identity (byte-stable): **%v**\n", rep.Hardening.ReplayIdentityOK)
		if len(rep.Hardening.AnalyzeLatency) > 0 {
			b.WriteString("\nAnalyze latency by trace size:\n\n")
			b.WriteString("| Spans | p50 | p95 |\n| --- | --- | --- |\n")
			for _, row := range rep.Hardening.AnalyzeLatency {
				fmt.Fprintf(&b, "| %d | %s | %s |\n", row.Spans, formatNs(row.P50Ns), formatNs(row.P95Ns))
			}
		}
		if len(rep.Hardening.MutatorBreakdown) > 0 {
			b.WriteString("\nPer-mutator detection (catalog size, not field FNR):\n\n")
			b.WriteString("| Mutator | Detected/Applied | Rate |\n| --- | --- | --- |\n")
			names := make([]string, 0, len(rep.Hardening.MutatorBreakdown))
			for k := range rep.Hardening.MutatorBreakdown {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, k := range names {
				v := rep.Hardening.MutatorBreakdown[k]
				fmt.Fprintf(&b, "| `%s` | %d/%d | %.0f%% |\n", k, v.Detected, v.Applied, v.Rate*100)
			}
		}
		if rep.Hardening.JudgeSpearmanNote != "" {
			fmt.Fprintf(&b, "\n_%s_\n", rep.Hardening.JudgeSpearmanNote)
		}
	}

	if len(rep.Notes) > 0 {
		b.WriteString("\n### Notes\n\n")
		for _, n := range rep.Notes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
	}

	b.WriteString("\n---\n\n")
	b.WriteString(footer)
	b.WriteString("\n")
	return b.String()
}

func reproduceBlock() string {
	var b strings.Builder
	b.WriteString("## Reproduce locally\n\n")
	b.WriteString("```bash\n")
	b.WriteString("go build -o gust-aee ./cmd/gust-aee\n")
	b.WriteString("./gust-aee report\n")
	b.WriteString("./gust-aee report --json --out benchmarks/fixtures/gust_self_aee.json\n")
	b.WriteString("./gust-aee report --doc benchmarks/RESULTS.md --site-doc docs/benchmarks/results.md\n")
	b.WriteString("```\n")
	return b.String()
}

// FormatDocMarkdown is the full file body for benchmarks/RESULTS.md.
func FormatDocMarkdown(rep *Report) string {
	var b strings.Builder
	b.WriteString("# AEE self-report (latest)\n\n")
	b.WriteString("This file is **updated by CI** via a pull request from the `benchmark` workflow\n")
	b.WriteString("(so a protected `main` branch still works). Do not edit the metrics tables by\n")
	b.WriteString("hand - re-run `gust-aee report` instead.\n\n")
	b.WriteString(FormatMarkdown(rep))
	b.WriteString("\n")
	b.WriteString(reproduceBlock())
	return b.String()
}

// FormatSiteResultsMarkdown is the Just the Docs page body for docs/benchmarks/results.md.
func FormatSiteResultsMarkdown(rep *Report) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: Latest results\n")
	b.WriteString("nav_order: 2\n")
	b.WriteString("parent: Benchmarks\n")
	b.WriteString("---\n")
	b.WriteString("# Latest AEE results\n\n")
	b.WriteString("This page is **updated by CI** via a pull request from the `benchmark` workflow\n")
	b.WriteString("(so a protected `main` branch still works). Do not edit the metrics tables by\n")
	b.WriteString("hand — re-run `gust-aee report --site-doc` instead.\n\n")
	b.WriteString(formatMarkdown(rep, siteMarkdownFooter))
	b.WriteString("\n")
	b.WriteString(reproduceBlock())
	return b.String()
}

// FormatThroughput renders large evaluator-calls/sec values readably.
func FormatThroughput(cps float64) string {
	switch {
	case cps >= 1e9:
		return fmt.Sprintf("%.1fB", cps/1e9)
	case cps >= 1e6:
		return fmt.Sprintf("%.1fM", cps/1e6)
	case cps >= 1e3:
		return fmt.Sprintf("%.1fK", cps/1e3)
	default:
		return fmt.Sprintf("%.0f", cps)
	}
}

func formatNs(ns int64) string {
	if ns < 1_000 {
		return fmt.Sprintf("%dns", ns)
	}
	if ns < 1_000_000 {
		return fmt.Sprintf("%.1fµs", float64(ns)/1e3)
	}
	if ns < 1_000_000_000 {
		return fmt.Sprintf("%.1fms", float64(ns)/1e6)
	}
	return fmt.Sprintf("%.2fs", float64(ns)/1e9)
}
