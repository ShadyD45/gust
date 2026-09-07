package evaluators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

// JudgePanelEvaluator aggregates multiple llm_judge (or aliased judge) evaluators.
type JudgePanelEvaluator struct{}

func (e *JudgePanelEvaluator) Name() string    { return "judge_panel" }
func (e *JudgePanelEvaluator) Version() string { return "1.0.0" }

type panelMember struct {
	Name   string
	Alias  string
	Rubric string
	Model  string
}

func (e *JudgePanelEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{
		EvaluatorName:    e.Name(),
		EvaluatorVersion: e.Version(),
	}
	if !allowLLMJudge(evalCtx) {
		res.Passed = false
		res.Message = "judge_panel is disabled: set policy allow_llm_judge: true"
		res.Evidence = map[string]any{"allow_llm_judge": false}
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	members, agg, threshold, sharedRubric, errMsg := parsePanelParams(expected)
	if errMsg != "" {
		res.Passed = false
		res.Message = errMsg
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}

	peers := peerEvaluators(evalCtx)
	votes := make([]map[string]any, 0, len(members))
	aliases := map[string]struct{}{}
	var scores []float64
	passCount := 0
	failCount := 0

	for _, m := range members {
		label := m.Alias
		if label == "" {
			label = m.Name
		}
		if _, dup := aliases[label]; dup {
			res.Passed = false
			res.Message = fmt.Sprintf("judge_panel duplicate alias %q", label)
			res.ExecutionTimeNs = time.Since(start).Nanoseconds()
			return res, nil
		}
		aliases[label] = struct{}{}
		if m.Name == "judge_panel" || label == "judge_panel" {
			res.Passed = false
			res.Message = "judge_panel cannot nest another judge_panel"
			res.ExecutionTimeNs = time.Since(start).Nanoseconds()
			return res, nil
		}
		ev, ok := peers[m.Name]
		if !ok {
			res.Passed = false
			res.Message = fmt.Sprintf("judge_panel unknown member %q", m.Name)
			res.Evidence = map[string]any{"missing": m.Name}
			res.ExecutionTimeNs = time.Since(start).Nanoseconds()
			return res, nil
		}
		if ev.Name() == "judge_panel" {
			res.Passed = false
			res.Message = "judge_panel cannot nest another judge_panel"
			res.ExecutionTimeNs = time.Since(start).Nanoseconds()
			return res, nil
		}

		memberAssert := &api.Assertion{
			ID:         label,
			Type:       api.AssertLLMJudge,
			Parameters: map[string]any{},
		}
		if expected != nil && expected.Parameters != nil {
			for k, v := range expected.Parameters {
				if k == "judges" || k == "aggregation" {
					continue
				}
				memberAssert.Parameters[k] = v
			}
		}
		rubric := m.Rubric
		if rubric == "" {
			rubric = sharedRubric
		}
		if rubric != "" {
			memberAssert.Parameters["rubric"] = rubric
		}
		if m.Model != "" {
			memberAssert.Parameters["model"] = m.Model
		}

		jr, err := ev.Evaluate(ctx, run, memberAssert, evalCtx)
		vote := map[string]any{
			"name":    m.Name,
			"alias":   label,
			"error":   "",
			"passed":  false,
			"score":   0.0,
			"model":   "",
			"message": "",
		}
		if err != nil {
			vote["error"] = err.Error()
			failCount++
			votes = append(votes, vote)
			continue
		}
		vote["passed"] = jr.Passed
		vote["score"] = jr.Score
		vote["message"] = jr.Message
		if jr.Evidence != nil {
			if model, ok := jr.Evidence["model"]; ok {
				vote["model"] = model
			}
			if r, ok := jr.Evidence["rationale"]; ok {
				vote["rationale"] = r
			}
		}
		scores = append(scores, jr.Score)
		if jr.Passed {
			passCount++
		} else {
			failCount++
		}
		votes = append(votes, vote)
	}

	mean := 0.0
	spread := 0.0
	if len(scores) > 0 {
		sum := 0.0
		minS, maxS := scores[0], scores[0]
		for _, s := range scores {
			sum += s
			if s < minS {
				minS = s
			}
			if s > maxS {
				maxS = s
			}
		}
		mean = sum / float64(len(scores))
		spread = maxS - minS
	}

	passed := false
	switch agg {
	case "all":
		passed = failCount == 0 && passCount == len(members)
	case "any":
		passed = passCount > 0
	case "mean_score":
		passed = mean >= threshold
	default: // majority
		passed = passCount > failCount
	}

	disagreement := passCount > 0 && failCount > 0
	calibrated := boolFromConfig(evalCtx, "llm_judge_calibrated")
	res.Passed = passed
	res.Score = mean
	res.Message = fmt.Sprintf("judge_panel %s: %d passed, %d failed, mean=%.2f", agg, passCount, failCount, mean)
	res.Evidence = map[string]any{
		"aggregation":  agg,
		"threshold":    threshold,
		"votes":        votes,
		"pass_count":   passCount,
		"fail_count":   failCount,
		"mean_score":   mean,
		"score_spread": spread,
		"disagreement": disagreement,
		"calibrated":   calibrated,
		"decision":     passed,
	}
	if disagreement {
		res.Evidence["note"] = "panel members disagreed; inspect votes before treating this as ground truth"
	}
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

func parsePanelParams(expected *api.Assertion) (members []panelMember, agg string, threshold float64, rubric string, errMsg string) {
	agg = "majority"
	threshold = 0.7
	if expected == nil || expected.Parameters == nil {
		return nil, agg, threshold, "", "judge_panel requires parameters.judges"
	}
	if v, ok := expected.Parameters["aggregation"].(string); ok && v != "" {
		agg = strings.ToLower(v)
	}
	switch agg {
	case "majority", "all", "any", "mean_score":
	default:
		return nil, agg, threshold, "", fmt.Sprintf("unknown aggregation %q", agg)
	}
	if v, ok := expected.Parameters["threshold"].(float64); ok {
		threshold = v
	}
	if v, ok := expected.Parameters["rubric"].(string); ok {
		rubric = v
	} else if v, ok := expected.Parameters["prompt"].(string); ok {
		rubric = v
	}
	raw, ok := expected.Parameters["judges"]
	if !ok {
		return nil, agg, threshold, rubric, "judge_panel requires parameters.judges"
	}
	members = parseJudges(raw)
	if len(members) == 0 {
		return nil, agg, threshold, rubric, "judge_panel requires at least one judge"
	}
	return members, agg, threshold, rubric, ""
}

func parseJudges(raw any) []panelMember {
	switch v := raw.(type) {
	case []string:
		out := make([]panelMember, 0, len(v))
		for _, name := range v {
			if strings.TrimSpace(name) != "" {
				out = append(out, panelMember{Name: name})
			}
		}
		return out
	case []any:
		out := make([]panelMember, 0, len(v))
		for _, item := range v {
			switch t := item.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					out = append(out, panelMember{Name: t})
				}
			case map[string]any:
				m := panelMember{}
				if n, ok := t["name"].(string); ok {
					m.Name = n
				}
				if n, ok := t["alias"].(string); ok {
					m.Alias = n
				}
				if n, ok := t["rubric"].(string); ok {
					m.Rubric = n
				}
				if n, ok := t["model"].(string); ok {
					m.Model = n
				}
				if m.Name != "" {
					out = append(out, m)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func peerEvaluators(evalCtx ports.EvaluationContext) map[string]ports.Evaluator {
	if evalCtx.Config == nil {
		return nil
	}
	raw, ok := evalCtx.Config["peer_evaluators"]
	if !ok {
		return nil
	}
	if m, ok := raw.(map[string]ports.Evaluator); ok {
		return m
	}
	return nil
}
