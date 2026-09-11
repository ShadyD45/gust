package validation

import (
	"gust/internal/ports"
	"gust/pkg/api"
)

func orderedPollFixtures() []api.Fixture {
	return []api.Fixture{
		{
			FixtureID:        "fx_poll_pending",
			Tool:             "poll",
			MatchStrategy:    api.MatchStrategyOrderedSequence,
			RecordedResponse: api.RecordedResponse{Status: "success", Body: "PENDING"},
			Provenance:       api.ProvenanceRecorded,
		},
		{
			FixtureID:        "fx_poll_done",
			Tool:             "poll",
			MatchStrategy:    api.MatchStrategyOrderedSequence,
			RecordedResponse: api.RecordedResponse{Status: "success", Body: "DONE"},
			Provenance:       api.ProvenanceRecorded,
		},
	}
}

func exactEchoFixtures() []api.Fixture {
	return []api.Fixture{
		{
			FixtureID:     "fx_echo",
			Tool:          "echo",
			RecordedInput: map[string]any{"msg": "hi"},
			RecordedResponse: api.RecordedResponse{
				Status: "success",
				Body:   "ok",
			},
			Provenance: api.ProvenanceRecorded,
		},
	}
}

func casesMode3() []Case {
	pollCalls := []ports.ToolCall{{Name: "poll"}, {Name: "poll"}}
	echoCall := []ports.ToolCall{{Name: "echo", Arguments: map[string]any{"msg": "hi"}}}
	return []Case{
		{
			ID: "mode3_ordered_fixture_concurrent", Category: CatMode3, Kind: KindMode3,
			Question:  "Does each concurrent sample see the same ordered fixture sequence?",
			Rationale: "cloned provider + ephemeral proxy; 4 workers, 20 samples",
			Fixtures:  orderedPollFixtures(), Mode3Calls: pollCalls,
			Mode3Samples: 20, Mode3Concurrency: 4, WantMode3Passes: 20, WantAllFound: true,
		},
		{
			ID: "mode3_exact_fixture_concurrent", Category: CatMode3, Kind: KindMode3,
			Question:  "Do exact-hash fixtures stay isolated under concurrency?",
			Rationale: "hash match must not race across samples",
			Fixtures:  exactEchoFixtures(), Mode3Calls: echoCall,
			Mode3Samples: 12, Mode3Concurrency: 4, WantMode3Passes: 12, WantAllFound: true,
		},
		{
			ID: "mode3_sample_isolation", Category: CatMode3, Kind: KindMode3,
			Question:  "Does one sample's consumption starve another?",
			Rationale: "each sample must get PENDING then DONE independently",
			Fixtures:  orderedPollFixtures(), Mode3Calls: pollCalls,
			Mode3Samples: 8, Mode3Concurrency: 8, WantMode3Passes: 8, WantAllFound: true,
		},
		{
			ID: "mode3_fixture_exhaustion", Category: CatMode3, Kind: KindMode3,
			Question:  "Does over-calling a sequence fail only that sample?",
			Rationale: "third poll has no fixture; other samples still isolated",
			Fixtures:  orderedPollFixtures(), Mode3Calls: pollCalls, Mode3ExtraCalls: 1,
			Mode3Samples: 6, Mode3Concurrency: 3, WantMode3Passes: 0, WantAllFound: false,
		},
		{
			ID: "mode3_retry_does_not_corrupt_fixture_state", Category: CatMode3, Kind: KindMode3,
			Question:  "Does retry reset per-sample fixture counters?",
			Rationale: "transient fail after consuming PENDING must Reset before the successful attempt",
			Fixtures:  orderedPollFixtures(), Mode3Calls: pollCalls, Mode3FailFirst: true,
			Mode3Samples: 8, Mode3Concurrency: 4, WantMode3Passes: 8, WantAllFound: true,
		},
		{
			ID: "mode3_stateful_counter_stress", Category: CatMode3, Kind: KindMode3,
			Question:  "Do concurrent samples isolate stateful get→inc→get counters?",
			Rationale: "N=100 concurrency=16; each sample must see 0 then 1 with no cross-sample leakage",
			Fixtures: []api.Fixture{
				{FixtureID: "fx_get_0", Tool: "get_counter", MatchStrategy: api.MatchStrategyOrderedSequence, RecordedResponse: api.RecordedResponse{Status: "success", Body: "0"}, Provenance: api.ProvenanceRecorded},
				{FixtureID: "fx_inc", Tool: "increment", MatchStrategy: api.MatchStrategyOrderedSequence, RecordedResponse: api.RecordedResponse{Status: "success", Body: "ok"}, Provenance: api.ProvenanceRecorded},
				{FixtureID: "fx_get_1", Tool: "get_counter", MatchStrategy: api.MatchStrategyOrderedSequence, RecordedResponse: api.RecordedResponse{Status: "success", Body: "1"}, Provenance: api.ProvenanceRecorded},
			},
			Mode3Calls: []ports.ToolCall{{Name: "get_counter"}, {Name: "increment"}, {Name: "get_counter"}},
			Mode3Samples: 100, Mode3Concurrency: 16, WantMode3Passes: 100, WantAllFound: true,
		},
	}
}
