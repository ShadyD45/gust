package main

import (
	"os"

	"gust/internal/cli"
)

// gust-aee is the internal self-benchmark binary (AEE + Validation Suite).
// It is not published to end users via package managers — only "gust" is.
func main() {
	os.Exit(cli.ExecuteAEE())
}
