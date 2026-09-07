package evidence

import (
	"encoding/json"
	"testing"
)

func FuzzCompareMaps(f *testing.F) {
	f.Add(`{"a":1,"b":"x"}`, `{"a":1,"b":"x"}`)
	f.Add(`{"nested":{"k":true}}`, `{"nested":{"k":false}}`)
	f.Fuzz(func(t *testing.T, left, right string) {
		var em, am map[string]any
		if json.Unmarshal([]byte(left), &em) != nil {
			return
		}
		if json.Unmarshal([]byte(right), &am) != nil {
			return
		}
		_ = CompareMaps(em, am, "")
	})
}
