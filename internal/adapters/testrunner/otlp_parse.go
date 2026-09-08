package testrunner

import (
	"gust/internal/adapters/ingest/otel"
	"gust/pkg/api"
)

func tryParseOTLP(data []byte, sampleID, traceID string) (api.AgentRun, error) {
	opts := otel.Options{RunID: sampleID, TraceID: traceID}
	return otel.NewMapper().MapBytes(data, opts)
}
