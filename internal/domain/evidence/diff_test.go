package evidence

import (
	"testing"
)

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
