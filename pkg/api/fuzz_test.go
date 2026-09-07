package api

import (
	"encoding/json"
	"testing"
)

func FuzzAgentRunJSON(f *testing.F) {
	seed := []byte(`{"schema_version":"0.5","run_id":"r1","agent":{"name":"a","version":"1"},"task":{"id":"t","input":"x"},"trace":[],"outcome":{"status":"completed"}}`)
	f.Add(seed)
	f.Fuzz(func(t *testing.T, data []byte) {
		var run AgentRun
		if err := json.Unmarshal(data, &run); err != nil {
			return
		}
		_ = run.Validate()
	})
}

func FuzzAssertionJSON(f *testing.F) {
	f.Add([]byte(`{"id":"a1","type":"task_success"}`))
	f.Add([]byte(`{"id":"j","type":"llm_judge","criticality":"hard"}`))
	f.Add([]byte(`{"id":"c","type":"pii_leak","tool":"send_email"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var a Assertion
		if err := json.Unmarshal(data, &a); err != nil {
			return
		}
		_ = ValidateAssertion(&a, 0)
		_ = SampleCriticality(a, false)
		_ = AssertionFailsSample(a)
	})
}
