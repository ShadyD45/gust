// Package otel maps OpenTelemetry / OpenInference trace exports into gust AgentRun traces.
//
// The mapper consumes OTLP JSON (the shape produced by OTLP/HTTP exporters and
// `--experimental-otlp-json` file exporters) so ingestion requires no protobuf
// dependency and no network. Mapping is pure and deterministic: the same export
// file always produces the same AgentRun, including its content hash.
package otel

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gust/pkg/api"
)

// Mapping errors. Callers should surface these rather than inventing data:
// a partial or unrecognized trace must never be silently turned into a passing run.
var (
	ErrEmptyPayload   = errors.New("otel: export contains no spans")
	ErrInvalidPayload = errors.New("otel: malformed OTLP JSON payload")
	ErrTraceNotFound  = errors.New("otel: requested trace id not present in export")
	ErrAmbiguousTrace = errors.New("otel: export contains multiple traces; select one with --trace-id")
)

// OpenInference / OpenTelemetry attribute keys consumed by the mapper.
const (
	attrSpanKind      = "openinference.span.kind"
	attrToolName      = "tool.name"
	attrToolParams    = "tool.parameters"
	attrInputValue    = "input.value"
	attrOutputValue   = "output.value"
	attrSessionID     = "session.id"
	attrServiceName   = "service.name"
	attrServiceVer    = "service.version"
	attrServiceCommit = "service.git.commit"
	AttrSampleID      = "gust.sample_id"
)

// Options controls how a trace is projected onto an AgentRun.
// Every field is optional; defaults are derived from the trace itself.
type Options struct {
	TraceID      string // select one trace when the export holds several
	RunID        string // override the generated run id (defaults to the trace id)
	AgentName    string // override resource service.name
	AgentVersion string // override resource service.version
	TaskID       string // override the derived task id
	TaskInput    string // override the derived task input
}

// ExportPayload is the subset of the OTLP trace export schema gust needs.
type ExportPayload struct {
	ResourceSpans []ResourceSpans `json:"resourceSpans"`
}

type ResourceSpans struct {
	Resource   Resource     `json:"resource"`
	ScopeSpans []ScopeSpans `json:"scopeSpans"`
}

type Resource struct {
	Attributes []KeyValue `json:"attributes"`
}

type ScopeSpans struct {
	Spans []OTLPSpan `json:"spans"`
}

type OTLPSpan struct {
	TraceID           string      `json:"traceId"`
	SpanID            string      `json:"spanId"`
	ParentSpanID      string      `json:"parentSpanId"`
	Name              string      `json:"name"`
	StartTimeUnixNano json.Number `json:"startTimeUnixNano"`
	EndTimeUnixNano   json.Number `json:"endTimeUnixNano"`
	Attributes        []KeyValue  `json:"attributes"`
	Status            OTLPStatus  `json:"status"`
}

type OTLPStatus struct {
	Code    any    `json:"code"` // numeric (2) or string ("STATUS_CODE_ERROR")
	Message string `json:"message"`
}

type KeyValue struct {
	Key   string   `json:"key"`
	Value AnyValue `json:"value"`
}

// AnyValue mirrors the OTLP any-value union.
type AnyValue struct {
	StringValue *string      `json:"stringValue,omitempty"`
	BoolValue   *bool        `json:"boolValue,omitempty"`
	IntValue    *json.Number `json:"intValue,omitempty"`
	DoubleValue *float64     `json:"doubleValue,omitempty"`
	ArrayValue  *ArrayValue  `json:"arrayValue,omitempty"`
	KvlistValue *KvlistValue `json:"kvlistValue,omitempty"`
}

type ArrayValue struct {
	Values []AnyValue `json:"values"`
}

type KvlistValue struct {
	Values []KeyValue `json:"values"`
}

// Mapper converts OTLP JSON exports into AgentRun traces.
type Mapper struct{}

// NewMapper creates an OTel/OpenInference mapper.
func NewMapper() *Mapper { return &Mapper{} }

// MapBytes parses an OTLP JSON export and maps a single trace to an AgentRun.
func (m *Mapper) MapBytes(data []byte, opts Options) (api.AgentRun, error) {
	if len(data) == 0 {
		return api.AgentRun{}, ErrEmptyPayload
	}
	var payload ExportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return api.AgentRun{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	return m.Map(payload, opts)
}

// SampleIDFromPayload returns the first gust.sample_id attribute found on a
// resource or span. Empty if the export was not tagged by a Mode 3 runner.
func SampleIDFromPayload(payload ExportPayload) string {
	for _, rs := range payload.ResourceSpans {
		if id := stringAttr(attributeMap(rs.Resource.Attributes), AttrSampleID); id != "" {
			return id
		}
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				if id := stringAttr(attributeMap(sp.Attributes), AttrSampleID); id != "" {
					return id
				}
			}
		}
	}
	return ""
}

// TraceIDs lists every trace present in an export, sorted for stable output.
func TraceIDs(payload ExportPayload) []string {
	seen := make(map[string]struct{})
	for _, rs := range payload.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				seen[sp.TraceID] = struct{}{}
			}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// locatedSpan pairs a span with the resource that emitted it.
type locatedSpan struct {
	span     OTLPSpan
	resource Resource
}

// Map projects one trace from a parsed export onto an AgentRun.
func (m *Mapper) Map(payload ExportPayload, opts Options) (api.AgentRun, error) {
	var all []locatedSpan
	for _, rs := range payload.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				all = append(all, locatedSpan{span: sp, resource: rs.Resource})
			}
		}
	}
	if len(all) == 0 {
		return api.AgentRun{}, ErrEmptyPayload
	}

	traceID := opts.TraceID
	if traceID == "" {
		ids := TraceIDs(payload)
		if len(ids) > 1 {
			return api.AgentRun{}, fmt.Errorf("%w: found %s", ErrAmbiguousTrace, strings.Join(ids, ", "))
		}
		traceID = ids[0]
	}

	var selected []locatedSpan
	for _, l := range all {
		if l.span.TraceID == traceID {
			selected = append(selected, l)
		}
	}
	if len(selected) == 0 {
		return api.AgentRun{}, fmt.Errorf("%w: %s", ErrTraceNotFound, traceID)
	}

	// Deterministic ordering: chronological, with span id as tiebreaker so that
	// identical timestamps can never reorder between runs.
	sort.SliceStable(selected, func(i, j int) bool {
		si, sj := unixNano(selected[i].span.StartTimeUnixNano), unixNano(selected[j].span.StartTimeUnixNano)
		if si != sj {
			return si < sj
		}
		return selected[i].span.SpanID < selected[j].span.SpanID
	})

	resourceAttrs := attributeMap(selected[0].resource.Attributes)

	spans := make([]api.Span, 0, len(selected))
	for _, l := range selected {
		span, err := mapSpan(l.span)
		if err != nil {
			return api.AgentRun{}, err
		}
		spans = append(spans, span)
	}

	root := rootSpan(selected)
	rootAttrs := attributeMap(root.Attributes)

	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         firstNonEmpty(opts.RunID, traceID),
		Agent: api.AgentInfo{
			Name:      firstNonEmpty(opts.AgentName, stringAttr(resourceAttrs, attrServiceName), "unknown-agent"),
			Version:   firstNonEmpty(opts.AgentVersion, stringAttr(resourceAttrs, attrServiceVer), "unknown"),
			GitCommit: stringAttr(resourceAttrs, attrServiceCommit),
		},
		Task: api.TaskInfo{
			ID:    firstNonEmpty(opts.TaskID, stringAttr(rootAttrs, attrSessionID), traceID),
			Input: firstNonEmpty(opts.TaskInput, stringAttr(rootAttrs, attrInputValue), root.Name),
		},
		Trace:   spans,
		Outcome: mapOutcome(root, rootAttrs, selected),
		Metadata: map[string]any{
			"ingest": map[string]any{
				"source":   "otel",
				"trace_id": traceID,
			},
		},
	}

	if err := run.Validate(); err != nil {
		return api.AgentRun{}, fmt.Errorf("otel: mapped run is invalid: %w", err)
	}
	return run, nil
}

func mapSpan(sp OTLPSpan) (api.Span, error) {
	if sp.SpanID == "" {
		return api.Span{}, fmt.Errorf("%w: span is missing spanId", ErrInvalidPayload)
	}
	if sp.Name == "" {
		return api.Span{}, fmt.Errorf("%w: span %s is missing a name", ErrInvalidPayload, sp.SpanID)
	}

	attrs := attributeMap(sp.Attributes)
	span := api.Span{
		SpanID:       sp.SpanID,
		ParentSpanID: sp.ParentSpanID,
		Name:         spanName(sp, attrs),
		Type:         spanType(attrs),
		StartTime:    time.Unix(0, unixNano(sp.StartTimeUnixNano)).UTC(),
		EndTime:      time.Unix(0, unixNano(sp.EndTimeUnixNano)).UTC(),
		Status:       spanStatus(sp.Status),
		Attributes:   map[string]any{},
	}

	if in, ok := spanInput(attrs); ok {
		span.Attributes["input"] = in
	}
	if out, ok := decodeValue(attrs[attrOutputValue]); ok {
		span.Attributes["output"] = out
	}
	if len(span.Attributes) == 0 {
		span.Attributes = nil
	}
	return span, nil
}

// spanName prefers the OpenInference tool name so that tool assertions match on
// the logical tool rather than an instrumentation span label.
func spanName(sp OTLPSpan, attrs map[string]AnyValue) string {
	if name := stringAttr(attrs, attrToolName); name != "" {
		return name
	}
	return sp.Name
}

// spanType maps the OpenInference span kind onto the gust span taxonomy.
func spanType(attrs map[string]AnyValue) api.SpanType {
	switch strings.ToUpper(stringAttr(attrs, attrSpanKind)) {
	case "TOOL":
		return api.SpanTypeTool
	case "LLM":
		return api.SpanTypeLLM
	case "RETRIEVER":
		return api.SpanTypeRetrieval
	case "AGENT", "CHAIN":
		return api.SpanTypeAgent
	case "GUARDRAIL", "EVALUATOR", "RERANKER", "EMBEDDING":
		return api.SpanTypeAgent
	}
	// Untagged spans carrying tool attributes are still tool calls.
	if _, ok := attrs[attrToolName]; ok {
		return api.SpanTypeTool
	}
	if _, ok := attrs[attrToolParams]; ok {
		return api.SpanTypeTool
	}
	return api.SpanTypeAgent
}

// spanInput resolves tool arguments, which assertions compare against.
func spanInput(attrs map[string]AnyValue) (any, bool) {
	if v, ok := decodeValue(attrs[attrToolParams]); ok {
		return v, true
	}
	return decodeValue(attrs[attrInputValue])
}

func spanStatus(st OTLPStatus) api.SpanStatus {
	if isErrorStatus(st.Code) {
		return api.SpanStatus{Code: "error", Message: st.Message}
	}
	return api.SpanStatus{Code: "ok", Message: st.Message}
}

// isErrorStatus accepts both the numeric (2) and symbolic OTLP encodings.
func isErrorStatus(code any) bool {
	switch v := code.(type) {
	case string:
		return strings.EqualFold(v, "STATUS_CODE_ERROR") || v == "2"
	case float64:
		return v == 2
	case int:
		return v == 2
	case int32:
		return v == 2
	case json.Number:
		n, err := v.Int64()
		return err == nil && n == 2
	}
	return false
}

func mapOutcome(root OTLPSpan, rootAttrs map[string]AnyValue, spans []locatedSpan) api.RunOutcome {
	outcome := api.RunOutcome{Status: "completed"}

	if isErrorStatus(root.Status.Code) {
		outcome.Status = "failed"
		outcome.Error = root.Status.Message
	}
	if out := stringAttr(rootAttrs, attrOutputValue); out != "" {
		outcome.Output = out
	}

	// Wall-clock span of the whole trace, not just the root, so partial
	// instrumentation of the root does not understate duration.
	var first, last int64
	for i, s := range spans {
		start, end := unixNano(s.span.StartTimeUnixNano), unixNano(s.span.EndTimeUnixNano)
		if i == 0 || start < first {
			first = start
		}
		if end > last {
			last = end
		}
	}
	if last > first {
		outcome.DurationNs = time.Duration(last - first)
	}
	return outcome
}

// rootSpan returns the span without a parent inside the trace, falling back to
// the earliest span when instrumentation omitted the parent linkage.
func rootSpan(spans []locatedSpan) OTLPSpan {
	ids := make(map[string]struct{}, len(spans))
	for _, s := range spans {
		ids[s.span.SpanID] = struct{}{}
	}
	for _, s := range spans {
		if s.span.ParentSpanID == "" {
			return s.span
		}
		if _, ok := ids[s.span.ParentSpanID]; !ok {
			return s.span
		}
	}
	return spans[0].span
}

func attributeMap(kvs []KeyValue) map[string]AnyValue {
	m := make(map[string]AnyValue, len(kvs))
	for _, kv := range kvs {
		m[kv.Key] = kv.Value
	}
	return m
}

func stringAttr(attrs map[string]AnyValue, key string) string {
	v, ok := attrs[key]
	if !ok || v.StringValue == nil {
		return ""
	}
	return *v.StringValue
}

// decodeValue converts an OTLP any-value into plain Go data. JSON-encoded
// strings (how OpenInference carries tool parameters) are parsed so that
// argument assertions can compare structured values.
func decodeValue(v AnyValue) (any, bool) {
	switch {
	case v.StringValue != nil:
		s := *v.StringValue
		trimmed := strings.TrimSpace(s)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			var parsed any
			if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
				return parsed, true
			}
		}
		return s, true
	case v.BoolValue != nil:
		return *v.BoolValue, true
	case v.IntValue != nil:
		if n, err := v.IntValue.Float64(); err == nil {
			return n, true
		}
		return nil, false
	case v.DoubleValue != nil:
		return *v.DoubleValue, true
	case v.KvlistValue != nil:
		m := make(map[string]any, len(v.KvlistValue.Values))
		for _, kv := range v.KvlistValue.Values {
			if inner, ok := decodeValue(kv.Value); ok {
				m[kv.Key] = inner
			}
		}
		return m, true
	case v.ArrayValue != nil:
		list := make([]any, 0, len(v.ArrayValue.Values))
		for _, item := range v.ArrayValue.Values {
			if inner, ok := decodeValue(item); ok {
				list = append(list, inner)
			}
		}
		return list, true
	}
	return nil, false
}

func unixNano(n json.Number) int64 {
	if n == "" {
		return 0
	}
	if i, err := n.Int64(); err == nil {
		return i
	}
	// Some exporters emit nanosecond timestamps as quoted floats.
	if f, err := strconv.ParseFloat(string(n), 64); err == nil {
		return int64(f)
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
