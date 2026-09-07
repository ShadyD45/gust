package validation

import (
	"fmt"
	"strings"
	"time"
)

// FormatMarkdown renders a GitHub-flavored summary.
func FormatMarkdown(rep *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Gust Validation Suite\n\n")
	fmt.Fprintf(&b, "- Suite version: `%s`\n", rep.SuiteVersion)
	fmt.Fprintf(&b, "- Cases: **%d/%d** passed (%.1f%%)\n", rep.Passed, rep.Total, rep.PassRate*100)
	if rep.GatePassed {
		fmt.Fprintf(&b, "- Gate: **PASS**\n\n")
	} else {
		fmt.Fprintf(&b, "- Gate: **FAIL**\n\n")
	}
	fmt.Fprintf(&b, "| Category | Passed | Total | Rate |\n|---|---:|---:|---:|\n")
	for _, cat := range []string{"pass", "fail", "flaky", "infra", "mutation", "fixture", "recovery"} {
		st, ok := rep.ByCategory[cat]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %.0f%% |\n", cat, st.Passed, st.Total, st.Rate*100)
	}
	if len(rep.Failures) > 0 {
		fmt.Fprintf(&b, "\n### Failures\n\n")
		for _, f := range rep.Failures {
			fmt.Fprintf(&b, "- `%s` (%s): %s — %s\n", f.ID, f.Category, f.Question, f.Detail)
		}
	}
	return b.String()
}

// FormatDocMarkdown writes benchmarks/VALIDATION.md style content.
func FormatDocMarkdown(rep *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Gust Validation Suite results\n\n")
	fmt.Fprintf(&b, "_Generated %s (suite %s). Regenerated via `gust-aee validate --doc …`._\n\n",
		rep.GeneratedAt, rep.SuiteVersion)
	fmt.Fprintf(&b, "Adversarial cases that try to fool Gust — expected pass/fail/flaky/infra/mutation/fixture/recovery outcomes.\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| Total cases | %d |\n", rep.Total)
	fmt.Fprintf(&b, "| Passed | %d |\n", rep.Passed)
	fmt.Fprintf(&b, "| Failed | %d |\n", rep.Failed)
	fmt.Fprintf(&b, "| Pass rate | %.1f%% |\n", rep.PassRate*100)
	fmt.Fprintf(&b, "| Gate | %s |\n\n", map[bool]string{true: "PASS", false: "FAIL"}[rep.GatePassed])
	fmt.Fprintf(&b, "## By category\n\n")
	fmt.Fprintf(&b, "| Category | Passed | Total | Rate |\n|---|---:|---:|---:|\n")
	for _, cat := range []string{"pass", "fail", "flaky", "infra", "mutation", "fixture", "recovery"} {
		st, ok := rep.ByCategory[cat]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %.0f%% |\n", cat, st.Passed, st.Total, st.Rate*100)
	}
	fmt.Fprintf(&b, "\n## Trust claim\n\n")
	fmt.Fprintf(&b, "When this gate is green, Gust correctly classifies the adversarial scenarios in this suite — ")
	fmt.Fprintf(&b, "evidence that the evaluator/statistics/fixture/mutation machinery can be trusted to judge agent behavior.\n")
	return b.String()
}

// FormatSiteResultsMarkdown writes docs/benchmarks/validation.md with Just the Docs front matter.
func FormatSiteResultsMarkdown(rep *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\n")
	fmt.Fprintf(&b, "title: Validation Suite\n")
	fmt.Fprintf(&b, "nav_order: 4\n")
	fmt.Fprintf(&b, "parent: Benchmarks\n")
	fmt.Fprintf(&b, "---\n")
	fmt.Fprintf(&b, "# Gust Validation Suite\n\n")
	fmt.Fprintf(&b, "Published %s · suite `%s`\n\n", time.Now().UTC().Format("2006-01-02"), rep.SuiteVersion)
	fmt.Fprintf(&b, "A catalog of adversarial scenarios (\"Should this pass/fail/be flaky/…?\") that Gust must classify correctly.\n\n")
	fmt.Fprintf(&b, "| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| Cases | **%d / %d** passed |\n", rep.Passed, rep.Total)
	fmt.Fprintf(&b, "| Gate | **%s** |\n\n", map[bool]string{true: "PASS", false: "FAIL"}[rep.GatePassed])
	fmt.Fprintf(&b, "## Category breakdown\n\n")
	fmt.Fprintf(&b, "| Category | Question theme | Passed | Total |\n|---|---|---:|---:|\n")
	labels := map[string]string{
		"pass": "Should this pass?", "fail": "Should this fail?", "flaky": "Should this be flaky?",
		"infra": "Infrastructure error?", "mutation": "Mutation detected?",
		"fixture": "Fixture match?", "recovery": "Considered recovery?",
	}
	for _, cat := range []string{"pass", "fail", "flaky", "infra", "mutation", "fixture", "recovery"} {
		st, ok := rep.ByCategory[cat]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "| `%s` | %s | %d | %d |\n", cat, labels[cat], st.Passed, st.Total)
	}
	fmt.Fprintf(&b, "\nMachine-readable: [`benchmarks/fixtures/gust_validation.json`](https://github.com/ShadyD45/gust/blob/main/benchmarks/fixtures/gust_validation.json).\n")
	fmt.Fprintf(&b, "\nReproduce: `gust-aee validate`\n")
	return b.String()
}
