package fixtures

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"gust/internal/ports"
	"gust/pkg/api"
)

func TestMemoryFixtureProvider(t *testing.T) {
	provider := NewMemoryFixtureProvider()

	// Load an exact fixture and sequential fixtures
	fixtures := []api.Fixture{
		{
			FixtureID: "fx_exact",
			Tool:      "get_user",
			RecordedInput: map[string]any{
				"user_id": 42,
			},
			RecordedResponse: api.RecordedResponse{
				Status: "success",
				Body:   map[string]any{"name": "Alice"},
			},
			Provenance: api.ProvenanceRecorded,
		},
		{
			FixtureID:     "fx_poll_1",
			Tool:          "poll_status",
			MatchStrategy: api.MatchStrategyOrderedSequence,
			RecordedResponse: api.RecordedResponse{
				Status: "success",
				Body:   "PENDING",
			},
			Provenance: api.ProvenanceRecorded,
		},
		{
			FixtureID:     "fx_poll_2",
			Tool:          "poll_status",
			MatchStrategy: api.MatchStrategyOrderedSequence,
			RecordedResponse: api.RecordedResponse{
				Status: "success",
				Body:   "COMPLETED",
			},
			Provenance: api.ProvenanceRecorded,
		},
	}

	if err := provider.LoadFixtures(fixtures); err != nil {
		t.Fatalf("LoadFixtures failed: %v", err)
	}

	ctx := context.Background()

	// 1. Test Exact Lookup
	callExact := ports.ToolCall{
		Name:      "get_user",
		Arguments: map[string]any{"user_id": 42},
	}
	resp, found, err := provider.Lookup(ctx, callExact)
	if err != nil || !found {
		t.Fatalf("Lookup exact failed: found=%v, err=%v", found, err)
	}
	bodyMap, _ := resp.Body.(map[string]any)
	if bodyMap["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", resp.Body)
	}

	// 2. Test Sequential FIFO
	callPoll := ports.ToolCall{Name: "poll_status"}

	// First call -> PENDING
	resp1, found, err := provider.Lookup(ctx, callPoll)
	if err != nil || !found || resp1.Body != "PENDING" {
		t.Errorf("expected PENDING on call 1, got %v", resp1.Body)
	}

	// Second call -> COMPLETED
	resp2, found, err := provider.Lookup(ctx, callPoll)
	if err != nil || !found || resp2.Body != "COMPLETED" {
		t.Errorf("expected COMPLETED on call 2, got %v", resp2.Body)
	}

	// 3. Reset and call again -> resets back to PENDING
	_ = provider.Reset()
	respReset, _, _ := provider.Lookup(ctx, callPoll)
	if respReset.Body != "PENDING" {
		t.Errorf("expected PENDING after reset, got %v", respReset.Body)
	}
}

func TestMockToolProxyServer(t *testing.T) {
	provider := NewMemoryFixtureProvider()
	_ = provider.LoadFixtures([]api.Fixture{
		{
			FixtureID: "fx_server_test",
			Tool:      "echo",
			RecordedInput: map[string]any{
				"msg": "hello",
			},
			RecordedResponse: api.RecordedResponse{
				Status: "success",
				Body:   map[string]any{"reply": "world"},
			},
			Provenance: api.ProvenanceRecorded,
		},
	})

	proxy, err := NewMockToolProxyServer(provider)
	if err != nil {
		t.Fatalf("NewMockToolProxyServer failed: %v", err)
	}
	endpoint := proxy.Start()
	defer proxy.Close()

	// Verify health check
	resp, err := http.Get(endpoint + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("health check failed: %v", err)
	}

	// Make tool call POST
	reqBody, _ := json.Marshal(ToolCallRequest{
		Tool:      "echo",
		Arguments: map[string]any{"msg": "hello"},
	})

	postResp, err := http.Post(endpoint+"/v1/tools/call", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("tool call POST failed: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", postResp.StatusCode)
	}

	var callResp ToolCallResponse
	if err := json.NewDecoder(postResp.Body).Decode(&callResp); err != nil {
		t.Fatalf("decode tool call response failed: %v", err)
	}

	resMap, _ := callResp.Body.(map[string]any)
	if resMap["reply"] != "world" {
		t.Errorf("expected world reply, got %v", callResp.Body)
	}
}
