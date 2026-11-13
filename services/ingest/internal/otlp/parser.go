package otlp

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	dbch "github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
)

// ParseResourceSpans converts OTLP ResourceSpans to internal Span structs.
// Extracts service.name from resource attributes and all GenAI + Vigil attributes from span attributes.
func ParseResourceSpans(rss []*tracepb.ResourceSpans) []dbch.Span {
	var spans []dbch.Span
	for _, rs := range rss {
		serviceName := ""
		if rs.Resource != nil {
			for _, attr := range rs.Resource.Attributes {
				if attr.Key == "service.name" {
					serviceName = anyValueString(attr.Value)
					break
				}
			}
		}

		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				s := convertSpan(sp, serviceName)
				spans = append(spans, s)
			}
		}
	}
	return spans
}

// convertSpan maps a single OTLP span proto to the internal Span struct.
func convertSpan(sp *tracepb.Span, serviceName string) dbch.Span {
	s := dbch.Span{
		TraceID:      hex.EncodeToString(sp.TraceId),
		SpanID:       hex.EncodeToString(sp.SpanId),
		ParentSpanID: hex.EncodeToString(sp.ParentSpanId),
		Name:         sp.Name,
		Kind:         kindString(sp.Kind),
		ServiceName:  serviceName,
		Attrs:        make(map[string]string),
		StartTime:    time.Unix(0, int64(sp.StartTimeUnixNano)).UTC(), // #nosec G115 -- nanosecond unix timestamp fits int64 for all practical dates
		EndTime:      time.Unix(0, int64(sp.EndTimeUnixNano)).UTC(),   // #nosec G115 -- same
		StatusCode:   statusCodeString(sp.Status),
		StatusMsg:    statusMsg(sp.Status),
	}

	for _, attr := range sp.Attributes {
		val := anyValueString(attr.Value)
		switch attr.Key {
		case "gen_ai.system":
			s.GenAISystem = val
		case "gen_ai.request.model":
			s.GenAIModel = val
		case "gen_ai.operation.name":
			s.GenAIOp = val
		case "gen_ai.usage.input_tokens":
			if n, err := strconv.ParseUint(val, 10, 32); err == nil {
				s.InputTokens = uint32(n)
			}
		case "gen_ai.usage.output_tokens":
			if n, err := strconv.ParseUint(val, 10, 32); err == nil {
				s.OutputTokens = uint32(n)
			}
		case "vigil.agent.id":
			s.VigilAgentID = val
		case "vigil.run.id":
			s.VigilRunID = val
		case "vigil.wallet":
			s.VigilWallet = val
		case "vigil.chain_id":
			if n, err := strconv.ParseUint(val, 10, 64); err == nil {
				s.VigilChainID = n
			}
		case "vigil.tx.hash":
			s.VigilTxHash = val
		case "vigil.flight_check.id":
			s.VigilFlightCheckID = val
		case "vigil.intent":
			s.VigilIntent = val
		default:
			s.Attrs[attr.Key] = val
		}
	}

	return s
}

// anyValueString converts an OTLP AnyValue to its string representation.
func anyValueString(v *commonpb.AnyValue) string {
	if v == nil {
		return ""
	}
	switch vt := v.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return vt.StringValue
	case *commonpb.AnyValue_IntValue:
		return strconv.FormatInt(vt.IntValue, 10)
	case *commonpb.AnyValue_DoubleValue:
		return strconv.FormatFloat(vt.DoubleValue, 'f', -1, 64)
	case *commonpb.AnyValue_BoolValue:
		return strconv.FormatBool(vt.BoolValue)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// kindString maps OTLP span kind enum to string.
func kindString(k tracepb.Span_SpanKind) string {
	switch k {
	case tracepb.Span_SPAN_KIND_INTERNAL:
		return "internal"
	case tracepb.Span_SPAN_KIND_SERVER:
		return "server"
	case tracepb.Span_SPAN_KIND_CLIENT:
		return "client"
	case tracepb.Span_SPAN_KIND_PRODUCER:
		return "producer"
	case tracepb.Span_SPAN_KIND_CONSUMER:
		return "consumer"
	default:
		return "unspecified"
	}
}

// statusCodeString maps OTLP status to string.
func statusCodeString(status *tracepb.Status) string {
	if status == nil {
		return "unset"
	}
	switch status.Code {
	case tracepb.Status_STATUS_CODE_OK:
		return "ok"
	case tracepb.Status_STATUS_CODE_ERROR:
		return "error"
	default:
		return "unset"
	}
}

// statusMsg extracts status message from OTLP status.
func statusMsg(status *tracepb.Status) string {
	if status == nil {
		return ""
	}
	return status.Message
}
