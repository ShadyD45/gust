// Package gust is the stable Go library API for embedding Gust in your own
// programs and tests.
//
// Prefer this package over importing gust/internal/... — internal packages are
// not semver-stable and may change without notice.
//
// Typical analyze usage:
//
//	report, err := gust.Analyze(ctx, run, assertions)
//	if err != nil { ... }
//	if !report.Passed { t.Fatalf("%+v", report.Results) }
//
// Domain types (AgentRun, Assertion, …) live in gust/pkg/api.
package gust
