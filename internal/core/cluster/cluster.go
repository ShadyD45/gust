package cluster

import (
	"fmt"
	"sort"
	"strings"

	"gust/pkg/api"
	"gust/pkg/jcs"
)

// Features are deterministic failure fingerprints extracted from an AgentRun.
type Features struct {
	ToolSequence  []string `json:"tool_sequence"`
	ErrorCodes    []string `json:"error_codes"`
	ArgSchemaHash string   `json:"arg_schema_hash"`
	OutcomeStatus string   `json:"outcome_status"`
	Fingerprint   string   `json:"fingerprint"`
}

// Extract builds a fingerprint from tool names, error codes, and arg schema shapes.
func Extract(run api.AgentRun) (Features, error) {
	f := Features{
		ToolSequence:  make([]string, 0),
		ErrorCodes:    make([]string, 0),
		OutcomeStatus: run.Outcome.Status,
	}
	argShapes := make([]map[string]string, 0)
	for _, span := range run.Trace {
		if span.Type == api.SpanTypeTool {
			f.ToolSequence = append(f.ToolSequence, span.Name)
			shape := argShape(span.Attributes["input"])
			argShapes = append(argShapes, shape)
		}
		if span.Status.Code != "" && span.Status.Code != "ok" {
			code := span.Status.Code
			if span.Status.Message != "" {
				code = code + ":" + normalizeErr(span.Status.Message)
			}
			f.ErrorCodes = append(f.ErrorCodes, code)
		}
	}
	if run.Outcome.Error != "" {
		f.ErrorCodes = append(f.ErrorCodes, "outcome:"+normalizeErr(run.Outcome.Error))
	}
	sort.Strings(f.ErrorCodes)
	hash, err := jcs.ContentHash(map[string]any{
		"tools":  f.ToolSequence,
		"errors": f.ErrorCodes,
		"args":   argShapes,
		"out":    f.OutcomeStatus,
	})
	if err != nil {
		return f, err
	}
	f.ArgSchemaHash = hash
	f.Fingerprint = hash
	return f, nil
}

func argShape(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out[k] = typeName(m[k])
	}
	return out
}

func typeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64, float32, int, int32, int64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func normalizeErr(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

// Member is one run assigned to a cluster.
type Member struct {
	RunID       string   `json:"run_id"`
	Fingerprint string   `json:"fingerprint"`
	Features    Features `json:"features"`
}

// Cluster groups near-duplicate failures.
type Cluster struct {
	ID             string   `json:"id"`
	Fingerprint    string   `json:"fingerprint"`
	Size           int      `json:"size"`
	Representative string   `json:"representative_run_id"`
	Members        []Member `json:"members"`
}

// Report is an offline clustering result.
type Report struct {
	Clusters  []Cluster `json:"clusters"`
	TotalRuns int       `json:"total_runs"`
}

// ClusterRuns groups runs by exact fingerprint equality.
func ClusterRuns(runs []api.AgentRun) (Report, error) {
	groups := map[string]*Cluster{}
	order := make([]string, 0)
	for _, run := range runs {
		feat, err := Extract(run)
		if err != nil {
			return Report{}, err
		}
		fp := feat.Fingerprint
		c, ok := groups[fp]
		if !ok {
			id := fmt.Sprintf("cluster_%s", short(fp))
			c = &Cluster{
				ID:             id,
				Fingerprint:    fp,
				Representative: run.RunID,
			}
			groups[fp] = c
			order = append(order, fp)
		}
		c.Members = append(c.Members, Member{RunID: run.RunID, Fingerprint: fp, Features: feat})
		c.Size = len(c.Members)
	}
	rep := Report{TotalRuns: len(runs), Clusters: make([]Cluster, 0, len(order))}
	for _, fp := range order {
		rep.Clusters = append(rep.Clusters, *groups[fp])
	}
	sort.Slice(rep.Clusters, func(i, j int) bool {
		if rep.Clusters[i].Size != rep.Clusters[j].Size {
			return rep.Clusters[i].Size > rep.Clusters[j].Size
		}
		return rep.Clusters[i].ID < rep.Clusters[j].ID
	})
	return rep, nil
}

func short(fp string) string {
	fp = strings.TrimPrefix(fp, "sha256:")
	if len(fp) > 12 {
		return fp[:12]
	}
	return fp
}

// AttachClusterID records the cluster on scenario provenance.
func AttachClusterID(sc *api.TestScenario, clusterID string) {
	if sc == nil || clusterID == "" {
		return
	}
	sc.Provenance.ClusterID = clusterID
}

// Representatives returns one AgentRun per cluster (first member match).
func Representatives(runs []api.AgentRun, report Report) []api.AgentRun {
	byID := map[string]api.AgentRun{}
	for _, r := range runs {
		byID[r.RunID] = r
	}
	out := make([]api.AgentRun, 0, len(report.Clusters))
	for _, c := range report.Clusters {
		if r, ok := byID[c.Representative]; ok {
			out = append(out, r)
		}
	}
	return out
}
