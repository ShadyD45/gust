package validation

import "gust/pkg/api"

func casesWilson() []Case {
	return []Case{
		{
			ID: "wilson_pass_100_100", Category: CatPass, Kind: KindWilson,
			Question: "Should this pass?", Rationale: "100/100 lower bound clears 0.95",
			Passes: 100, Total: 100, MinPass: 0.95, Confidence: 0.95, MinSamples: 5,
			WantVerdict: api.VerdictPass,
		},
		{
			ID: "wilson_flaky_20_20", Category: CatFlaky, Kind: KindWilson,
			Question: "Should this be flaky?", Rationale: "20/20 straddles 0.95 threshold",
			Passes: 20, Total: 20, MinPass: 0.95, Confidence: 0.95, MinSamples: 5,
			WantVerdict: api.VerdictFlaky,
		},
		{
			ID: "wilson_fail_17_20", Category: CatFail, Kind: KindWilson,
			Question: "Should this fail?", Rationale: "17/20 upper bound below 0.95",
			Passes: 17, Total: 20, MinPass: 0.95, Confidence: 0.95, MinSamples: 5,
			WantVerdict: api.VerdictFail,
		},
		{
			ID: "wilson_fail_0_20", Category: CatFail, Kind: KindWilson,
			Question: "Should this fail?", Rationale: "zero passes is definitive fail",
			Passes: 0, Total: 20, MinPass: 0.95, Confidence: 0.95, MinSamples: 5,
			WantVerdict: api.VerdictFail,
		},
		{
			ID: "wilson_insufficient_n3", Category: CatFail, Kind: KindWilson,
			Question: "Should this fail?", Rationale: "below min samples is INSUFFICIENT_SAMPLES",
			Passes: 3, Total: 3, MinPass: 0.95, Confidence: 0.95, MinSamples: 5,
			WantVerdict: api.VerdictInsufficientSamples,
		},
		{
			ID: "wilson_pass_loose_threshold", Category: CatPass, Kind: KindWilson,
			Question: "Should this pass?", Rationale: "19/20 clears min_pass 0.70",
			Passes: 19, Total: 20, MinPass: 0.70, Confidence: 0.95, MinSamples: 5,
			WantVerdict: api.VerdictPass,
		},
		{
			ID: "wilson_flaky_straddle_80", Category: CatFlaky, Kind: KindWilson,
			Question: "Should this be flaky?", Rationale: "interval straddles 0.80",
			Passes: 16, Total: 20, MinPass: 0.80, Confidence: 0.95, MinSamples: 5,
			WantVerdict: api.VerdictFlaky,
		},
		{
			ID: "wilson_conf_990_stricter", Category: CatFlaky, Kind: KindWilson,
			Question: "Should this be flaky?", Rationale: "higher confidence widens interval into flaky",
			Passes: 95, Total: 100, MinPass: 0.95, Confidence: 0.99, MinSamples: 5,
			WantVerdict: api.VerdictFlaky,
		},
		{
			ID: "wilson_pass_nonstandard_975", Category: CatPass, Kind: KindWilson,
			Question: "Should this pass?", Rationale: "non-table confidence 0.975 still computes via Erfinv",
			Passes: 100, Total: 100, MinPass: 0.90, Confidence: 0.975, MinSamples: 5,
			WantVerdict: api.VerdictPass,
		},
		{
			ID: "wilson_fail_40_100", Category: CatFail, Kind: KindWilson,
			Question: "Should this fail?", Rationale: "low rate at large N is fail",
			Passes: 40, Total: 100, MinPass: 0.95, Confidence: 0.95, MinSamples: 5,
			WantVerdict: api.VerdictFail,
		},
	}
}

func casesInfra() []Case {
	return []Case{
		{
			ID: "infra_one_error_tolerated", Category: CatInfra, Kind: KindInfra,
			Question: "Should this be an infrastructure error?", Rationale: "1/5 exec errors under 20% rate still yields a result",
			FailFirstN: 1, Samples: 5, MaxExecRate: 0.50, WantUnstable: false, WantExecErrs: 1,
		},
		{
			ID: "infra_unstable_over_rate", Category: CatInfra, Kind: KindInfra,
			Question: "Should this be an infrastructure error?", Rationale: "exec error rate above max is runner-unstable",
			FailFirstN: 3, Samples: 5, MaxExecRate: 0.20, WantUnstable: true,
		},
		{
			ID: "infra_all_fail_unstable", Category: CatInfra, Kind: KindInfra,
			Question: "Should this be an infrastructure error?", Rationale: "all samples erroring is harness failure",
			FailFirstN: 5, Samples: 5, MaxExecRate: 0.20, WantUnstable: true,
		},
		{
			ID: "infra_zero_errors_clean", Category: CatPass, Kind: KindInfra,
			Question: "Should this pass?", Rationale: "no exec errors with synthetic perfect runner",
			FailFirstN: 0, Samples: 5, MaxExecRate: 0.20, WantUnstable: false, WantExecErrs: 0,
		},
		{
			ID: "infra_boundary_rate_ok", Category: CatInfra, Kind: KindInfra,
			Question: "Should this be an infrastructure error?", Rationale: "exactly at max rate is still tolerated (strict >)",
			FailFirstN: 1, Samples: 5, MaxExecRate: 0.20, WantUnstable: false, WantExecErrs: 1,
		},
	}
}
