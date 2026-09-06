package judge

import (
	"fmt"
	"time"

	"gust/pkg/api"
)

// SeedDataset returns ≥50 synthetic human-labeled calibration cases for local
// development and CI (mock judge alignment). Real human labels should replace
// these before claiming production calibration.
func SeedDataset() *CalibrationDataset {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := make([]CalibrationCase, 0, 50)
	for i := 0; i < 50; i++ {
		// Alternate quality bands so ranks vary.
		band := i % 5
		var status, output string
		var human float64
		switch band {
		case 0:
			status, output, human = "failed", "", 0.1
		case 1:
			status, output, human = "completed", "partial answer only", 0.35
		case 2:
			status, output, human = "completed", "order noted", 0.55
		case 3:
			status, output, human = "completed", "Order cancelled successfully for the customer.", 0.8
		default:
			status, output, human = "completed", "Order cancelled successfully with confirmation id and polite closing.", 0.95
		}
		// Slight monotonic jitter so ties are limited.
		human += float64(i%3) * 0.01
		if human > 1 {
			human = 1
		}
		cases = append(cases, CalibrationCase{
			ID:         fmt.Sprintf("cal_%03d", i+1),
			Rubric:     "The agent should clearly confirm the order cancellation in a helpful tone.",
			HumanScore: human,
			Threshold:  0.7,
			Run: api.AgentRun{
				SchemaVersion: api.SchemaVersion,
				RunID:         fmt.Sprintf("cal_run_%03d", i+1),
				Agent:         api.AgentInfo{Name: "support", Version: "1.0"},
				Task:          api.TaskInfo{ID: "cancel", Input: fmt.Sprintf("cancel order %d", 1000+i)},
				Trace: []api.Span{{
					SpanID:    "s1",
					Name:      "cancel_order",
					Type:      api.SpanTypeTool,
					StartTime: now,
					EndTime:   now.Add(time.Millisecond),
					Status:    api.SpanStatus{Code: map[bool]string{true: "ok", false: "error"}[status == "completed"]},
				}},
				Outcome: api.RunOutcome{Status: status, Output: output},
			},
		})
	}
	return &CalibrationDataset{
		Version:     "1.0",
		Description: "Synthetic seed calibration set (≥50). Replace with human labels before production enablement.",
		Cases:       cases,
	}
}
