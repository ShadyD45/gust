package validation

import (
	"gust/internal/ports"
	"gust/pkg/api"
)

func casesInvariants() []Case {
	failed := baseRun("s6_soft")
	failed.Outcome.Status = "failed"
	failed.Outcome.Error = "boom"

	pollCalls := []ports.ToolCall{{Name: "poll"}, {Name: "poll"}}

	return []Case{
		{
			ID: "s1_s8_fixture_isolation", Category: CatInvariant, Kind: KindInvariant,
			Question:         "S1/S8: Does every sample get isolated fixture state?",
			Rationale:        "concurrent ordered fixtures must not share sequence counters",
			InvariantID:      "s1_s8",
			Fixtures:         orderedPollFixtures(),
			Mode3Calls:       pollCalls,
			Mode3Samples:     20,
			Mode3Concurrency: 4,
			WantMode3Passes:  20,
		},
		{
			ID: "s2_nontransient_not_retried", Category: CatInvariant, Kind: KindInvariant,
			Question:    "S2: Can a failed agent execution become PASS through retry?",
			Rationale:   "only errors classified as transient may retry",
			InvariantID: "s2",
		},
		{
			ID: "s3_s4_one_trace_per_sample", Category: CatInvariant, Kind: KindInvariant,
			Question:    "S3/S4: Does every sample have exactly one attributable trace?",
			Rationale:   "N samples produce N unique run IDs, each with one trace",
			InvariantID: "s3_s4",
		},
		{
			ID: "s5_exec_errors_counted", Category: CatInvariant, Kind: KindInvariant,
			Question:     "S5: Are execution errors ever silently dropped?",
			Rationale:    "an execution error consumes a sample and lowers reliability",
			InvariantID:  "s5",
			FailFirstN:   1,
			Samples:      5,
			MaxExecRate:  0.50,
			WantExecErrs: 1,
			WantUnstable: false,
		},
		{
			ID: "s6_soft_cannot_fail_sample", Category: CatInvariant, Kind: KindInvariant,
			Question:    "S6: Can a soft assertion fail a sample?",
			Rationale:   "soft failures produce evidence only",
			InvariantID: "s6",
			Run:         failed,
			WantPassed:  true,
			Assertions: []api.Assertion{{
				ID:          "soft_success",
				Type:        api.AssertTaskSuccess,
				Criticality: api.CriticalitySoft,
			}},
		},
		{
			ID: "s6_hard_not_downgraded", Category: CatInvariant, Kind: KindInvariant,
			Question:    "S6: Can a hard assertion be downgraded by a soft evaluator?",
			Rationale:   "hard task_success still fails the sample even if a soft assertion passes",
			InvariantID: "s6_hard",
			Run:         failed,
			WantPassed:  false,
			Assertions: []api.Assertion{
				{ID: "hard_success", Type: api.AssertTaskSuccess, Criticality: api.CriticalityHard},
				{ID: "soft_ok", Type: api.AssertMaxSteps, Criticality: api.CriticalitySoft, Limit: 100},
			},
		},
		{
			ID: "s7_deterministic_equivalent_evidence", Category: CatInvariant, Kind: KindInvariant,
			Question:    "S7: Does the same scenario + fixtures + deterministic runner yield equivalent evidence?",
			Rationale:   "synthetic seed + fixed outcomes must match across two sampler runs",
			InvariantID: "s7",
		},
		{
			ID: "s9_policy_thresholds", Category: CatInvariant, Kind: KindInvariant,
			Question:    "S9: Is policy configuration reflected in the final verdict?",
			Rationale:   "hard_constraints.forbidden_tools counts change the CI exit code",
			InvariantID: "s9",
		},
		{
			ID: "s10_replay_no_live_io", Category: CatInvariant, Kind: KindInvariant,
			Question:    "S10: Can replay invoke a live external dependency?",
			Rationale:   "ReplayTrace only looks up fixtures; it never Records or dials the network",
			InvariantID: "s10",
		},
	}
}
