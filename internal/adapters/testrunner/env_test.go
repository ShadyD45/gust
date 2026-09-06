package testrunner

import (
	"strings"
	"testing"

	"gust/internal/adapters/ingest/otel"
)

func TestApplyOTelEnv(t *testing.T) {
	env := ApplyOTelEnv([]string{"PATH=/bin", EnvOTelResourceAttrs + "=service.name=agent"}, "http://127.0.0.1:4318", "sid-9")
	got := map[string]string{}
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		got[k] = v
	}
	if got[EnvOTelEndpoint] != "http://127.0.0.1:4318" {
		t.Fatalf("endpoint %s", got[EnvOTelEndpoint])
	}
	if got[EnvOTelProtocol] != "http/protobuf" {
		t.Fatalf("protocol %s", got[EnvOTelProtocol])
	}
	if got[EnvOTelTracesEndpoint] != "http://127.0.0.1:4318/v1/traces" {
		t.Fatalf("traces %s", got[EnvOTelTracesEndpoint])
	}
	if got[EnvIngestURL] != "http://127.0.0.1:4318/v1/runs" {
		t.Fatalf("ingest %s", got[EnvIngestURL])
	}
	if !strings.Contains(got[EnvOTelResourceAttrs], "service.name=agent") {
		t.Fatalf("lost existing resource attrs: %s", got[EnvOTelResourceAttrs])
	}
	if !strings.Contains(got[EnvOTelResourceAttrs], otel.AttrSampleID+"=sid-9") {
		t.Fatalf("missing sample id: %s", got[EnvOTelResourceAttrs])
	}
}

func TestApplyOTelEnvEmptyURL(t *testing.T) {
	env := []string{"A=1"}
	if got := ApplyOTelEnv(env, "", "sid"); len(got) != 1 || got[0] != "A=1" {
		t.Fatalf("%v", got)
	}
}
