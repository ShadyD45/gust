package stats

import "testing"

func TestAttainablePASS(t *testing.T) {
	ok, lower, err := AttainablePASS(20, 0.95, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("20 samples cannot reach 95%% floor, lower=%.4f", lower)
	}
	ok, lower, err = AttainablePASS(100, 0.95, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || lower < 0.95 {
		t.Fatalf("expected attainable, lower=%.4f", lower)
	}
}
