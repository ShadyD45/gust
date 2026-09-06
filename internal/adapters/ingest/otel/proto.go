package otel

import (
	"encoding/json"
	"fmt"
	"strconv"

	colltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// payloadFromProtobuf unmarshals an OTLP ExportTraceServiceRequest into the
// same ExportPayload the JSON mapper already understands.
func payloadFromProtobuf(data []byte) (ExportPayload, error) {
	var req colltrace.ExportTraceServiceRequest
	if err := proto.Unmarshal(data, &req); err != nil {
		return ExportPayload{}, fmt.Errorf("%w: protobuf: %v", ErrInvalidPayload, err)
	}
	return payloadFromProtoRequest(&req), nil
}

func payloadFromProtoRequest(req *colltrace.ExportTraceServiceRequest) ExportPayload {
	out := ExportPayload{ResourceSpans: make([]ResourceSpans, 0, len(req.ResourceSpans))}
	for _, rs := range req.ResourceSpans {
		converted := ResourceSpans{}
		if rs.Resource != nil {
			converted.Resource.Attributes = protoAttrs(rs.Resource.Attributes)
		}
		for _, ss := range rs.ScopeSpans {
			scope := ScopeSpans{Spans: make([]OTLPSpan, 0, len(ss.Spans))}
			for _, sp := range ss.Spans {
				if sp == nil {
					continue
				}
				scope.Spans = append(scope.Spans, OTLPSpan{
					TraceID:           hexID(sp.TraceId),
					SpanID:            hexID(sp.SpanId),
					ParentSpanID:      hexID(sp.ParentSpanId),
					Name:              sp.Name,
					StartTimeUnixNano: json.Number(strconv.FormatUint(sp.StartTimeUnixNano, 10)),
					EndTimeUnixNano:   json.Number(strconv.FormatUint(sp.EndTimeUnixNano, 10)),
					Attributes:        protoAttrs(sp.Attributes),
					Status:            protoStatus(sp.Status),
				})
			}
			converted.ScopeSpans = append(converted.ScopeSpans, scope)
		}
		out.ResourceSpans = append(out.ResourceSpans, converted)
	}
	return out
}

func protoAttrs(attrs []*commonpb.KeyValue) []KeyValue {
	out := make([]KeyValue, 0, len(attrs))
	for _, a := range attrs {
		if a == nil {
			continue
		}
		out = append(out, KeyValue{Key: a.Key, Value: protoAny(a.Value)})
	}
	return out
}

func protoAny(v *commonpb.AnyValue) AnyValue {
	if v == nil {
		return AnyValue{}
	}
	switch t := v.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		s := t.StringValue
		return AnyValue{StringValue: &s}
	case *commonpb.AnyValue_BoolValue:
		b := t.BoolValue
		return AnyValue{BoolValue: &b}
	case *commonpb.AnyValue_IntValue:
		n := json.Number(strconv.FormatInt(t.IntValue, 10))
		return AnyValue{IntValue: &n}
	case *commonpb.AnyValue_DoubleValue:
		d := t.DoubleValue
		return AnyValue{DoubleValue: &d}
	case *commonpb.AnyValue_ArrayValue:
		arr := &ArrayValue{}
		if t.ArrayValue != nil {
			for _, item := range t.ArrayValue.Values {
				arr.Values = append(arr.Values, protoAny(item))
			}
		}
		return AnyValue{ArrayValue: arr}
	case *commonpb.AnyValue_KvlistValue:
		kv := &KvlistValue{}
		if t.KvlistValue != nil {
			kv.Values = protoAttrs(t.KvlistValue.Values)
		}
		return AnyValue{KvlistValue: kv}
	}
	return AnyValue{}
}

func protoStatus(st *tracepb.Status) OTLPStatus {
	if st == nil {
		return OTLPStatus{}
	}
	return OTLPStatus{Code: int32(st.Code), Message: st.Message}
}

func hexID(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&0x0f]
	}
	return string(out)
}
