package evidence

import (
	"testing"
)

func TestCompareMapsNumericTypes(t *testing.T) {
	diffs := CompareMaps(
		map[string]any{"order_id": 123},
		map[string]any{"order_id": float64(123)},
		"",
	)
	if len(diffs) != 0 {
		t.Fatalf("int vs float64 should match, got %+v", diffs)
	}
}

func TestCompareMaps(t *testing.T) {
	exp := map[string]any{
		"a": 1,
		"b": map[string]any{
			"c": "target",
			"d": true,
		},
		"missing": "key",
	}

	act := map[string]any{
		"a": 1,
		"b": map[string]any{
			"c": "different",
			"d": true,
		},
		"extra": "ignored",
	}

	diffs := CompareMaps(exp, act, "")

	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d: %+v", len(diffs), diffs)
	}

	// diff 1: b.c mismatch
	if diffs[0].Path != "b.c" {
		t.Errorf("expected diff at b.c, got %s", diffs[0].Path)
	}

	// diff 2: missing key
	if diffs[1].Path != "missing" {
		t.Errorf("expected diff at missing, got %s", diffs[1].Path)
	}
}
