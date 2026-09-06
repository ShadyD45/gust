package otel

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	colltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"gust/pkg/api"
)

func taggedPayload(t *testing.T, sampleID string) []byte {
	t.Helper()
	var payload ExportPayload
	if err := json.Unmarshal(goldenExport(t), &payload); err != nil {
		t.Fatal(err)
	}
	sid := sampleID
	payload.ResourceSpans[0].Resource.Attributes = append(payload.ResourceSpans[0].Resource.Attributes, KeyValue{
		Key:   AttrSampleID,
		Value: AnyValue{StringValue: &sid},
	})
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestReceiver_HTTPJSONWait(t *testing.T) {
	recv, err := NewReceiver(ReceiverConfig{Listen: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := recv.Start(); err != nil {
		t.Fatal(err)
	}
	defer recv.Close()

	sid := "sample-http-1"
	var wg sync.WaitGroup
	wg.Add(1)
	var gotRunID string
	var waitErr error
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		run, err := recv.Wait(ctx, sid)
		waitErr = err
		if err == nil {
			gotRunID = run.RunID
		}
	}()

	time.Sleep(20 * time.Millisecond)
	resp, err := http.Post(recv.Endpoint()+"/v1/traces", "application/json", bytes.NewReader(taggedPayload(t, sid)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	wg.Wait()
	if waitErr != nil {
		t.Fatal(waitErr)
	}
	if gotRunID != sid {
		t.Fatalf("run id %s want sample id", gotRunID)
	}
}

func TestPayloadFromProtobufRoundTrip(t *testing.T) {
	var jsonPayload ExportPayload
	if err := json.Unmarshal(goldenExport(t), &jsonPayload); err != nil {
		t.Fatal(err)
	}
	runJSON, err := NewMapper().Map(jsonPayload, Options{})
	if err != nil {
		t.Fatal(err)
	}

	req := &colltrace.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
				strKV("service.name", "support-agent"),
				strKV("service.version", "1.4"),
			}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Spans: []*tracepb.Span{{
					TraceId:           []byte{0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6, 0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36},
					SpanId:            []byte{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7},
					Name:              "AgentExecutor",
					StartTimeUnixNano: 1767225600000000000,
					EndTimeUnixNano:   1767225600030000000,
					Attributes: []*commonpb.KeyValue{
						strKV("openinference.span.kind", "AGENT"),
						strKV("session.id", "refund-001"),
						strKV("input.value", "Cancel my latest order"),
						strKV("output.value", "Order 123 cancelled successfully."),
					},
				}},
			}},
		}},
	}
	payload := payloadFromProtoRequest(req)
	run, err := NewMapper().Map(payload, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Agent.Name != runJSON.Agent.Name {
		t.Fatalf("agent name %s", run.Agent.Name)
	}
	if run.Task.Input != "Cancel my latest order" {
		t.Fatalf("task input %s", run.Task.Input)
	}
}

func strKV(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func TestReceiver_WaitUntaggedWhenSingleWaiter(t *testing.T) {
	recv, err := NewReceiver(ReceiverConfig{Listen: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := recv.Start(); err != nil {
		t.Fatal(err)
	}
	defer recv.Close()

	sid := "untagged-1"
	var wg sync.WaitGroup
	wg.Add(1)
	var waitErr error
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, waitErr = recv.Wait(ctx, sid)
	}()
	time.Sleep(20 * time.Millisecond)
	resp, err := http.Post(recv.Endpoint()+"/v1/traces", "application/json", bytes.NewReader(goldenExport(t)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	wg.Wait()
	if waitErr != nil {
		t.Fatal(waitErr)
	}
}

func TestReceiver_IngestRunHTTP(t *testing.T) {
	recv, err := NewReceiver(ReceiverConfig{Listen: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := recv.Start(); err != nil {
		t.Fatal(err)
	}
	defer recv.Close()

	now := time.Now().UTC()
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "run-export-1",
		Agent:         api.AgentInfo{Name: "agent", Version: "1"},
		Task:          api.TaskInfo{ID: "t", Input: "do it"},
		Trace: []api.Span{{
			SpanID:    "s1",
			Name:      "step",
			Type:      api.SpanTypeAgent,
			StartTime: now,
			EndTime:   now.Add(time.Millisecond),
			Status:    api.SpanStatus{Code: "ok"},
		}},
		Outcome:  api.RunOutcome{Status: "completed", Output: "ok"},
		Metadata: map[string]any{"sample_id": "sid-export"},
	}
	body, _ := json.Marshal(run)
	resp, err := http.Post(recv.Endpoint()+"/v1/runs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d", resp.StatusCode)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := recv.Wait(ctx, "sid-export")
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != "run-export-1" {
		t.Fatalf("run id %s", got.RunID)
	}
}

func TestReceiver_WaitTimeout(t *testing.T) {
	recv, err := NewReceiver(ReceiverConfig{Listen: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := recv.Start(); err != nil {
		t.Fatal(err)
	}
	defer recv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = recv.Wait(ctx, "never-arrives")
	if err == nil {
		t.Fatal("expected timeout")
	}
}

func TestReceiver_ConcurrentSamples(t *testing.T) {
	recv, err := NewReceiver(ReceiverConfig{Listen: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := recv.Start(); err != nil {
		t.Fatal(err)
	}
	defer recv.Close()

	ids := []string{"a", "b", "c"}
	var wg sync.WaitGroup
	errs := make([]error, len(ids))
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, errs[i] = recv.Wait(ctx, id)
		}(i, id)
	}
	time.Sleep(20 * time.Millisecond)
	for _, id := range ids {
		resp, err := http.Post(recv.Endpoint()+"/v1/traces", "application/json", bytes.NewReader(taggedPayload(t, id)))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("sample %d: %v", i, err)
		}
	}
}
