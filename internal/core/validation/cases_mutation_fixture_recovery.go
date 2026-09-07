package validation

import (
	"time"

	"gust/internal/adapters/mutators"
	"gust/internal/ports"
	"gust/pkg/api"
)

func casesMutation() []Case {
	names := []string{}
	for _, m := range mutators.AllBuiltinMutators() {
		names = append(names, m.Name())
	}
	out := make([]Case, 0, len(names)*2)
	for _, name := range names {
		out = append(out, Case{
			ID: "mut_detect_g0_" + name, Category: CatMutation, Kind: KindMutation,
			Question: "Should this mutation be detected?", Rationale: "mutator on golden_order_cancel must be caught or skipped",
			MutatorName: name, GoldenIdx: 0, WantDetected: true,
		})
		out = append(out, Case{
			ID: "mut_detect_g1_" + name, Category: CatMutation, Kind: KindMutation,
			Question: "Should this mutation be detected?", Rationale: "mutator on golden_recovery must be caught or skipped",
			MutatorName: name, GoldenIdx: 1, WantDetected: true,
		})
	}
	return out
}

func casesFixture() []Case {
	in := map[string]any{"user_id": 42}
	hash := mustHash(in)
	exact := []api.Fixture{{
		FixtureID: "fx_exact", Tool: "get_user", InputHash: hash,
		MatchStrategy: api.MatchStrategyExactHash, RecordedInput: in,
		RecordedResponse: api.RecordedResponse{Status: "success", Body: "alice"},
		Provenance: api.ProvenanceRecorded,
	}}
	seq := []api.Fixture{
		{
			FixtureID: "fx1", Tool: "poll", MatchStrategy: api.MatchStrategyOrderedSequence,
			RecordedResponse: api.RecordedResponse{Status: "success", Body: "PENDING"},
			Provenance: api.ProvenanceRecorded,
		},
		{
			FixtureID: "fx2", Tool: "poll", MatchStrategy: api.MatchStrategyOrderedSequence,
			RecordedResponse: api.RecordedResponse{Status: "success", Body: "DONE"},
			Provenance: api.ProvenanceRecorded,
		},
	}
	malformed := []api.Fixture{{
		FixtureID: "fx_mal", Tool: "echo", MatchStrategy: api.MatchStrategyExactHash,
		InputHash: mustHash(map[string]any{"x": 1}), RecordedInput: map[string]any{"x": 1},
		Mode: api.FailureModeMalformed, Provenance: api.ProvenanceAuthored,
		RecordedResponse: api.RecordedResponse{Status: "success", Body: "ignored"},
	}}
	partial := []api.Fixture{{
		FixtureID: "fx_pf", Tool: "echo", MatchStrategy: api.MatchStrategyExactHash,
		InputHash: mustHash(map[string]any{"x": 1}), RecordedInput: map[string]any{"x": 1},
		Mode: api.FailureModePartialFailure, Provenance: api.ProvenanceAuthored,
		RecordedResponse: api.RecordedResponse{Status: "success", Body: "ignored"},
	}}
	recordedErr := []api.Fixture{{
		FixtureID: "fx_re", Tool: "lookup", MatchStrategy: api.MatchStrategyExactHash,
		InputHash: mustHash(map[string]any{"id": 9}), RecordedInput: map[string]any{"id": 9},
		Mode: api.FailureModeRecordedError,
		RecordedResponse: api.RecordedResponse{Status: "error", Error: "timeout", Body: nil},
		Provenance: api.ProvenanceRecorded,
	}}

	return []Case{
		{
			ID: "fix_exact_hit", Category: CatFixture, Kind: KindFixture,
			Question: "Should this fixture match?", Rationale: "exact hash match returns recorded body",
			Fixtures: exact, Call: ports.ToolCall{Name: "get_user", Arguments: in},
			WantFound: true, WantStatus: "success",
		},
		{
			ID: "fix_exact_miss", Category: CatFixture, Kind: KindFixture,
			Question: "Should this fixture match?", Rationale: "wrong args do not match",
			Fixtures: exact, Call: ports.ToolCall{Name: "get_user", Arguments: map[string]any{"user_id": 99}},
			WantFound: false,
		},
		{
			ID: "fix_ordered_fifo", Category: CatFixture, Kind: KindFixture,
			Question: "Should this fixture match?", Rationale: "ordered sequence advances FIFO",
			Fixtures: seq, CallSequence: []ports.ToolCall{{Name: "poll"}, {Name: "poll"}},
			WantBodies: []string{"PENDING", "DONE"}, WantFound: true,
		},
		{
			ID: "fix_ordered_exhausted", Category: CatFixture, Kind: KindFixture,
			Question: "Should this fixture match?", Rationale: "third poll with only two fixtures is a miss",
			Fixtures: seq, CallSequence: []ports.ToolCall{{Name: "poll"}, {Name: "poll"}, {Name: "poll"}},
			WantFound: false,
		},
		{
			ID: "fix_malformed_mode", Category: CatFixture, Kind: KindFixture,
			Question: "Should this fixture match?", Rationale: "malformed mode returns broken body",
			Fixtures: malformed, Call: ports.ToolCall{Name: "echo", Arguments: map[string]any{"x": 1}},
			WantFound: true, WantStatus: "success",
		},
		{
			ID: "fix_partial_failure_mode", Category: CatFixture, Kind: KindFixture,
			Question: "Should this fixture match?", Rationale: "partial_failure injects error status",
			Fixtures: partial, Call: ports.ToolCall{Name: "echo", Arguments: map[string]any{"x": 1}},
			WantFound: true, WantStatus: "error",
		},
		{
			ID: "fix_recorded_error_preserved", Category: CatFixture, Kind: KindFixture,
			Question: "Should this fixture match?", Rationale: "recorded_error surfaces Status=error",
			Fixtures: recordedErr, Call: ports.ToolCall{Name: "lookup", Arguments: map[string]any{"id": 9}},
			WantFound: true, WantStatus: "error",
		},
		{
			ID: "fix_wrong_tool_miss", Category: CatFixture, Kind: KindFixture,
			Question: "Should this fixture match?", Rationale: "different tool name does not match",
			Fixtures: exact, Call: ports.ToolCall{Name: "other", Arguments: in},
			WantFound: false,
		},
		{
			ID: "fix_clone_isolates_sequence", Category: CatFixture, Kind: KindFixture,
			Question: "Should this fixture match?", Rationale: "cloned provider has independent counters (checked via two fresh sequences)",
			Fixtures: seq, CallSequence: []ports.ToolCall{{Name: "poll"}},
			WantBodies: []string{"PENDING"}, WantFound: true,
		},
	}
}

func casesRecovery() []Case {
	recovered := withTrace(baseRun("rec_ok"),
		errorSpan("e1", "fetch_doc", "network"),
		toolSpan("t1", "fetch_doc", nil, true),
	)
	recovered.Outcome.Output = "recovered doc"

	noRecovery := withTrace(baseRun("rec_fail"),
		errorSpan("e1", "fetch_doc", "network"),
		toolSpan("t1", "unrelated", nil, true),
	)

	causal := withTrace(baseRun("rec_causal"),
		errorSpan("e1", "fetch_doc", "network"),
		llmSpan("l1"),
		toolSpan("t1", "fetch_doc", nil, true),
	)

	neverErrored := withTrace(baseRun("rec_clean"),
		toolSpan("t1", "fetch_doc", nil, true),
	)

	return []Case{
		{
			ID: "recovery_retry_same_tool", Category: CatRecovery, Kind: KindAnalyze,
			Question: "Should this be considered recovery?", Rationale: "error then successful same tool is recovery",
			Run: recovered, WantPassed: true,
			Assertions: []api.Assertion{a("a1", api.AssertErrorRecovery)},
		},
		{
			ID: "recovery_missing_retry", Category: CatRecovery, Kind: KindAnalyze,
			Question: "Should this be considered recovery?", Rationale: "error without matching retry fails",
			Run: noRecovery, WantPassed: false,
			Assertions: []api.Assertion{a("a1", api.AssertErrorRecovery)},
		},
		{
			ID: "recovery_with_intervening_llm", Category: CatRecovery, Kind: KindAnalyze,
			Question: "Should this be considered recovery?", Rationale: "later matching tool after llm still recovers",
			Run: causal, WantPassed: true,
			Assertions: []api.Assertion{a("a1", api.AssertErrorRecovery)},
		},
		{
			ID: "recovery_no_error_passes", Category: CatRecovery, Kind: KindAnalyze,
			Question: "Should this be considered recovery?", Rationale: "no error spans means recovery assertion passes",
			Run: neverErrored, WantPassed: true,
			Assertions: []api.Assertion{a("a1", api.AssertErrorRecovery)},
		},
		{
			ID: "recovery_explicit_recovery_tool", Category: CatRecovery, Kind: KindAnalyze,
			Question: "Should this be considered recovery?", Rationale: "listed recovery_tools after error counts",
			Run: withTrace(baseRun("rec_listed"),
				errorSpan("e1", "fetch_doc", "network"),
				toolSpan("t1", "retrieve_weather", nil, true),
			),
			WantPassed: true,
			Assertions: []api.Assertion{
				a("a1", api.AssertErrorRecovery, withParams(map[string]any{
					"recovery_tools": []any{"retrieve_weather"},
				})),
			},
		},
		{
			ID: "recovery_wrong_listed_tool", Category: CatRecovery, Kind: KindAnalyze,
			Question: "Should this be considered recovery?", Rationale: "unlisted later tool is not recovery",
			Run: withTrace(baseRun("rec_wrong_listed"),
				errorSpan("e1", "fetch_doc", "network"),
				toolSpan("t1", "unrelated", nil, true),
			),
			WantPassed: false,
			Assertions: []api.Assertion{
				a("a1", api.AssertErrorRecovery, withParams(map[string]any{
					"recovery_tools": []any{"retrieve_weather"},
				})),
			},
		},
	}
}

func casesAdversarialExtras() []Case {
	outOfOrder := withTrace(baseRun("adv_ooo"),
		toolSpan("late", "b", nil, true),
		toolSpan("early", "a", nil, true),
	)
	// Fix times so early really starts first for latency
	t0 := now()
	outOfOrder.Trace[0].StartTime = t0.Add(50 * time.Millisecond)
	outOfOrder.Trace[0].EndTime = t0.Add(100 * time.Millisecond)
	outOfOrder.Trace[1].StartTime = t0
	outOfOrder.Trace[1].EndTime = t0.Add(10 * time.Millisecond)

	partialTrace := baseRun("adv_partial")
	partialTrace.Trace = []api.Span{toolSpan("s1", "get_orders", map[string]any{"customer_id": 1}, true)}
	// missing end? spans have end

	malformedAttrs := withTrace(baseRun("adv_mal_attrs"),
		func() api.Span {
			s := toolSpan("s1", "cancel_order", nil, true)
			s.Attributes = map[string]any{"input": "not-a-map"}
			return s
		}(),
	)

	return []Case{
		{
			ID: "adv_latency_unordered_spans", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "latency uses min(start)/max(end) not slice order",
			Run: outOfOrder, WantPassed: true,
			Assertions: []api.Assertion{a("a1", api.AssertMaxLatency, withLimit(200))},
		},
		{
			ID: "adv_partial_trace_required", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "partial trace missing required cancel",
			Run: partialTrace, WantPassed: false,
			Assertions: []api.Assertion{a("a1", api.AssertRequiredTool, withTool("cancel_order"))},
		},
		{
			ID: "adv_malformed_input_attr", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "non-map tool input cannot satisfy arg assertion",
			Run: malformedAttrs, WantPassed: false,
			Assertions: []api.Assertion{
				a("a1", api.AssertToolCall, withTool("cancel_order"), withArgs(map[string]any{"order_id": 1})),
			},
		},
		{
			ID: "adv_agent_span_counts_step", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "agent spans count toward max_steps",
			Run: withTrace(baseRun("adv_agent_steps"), agentSpan("a1"), agentSpan("a2")),
			WantPassed: false,
			Assertions: []api.Assertion{a("a1", api.AssertMaxSteps, withLimit(1))},
		},
		{
			ID: "adv_retrieval_not_step", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "retrieval spans are sub-steps",
			Run: withTrace(baseRun("adv_ret"),
				toolSpan("t1", "search", nil, true),
				func() api.Span {
					s := llmSpan("r1")
					s.Type = api.SpanTypeRetrieval
					s.Name = "retrieve"
					return s
				}(),
			),
			WantPassed: true,
			Assertions: []api.Assertion{a("a1", api.AssertMaxSteps, withLimit(1))},
		},
		{
			ID: "adv_output_substring_fail", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "expected_output substring absent",
			Run: func() api.AgentRun {
				r := baseRun("adv_out")
				r.Outcome.Output = "hello world"
				return r
			}(),
			WantPassed: false,
			Assertions: []api.Assertion{
				a("a1", api.AssertTaskSuccess, withParams(map[string]any{"expected_output": "cancelled"})),
			},
		},
	}
}
