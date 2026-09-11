package cli

// Set via -ldflags at release build time, e.g.
// -X gust/internal/cli.version=v0.6.0
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Version returns the CLI version string (goreleaser ldflags or "dev").
func Version() string {
	if version == "dev" {
		return "dev"
	}
	return version
}
