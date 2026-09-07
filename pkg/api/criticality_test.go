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
