package evidence

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// DiffItem represents a single property discrepancy.
type DiffItem struct {
	Path     string `json:"path"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Message  string `json:"message"`
}

// CompareMaps recursively compares expected vs actual maps and returns a list of diffs.
func CompareMaps(expected, actual map[string]any, prefix string) []DiffItem {
	var diffs []DiffItem

	// Check expected keys in actual
	for k, expVal := range expected {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}

		actVal, ok := actual[k]
		if !ok {
			diffs = append(diffs, DiffItem{
				Path:     path,
				Expected: expVal,
				Actual:   nil,
				Message:  fmt.Sprintf("missing expected key %q", path),
			})
			continue
		}

		// If both are maps, recurse
		expMap, expIsMap := expVal.(map[string]any)
		actMap, actIsMap := actVal.(map[string]any)
		if expIsMap && actIsMap {
			diffs = append(diffs, CompareMaps(expMap, actMap, path)...)
			continue
		}

		// Otherwise compare values. JSON unmarshalling uses float64; YAML
		// often uses int — treat numeric types as equal when the number matches.
		if !valuesEqual(expVal, actVal) {
			diffs = append(diffs, DiffItem{
				Path:     path,
				Expected: expVal,
				Actual:   actVal,
				Message:  fmt.Sprintf("value mismatch at %q: expected %v, got %v", path, expVal, actVal),
			})
		}
	}

	// Sort diffs by path for deterministic evidence
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Path < diffs[j].Path
	})

	return diffs
}

func valuesEqual(expected, actual any) bool {
	if reflect.DeepEqual(expected, actual) {
		return true
	}
	ef, eok := asFloat(expected)
	af, aok := asFloat(actual)
	return eok && aok && ef == af
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
