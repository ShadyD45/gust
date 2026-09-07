package validation

import "gust/pkg/api"

func casesAnalyzePass() []Case {
	cancel := withTrace(baseRun("pass_cancel"),
		toolSpan("s1", "get_orders", map[string]any{"customer_id": 42}, true),
		toolSpan("s2", "cancel_order", map[string]any{"order_id": 123}, true),
	)
	cancel.Outcome.Output = "Order 123 cancelled successfully."

	out := []Case{
		{
			ID: "pass_task_success", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "completed outcome with expected substring",
			Run: cancel, WantPassed: true,
			Assertions: []api.Assertion{
				a("a1", api.AssertTaskSuccess, withParams(map[string]any{"expected_output": "cancelled"})),
			},
		},
		{
			ID: "pass_required_tool", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "required tool was invoked",
			Run: cancel, WantPassed: true,
			Assertions: []api.Assertion{a("a1", api.AssertRequiredTool, withTool("cancel_order"))},
		},
		{
			ID: "pass_tool_args", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "tool arguments match exactly",
			Run: cancel, WantPassed: true,
			Assertions: []api.Assertion{
				a("a1", api.AssertToolCall, withTool("cancel_order"), withArgs(map[string]any{"order_id": 123})),
			},
		},
		{
			ID: "pass_forbidden_absent", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "forbidden tool never called",
			Run: cancel, WantPassed: true,
			Assertions: []api.Assertion{a("a1", api.AssertForbiddenToolCall, withTool("forbidden_admin_access"))},
		},
		{
			ID: "pass_tool_sequence_subseq", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "tools appear in relative order",
			Run: cancel, WantPassed: true,
			Assertions: []api.Assertion{
				a("a1", api.AssertToolSequence, withParams(map[string]any{
					"sequence": []any{"get_orders", "cancel_order"},
					"match":    "subsequence",
				})),
			},
		},
		{
			ID: "pass_tool_sequence_exact", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "exact tool sequence matches",
			Run: cancel, WantPassed: true,
			Assertions: []api.Assertion{
				a("a1", api.AssertToolSequence, withParams(map[string]any{
					"sequence": []any{"get_orders", "cancel_order"},
					"match":    "exact",
				})),
			},
		},
		{
			ID: "pass_max_steps_tools_only", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "llm spans do not inflate step count",
			Run: withTrace(baseRun("pass_steps"),
				llmSpan("l1"),
				toolSpan("t1", "get_orders", map[string]any{"customer_id": 1}, true),
				llmSpan("l2"),
			),
			WantPassed: true,
			Assertions: []api.Assertion{a("a1", api.AssertMaxSteps, withLimit(1))},
		},
		{
			ID: "pass_max_latency", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "wall-clock span span within budget",
			Run: cancel, WantPassed: true,
			Assertions: []api.Assertion{a("a1", api.AssertMaxLatency, withLimit(5000))},
		},
		{
			ID: "pass_schema_output", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "output JSON matches declared schema",
			Run: func() api.AgentRun {
				r := baseRun("pass_schema")
				r.Outcome.Output = `{"status":"cancelled","order_id":123}`
				return r
			}(),
			WantPassed: true,
			Assertions: []api.Assertion{
				a("a1", api.AssertSchemaValid, withParams(map[string]any{
					"schema": map[string]any{
						"type":     "object",
						"required": []any{"status", "order_id"},
						"properties": map[string]any{
							"status":   map[string]any{"type": "string"},
							"order_id": map[string]any{"type": "integer"},
						},
					},
				})),
			},
		},
		{
			ID: "pass_empty_trace_task_only", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "zero-tool agent can still succeed on task_success",
			Run: baseRun("pass_empty"), WantPassed: true,
			Assertions: []api.Assertion{a("a1", api.AssertTaskSuccess)},
		},
		{
			ID: "pass_extra_tool_keys_ignored", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "argument subset match ignores extra keys",
			Run: withTrace(baseRun("pass_extra"),
				toolSpan("s1", "cancel_order", map[string]any{"order_id": 123, "reason": "user"}, true),
			),
			WantPassed: true,
			Assertions: []api.Assertion{
				a("a1", api.AssertToolCall, withTool("cancel_order"), withArgs(map[string]any{"order_id": 123})),
			},
		},
		{
			ID: "pass_occurrence_last", Category: CatPass, Kind: KindAnalyze,
			Question: "Should this pass?", Rationale: "occurrence=last selects final matching call",
			Run: withTrace(baseRun("pass_occ"),
				toolSpan("s1", "cancel_order", map[string]any{"order_id": 1}, true),
				toolSpan("s2", "cancel_order", map[string]any{"order_id": 123}, true),
			),
			WantPassed: true,
			Assertions: []api.Assertion{
				a("a1", api.AssertToolCall, withTool("cancel_order"), withArgs(map[string]any{"order_id": 123}),
					withParams(map[string]any{"occurrence": "last"})),
			},
		},
	}
	return out
}

func casesAnalyzeFail() []Case {
	cancel := withTrace(baseRun("fail_cancel"),
		toolSpan("s1", "get_orders", map[string]any{"customer_id": 42}, true),
		toolSpan("s2", "cancel_order", map[string]any{"order_id": 999}, true),
	)
	cancel.Outcome.Output = "Order 999 cancelled."

	failed := baseRun("fail_status")
	failed.Outcome.Status = "failed"
	failed.Outcome.Error = "boom"

	return []Case{
		{
			ID: "fail_task_status", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "outcome.status is failed",
			Run: failed, WantPassed: false,
			Assertions: []api.Assertion{a("a1", api.AssertTaskSuccess)},
		},
		{
			ID: "fail_wrong_order_id", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "tool args do not match expected order_id",
			Run: cancel, WantPassed: false,
			Assertions: []api.Assertion{
				a("a1", api.AssertToolCall, withTool("cancel_order"), withArgs(map[string]any{"order_id": 123})),
			},
		},
		{
			ID: "fail_forbidden_present", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "forbidden tool was called",
			Run: withTrace(baseRun("fail_forbidden"),
				toolSpan("s1", "forbidden_admin_access", map[string]any{"cmd": "drop"}, true),
			),
			WantPassed: false,
			Assertions: []api.Assertion{a("a1", api.AssertForbiddenToolCall, withTool("forbidden_admin_access"))},
		},
		{
			ID: "fail_missing_required", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "required tool never called",
			Run: withTrace(baseRun("fail_req"), toolSpan("s1", "get_orders", map[string]any{"customer_id": 1}, true)),
			WantPassed: false,
			Assertions: []api.Assertion{a("a1", api.AssertRequiredTool, withTool("cancel_order"))},
		},
		{
			ID: "fail_sequence_wrong_order", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "tools appear in wrong relative order",
			Run: withTrace(baseRun("fail_seq"),
				toolSpan("s1", "cancel_order", map[string]any{"order_id": 1}, true),
				toolSpan("s2", "get_orders", map[string]any{"customer_id": 1}, true),
			),
			WantPassed: false,
			Assertions: []api.Assertion{
				a("a1", api.AssertToolSequence, withParams(map[string]any{
					"sequence": []any{"get_orders", "cancel_order"},
				})),
			},
		},
		{
			ID: "fail_max_steps", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "too many top-level tool actions",
			Run: withTrace(baseRun("fail_steps"),
				toolSpan("s1", "a", nil, true),
				toolSpan("s2", "b", nil, true),
				toolSpan("s3", "c", nil, true),
			),
			WantPassed: false,
			Assertions: []api.Assertion{a("a1", api.AssertMaxSteps, withLimit(2))},
		},
		{
			ID: "fail_schema_missing_field", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "output missing required schema field",
			Run: func() api.AgentRun {
				r := baseRun("fail_schema")
				r.Outcome.Output = `{"status":"cancelled"}`
				return r
			}(),
			WantPassed: false,
			Assertions: []api.Assertion{
				a("a1", api.AssertSchemaValid, withParams(map[string]any{
					"schema": map[string]any{
						"type":     "object",
						"required": []any{"status", "order_id"},
						"properties": map[string]any{
							"status":   map[string]any{"type": "string"},
							"order_id": map[string]any{"type": "integer"},
						},
					},
				})),
			},
		},
		{
			ID: "fail_schema_non_json", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "non-JSON output with declared schema",
			Run: func() api.AgentRun {
				r := baseRun("fail_schema_text")
				r.Outcome.Output = "not json"
				return r
			}(),
			WantPassed: false,
			Assertions: []api.Assertion{
				a("a1", api.AssertSchemaValid, withParams(map[string]any{
					"schema": map[string]any{"type": "object"},
				})),
			},
		},
		{
			ID: "fail_schema_not_envelope_noop", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "valid envelope must not mask bad output schema",
			Run: func() api.AgentRun {
				r := baseRun("fail_schema_env")
				r.Outcome.Output = `{"wrong":true}`
				return r
			}(),
			WantPassed: false,
			Assertions: []api.Assertion{
				a("a1", api.AssertSchemaValid, withParams(map[string]any{
					"schema": map[string]any{
						"type":     "object",
						"required": []any{"status"},
						"properties": map[string]any{
							"status": map[string]any{"type": "string"},
						},
					},
				})),
			},
		},
		{
			ID: "fail_exact_sequence_extra", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "exact match rejects extra tool calls",
			Run: withTrace(baseRun("fail_exact"),
				toolSpan("s1", "get_orders", map[string]any{"customer_id": 1}, true),
				toolSpan("s2", "cancel_order", map[string]any{"order_id": 1}, true),
				toolSpan("s3", "notify", nil, true),
			),
			WantPassed: false,
			Assertions: []api.Assertion{
				a("a1", api.AssertToolSequence, withParams(map[string]any{
					"sequence": []any{"get_orders", "cancel_order"},
					"match":    "exact",
				})),
			},
		},
		{
			ID: "fail_missing_span_tool", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "assertion for tool absent from empty-ish trace",
			Run: withTrace(baseRun("fail_missing"), llmSpan("l1")),
			WantPassed: false,
			Assertions: []api.Assertion{a("a1", api.AssertToolCall, withTool("cancel_order"))},
		},
		{
			ID: "fail_duplicate_exact_sequence", Category: CatFail, Kind: KindAnalyze,
			Question: "Should this fail?", Rationale: "duplicate tool breaks exact sequence",
			Run: withTrace(baseRun("fail_dup"),
				toolSpan("s1", "get_orders", map[string]any{"customer_id": 1}, true),
				toolSpan("s2", "get_orders", map[string]any{"customer_id": 1}, true),
				toolSpan("s3", "cancel_order", map[string]any{"order_id": 1}, true),
			),
			WantPassed: false,
			Assertions: []api.Assertion{
				a("a1", api.AssertToolSequence, withParams(map[string]any{
					"sequence": []any{"get_orders", "cancel_order"},
					"match":    "exact",
				})),
			},
		},
	}
}
