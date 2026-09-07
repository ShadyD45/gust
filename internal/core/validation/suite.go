package validation

// AllCases returns the full Gust Validation Suite catalog.
func AllCases() []Case {
	out := make([]Case, 0, 96)
	out = append(out, casesAnalyzePass()...)
	out = append(out, casesAnalyzeFail()...)
	out = append(out, casesWilson()...)
	out = append(out, casesInfra()...)
	out = append(out, casesMutation()...)
	out = append(out, casesFixture()...)
	out = append(out, casesRecovery()...)
	out = append(out, casesAdversarialExtras()...)
	out = append(out, casesMode3()...)
	return out
}
