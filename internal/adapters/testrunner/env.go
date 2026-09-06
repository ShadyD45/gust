package testrunner

import (
	"strings"

	"gust/internal/adapters/ingest/otel"
)

const (
	EnvOTelProtocol       = "OTEL_EXPORTER_OTLP_PROTOCOL"
	EnvOTelTracesEndpoint = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	EnvOTelResourceAttrs  = "OTEL_RESOURCE_ATTRIBUTES"
	EnvIngestURL          = "AGENTEVAL_INGEST_URL"
)

// ApplyOTelEnv upserts the standard OTLP exporter variables so a child process
// (or an HTTP agent that reads the invoke body) reaches gust without extra config.
func ApplyOTelEnv(environ []string, otelURL, sampleID string) []string {
	if otelURL == "" {
		return environ
	}
	base := strings.TrimRight(otelURL, "/")
	environ = upsertEnv(environ, EnvOTelEndpoint, base)
	environ = upsertEnv(environ, EnvOTelProtocol, "http/protobuf")
	environ = upsertEnv(environ, EnvOTelTracesEndpoint, base+"/v1/traces")
	environ = upsertEnv(environ, EnvIngestURL, base+"/v1/runs")
	if sampleID != "" {
		environ = mergeResourceAttribute(environ, otel.AttrSampleID+"="+sampleID)
	}
	return environ
}

func upsertEnv(environ []string, key, value string) []string {
	prefix := key + "="
	for i, e := range environ {
		if strings.HasPrefix(e, prefix) {
			environ[i] = prefix + value
			return environ
		}
	}
	return append(environ, prefix+value)
}

func mergeResourceAttribute(environ []string, kv string) []string {
	prefix := EnvOTelResourceAttrs + "="
	for i, e := range environ {
		if !strings.HasPrefix(e, prefix) {
			continue
		}
		existing := strings.TrimPrefix(e, prefix)
		environ[i] = prefix + appendResourceKV(existing, kv)
		return environ
	}
	return append(environ, prefix+kv)
}

func appendResourceKV(existing, kv string) string {
	if existing == "" {
		return kv
	}
	key := kv
	if i := strings.IndexByte(kv, '='); i >= 0 {
		key = kv[:i]
	}
	for _, part := range strings.Split(existing, ",") {
		part = strings.TrimSpace(part)
		if part == kv || strings.HasPrefix(part, key+"=") {
			return existing
		}
	}
	return existing + "," + kv
}
