package testrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

var _ ports.TestRunner = (*FixtureProbeRunner)(nil)

// FixtureProbeRunner POSTs a scripted sequence of tool calls to fixtureEndpoint.
// Used by Mode 3 isolation tests — not a live agent.
type FixtureProbeRunner struct {
	Calls []ports.ToolCall
	// ExtraCalls repeats the last call this many additional times (exhaustion tests).
	ExtraCalls int
	// FailFirst returns ports.ErrTransient on the first Run per fixture endpoint (retry test).
	FailFirst bool
	// ConsumeThenFail, with FailFirst, issues the first tool call before failing so retry must Reset.
	ConsumeThenFail bool
	failed          sync.Map
}

func (r *FixtureProbeRunner) Name() string { return "fixture_probe" }

func (r *FixtureProbeRunner) Run(ctx context.Context, req ports.SampleRequest) (api.AgentRun, error) {
	fixtureEndpoint := req.FixtureEndpointOrEmpty()
	if r.FailFirst {
		if _, loaded := r.failed.LoadOrStore(fixtureEndpoint, true); !loaded {
			if r.ConsumeThenFail && len(r.Calls) > 0 {
				_, _, _ = postFixtureTool(ctx, fixtureEndpoint, r.Calls[0])
			}
			return api.AgentRun{}, fmt.Errorf("%w: simulated blip", ports.ErrTransient)
		}
	}
	if fixtureEndpoint == "" {
		return api.AgentRun{}, fmt.Errorf("fixture_probe: fixture endpoint is required")
	}

	calls := append([]ports.ToolCall(nil), r.Calls...)
	for i := 0; i < r.ExtraCalls; i++ {
		if len(r.Calls) == 0 {
			break
		}
		calls = append(calls, r.Calls[len(r.Calls)-1])
	}

	now := time.Now().UTC()
	spans := make([]api.Span, 0, len(calls)+1)
	spans = append(spans, api.Span{
		SpanID:    "agent",
		Name:      "probe",
		Type:      api.SpanTypeAgent,
		StartTime: now,
		EndTime:   now,
		Status:    api.SpanStatus{Code: "ok"},
	})
	bodies := make([]string, 0, len(calls))
	allFound := true
	for i, call := range calls {
		resp, found, err := postFixtureTool(ctx, fixtureEndpoint, call)
		if err != nil {
			return api.AgentRun{}, err
		}
		code := "ok"
		if !found {
			code = "error"
			allFound = false
		}
		end := time.Now().UTC()
		spans = append(spans, api.Span{
			SpanID:    fmt.Sprintf("tool_%d", i),
			Name:      call.Name,
			Type:      api.SpanTypeTool,
			StartTime: now,
			EndTime:   end,
			Status:    api.SpanStatus{Code: code},
			Attributes: map[string]any{
				"body":  resp,
				"found": found,
			},
		})
		bodies = append(bodies, resp)
		now = end
	}

	status := "completed"
	if !allFound {
		status = "failed"
	}
	runID := req.SampleID
	if runID == "" {
		runID = fmt.Sprintf("probe_%s_%d", req.Scenario.ID, time.Now().UnixNano())
	}
	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         runID,
		Agent:         api.AgentInfo{Name: "fixture-probe", Version: "1.0"},
		Task:          req.Scenario.Task,
		Trace:         spans,
		Outcome: api.RunOutcome{
			Status:     status,
			Output:     fmt.Sprintf("%v", bodies),
			DurationNs: time.Millisecond,
		},
		Metadata: map[string]any{
			"fixture_endpoint": fixtureEndpoint,
			"bodies":           bodies,
			"all_found":        allFound,
		},
	}
	stampSampleMetadata(&run, req)
	return run, nil
}

type toolCallJSON struct {
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
}

type toolRespJSON struct {
	Status string `json:"status"`
	Body   any    `json:"body"`
	Error  string `json:"error"`
}

func postFixtureTool(ctx context.Context, endpoint string, call ports.ToolCall) (body string, found bool, err error) {
	payload, err := json.Marshal(toolCallJSON{Tool: call.Name, Arguments: call.Arguments})
	if err != nil {
		return "", false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/v1/tools/call", bytes.NewReader(payload))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	var parsed toolRespJSON
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", false, err
	}
	if resp.StatusCode == http.StatusNotFound || parsed.Error != "" && parsed.Status == "error" {
		return fmt.Sprint(parsed.Error), false, nil
	}
	return fmt.Sprint(parsed.Body), true, nil
}
