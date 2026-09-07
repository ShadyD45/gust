package api

import "testing"

func TestSampleCriticalityJudgeStaysSoftUntilCalibrated(t *testing.T) {
	a := Assertion{ID: "j", Type: AssertLLMJudge, Criticality: CriticalityHard}
	if SampleCriticality(a, false) != CriticalitySoft {
		t.Fatal("uncalibrated hard judge must be soft")
	}
	if AssertionFailsSample(a) {
		t.Fatal("uncalibrated judge must not fail a sample")
	}
	if SampleCriticality(a, true) != CriticalityHard {
		t.Fatal("calibrated explicit hard judge should stay hard")
	}
	p := Assertion{ID: "p", Type: AssertJudgePanel, Criticality: CriticalityHard}
	if SampleCriticality(p, false) != CriticalitySoft {
		t.Fatal("uncalibrated panel must be soft")
	}
}

func TestCountPolicyHardFailuresNamedBucketsOnly(t *testing.T) {
	evs := []EvaluationResult{
		{EvaluatorName: "forbidden_tool", Passed: false},
		{EvaluatorName: "forbidden_tool", Passed: false, Criticality: CriticalitySoft},
		{EvaluatorName: "schema_validation", Passed: false},
		{EvaluatorName: "tool_sequence", Passed: false, Criticality: CriticalityHard},
		{EvaluatorName: "llm_judge", Passed: false, Criticality: CriticalitySoft},
	}
	c := CountPolicyHardFailures(evs)
	if c.Forbidden != 1 || c.Schema != 1 {
		t.Fatalf("counts=%+v want forbidden=1 schema=1", c)
	}
	hc := HardConstraints{ForbiddenTools: 1, SchemaViolations: 0}
	if !c.Exceeds(hc) {
		t.Fatal("schema 1 > 0 should exceed")
	}
	hc.SchemaViolations = 1
	if c.Exceeds(hc) {
		t.Fatal("at maxima should not exceed")
	}
}

func TestAssertionPolicyHardNamedBuckets(t *testing.T) {
	if !AssertionPolicyHard(Assertion{Type: AssertForbiddenToolCall}) {
		t.Fatal("forbidden_tool_call should count toward named hard constraints")
	}
	if AssertionPolicyHard(Assertion{Type: AssertToolSequence, Criticality: CriticalityHard}) {
		t.Fatal("hard tool_sequence is a sample failure, not a named hard-constraint bucket")
	}
	if AssertionPolicyHard(Assertion{Type: AssertForbiddenToolCall, Criticality: CriticalitySoft}) {
		t.Fatal("soft forbidden_tool_call must not count")
	}
}

func TestValidateCustomAssertionType(t *testing.T) {
	a := Assertion{ID: "c", Type: "pii_leak"}
	if err := ValidateAssertion(&a, 0); err != nil {
		t.Fatal(err)
	}
	a.Type = ""
	if err := ValidateAssertion(&a, 0); err == nil {
		t.Fatal("empty type should fail")
	}
}
