package main

import (
	"os"

	"gust/internal/cli"
	"gust/pkg/api"
)

func main() {
	code := cli.Execute()
	if code != 0 {
		os.Exit(code)
	}
	// Cobra RunE paths may call os.Exit for policy codes; default success:
	_ = api.ExitSuccess
}
