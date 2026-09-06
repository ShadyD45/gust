//go:build ignore

package main

import (
	"encoding/json"
	"os"

	corejudge "gust/internal/core/judge"
)

func main() {
	ds := corejudge.SeedDataset()
	b, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll("testdata/judge/calibration", 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile("testdata/judge/calibration/v1.json", b, 0o644); err != nil {
		panic(err)
	}
}
