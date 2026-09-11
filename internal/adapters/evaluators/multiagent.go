package evaluators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

// agentNames returns ordered agent span names (or agent.name attributes).
func agentNames(run api.AgentRun) []string {
	out := make([]string, 0)
	for _, sp := range run.Trace {
		if sp.Type != api.SpanTypeAgent {
			continue
		}
		name := sp.Name
		if v, ok := sp.Attributes["agent.name"].(string); ok && v != "" {
			name = v
		} else if v, ok := sp.Attributes["role"].(string); ok && v != "" {
			name = v
		}
		out = append(out, name)
	}
	if len(out) == 0 && run.Agent.Name != "" {
		out = append(out, run.Agent.Name)
	}
	return out
}

// AgentHandoffEvaluator checks that control passed from one agent/role to another.
type AgentHandoffEvaluator struct{}

func (e *AgentHandoffEvaluator) Name() string    { return "agent_handoff" }
func (e *AgentHandoffEvaluator) Version() string { return "1.0.0" }
func (e *AgentHandoffEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{EvaluatorName: e.Name(), EvaluatorVersion: e.Version()}
	from := ""
	to := ""
	if expected != nil && expected.Parameters != nil {
		from, _ = expected.Parameters["from"].(string)
		to, _ = expected.Parameters["to"].(string)
	}
	names := agentNames(run)
	res.Evidence = map[string]any{"agents": names, "from": from, "to": to}
	if from == "" || to == "" {
		res.Passed = false
		res.Message = "agent_handoff requires parameters.from and parameters.to"
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}
	foundFrom, foundToAfter := false, false
	for _, n := range names {
		if !foundFrom && n == from {
			foundFrom = true
			continue
		}
		if foundFrom && n == to {
			foundToAfter = true
			break
		}
	}
	res.Passed = foundFrom && foundToAfter
	if res.Passed {
		res.Score = 1
		res.Message = fmt.Sprintf("handoff %s → %s observed", from, to)
	} else {
		res.Score = 0
		res.Message = fmt.Sprintf("handoff %s → %s not found in agent sequence %v", from, to, names)
	}
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// RoleAdherenceEvaluator checks that an expected role/agent participated.
type RoleAdherenceEvaluator struct{}

func (e *RoleAdherenceEvaluator) Name() string    { return "role_adherence" }
func (e *RoleAdherenceEvaluator) Version() string { return "1.0.0" }
func (e *RoleAdherenceEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{EvaluatorName: e.Name(), EvaluatorVersion: e.Version()}
	role := ""
	if expected != nil {
		if expected.Tool != "" {
			role = expected.Tool
		}
		if expected.Parameters != nil {
			if r, ok := expected.Parameters["role"].(string); ok && r != "" {
				role = r
			}
			if r, ok := expected.Parameters["agent"].(string); ok && r != "" {
				role = r
			}
		}
	}
	names := agentNames(run)
	res.Evidence = map[string]any{"agents": names, "expected_role": role}
	if role == "" {
		res.Passed = false
		res.Message = "role_adherence requires tool or parameters.role"
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}
	for _, n := range names {
		if strings.EqualFold(n, role) {
			res.Passed = true
			res.Score = 1
			res.Message = fmt.Sprintf("role %q participated", role)
			res.ExecutionTimeNs = time.Since(start).Nanoseconds()
			return res, nil
		}
	}
	res.Passed = false
	res.Score = 0
	res.Message = fmt.Sprintf("role %q not found in %v", role, names)
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}

// CoordinationOrderEvaluator checks an exact or subsequence of agent names.
type CoordinationOrderEvaluator struct{}

func (e *CoordinationOrderEvaluator) Name() string    { return "coordination_order" }
func (e *CoordinationOrderEvaluator) Version() string { return "1.0.0" }
func (e *CoordinationOrderEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	start := time.Now()
	res := ports.EvaluationResult{EvaluatorName: e.Name(), EvaluatorVersion: e.Version()}
	var seq []string
	match := "subsequence"
	if expected != nil && expected.Parameters != nil {
		if raw, ok := expected.Parameters["sequence"].([]any); ok {
			for _, v := range raw {
				if s, ok := v.(string); ok {
					seq = append(seq, s)
				}
			}
		}
		if s, ok := expected.Parameters["match"].(string); ok && s != "" {
			match = s
		}
	}
	names := agentNames(run)
	res.Evidence = map[string]any{"agents": names, "expected": seq, "match": match}
	if len(seq) == 0 {
		res.Passed = false
		res.Message = "coordination_order requires parameters.sequence"
		res.ExecutionTimeNs = time.Since(start).Nanoseconds()
		return res, nil
	}
	ok := false
	switch match {
	case "exact":
		ok = len(names) == len(seq)
		if ok {
			for i := range seq {
				if names[i] != seq[i] {
					ok = false
					break
				}
			}
		}
	default:
		j := 0
		for _, n := range names {
			if j < len(seq) && n == seq[j] {
				j++
			}
		}
		ok = j == len(seq)
	}
	res.Passed = ok
	if ok {
		res.Score = 1
		res.Message = "coordination order satisfied"
	} else {
		res.Score = 0
		res.Message = fmt.Sprintf("coordination order %v not satisfied by %v", seq, names)
	}
	res.ExecutionTimeNs = time.Since(start).Nanoseconds()
	return res, nil
}
