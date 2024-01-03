package common

import (
	"context"
	"encoding/hex"

	"go.opentelemetry.io/otel/trace"
)

const (
	HeaderXForwardedFor = "x-forwarded-for"
	HeaderXClientId     = "x-client-id" // must be lowercase
	HeaderXTraceId      = "x-trace-id"
	HeaderXRequestId    = "x-request-id"

	ClientIdUnknown = "unknown"
	LogFieldTraceId = "trace_id"
)

func TraceIdFromCtx(ctx context.Context) (*trace.TraceID, bool) {
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		traceId := span.TraceID()
		return &traceId, true
	}
	return nil, false
}

func CtxWithTraceId(ctx context.Context, traceIdStr string) context.Context {
	var traceId trace.TraceID
	if len(traceIdStr) > len(traceId)*2 {
		traceIdStr = traceIdStr[:len(traceId)*2]
	} else if len(traceIdStr)%2 != 0 {
		traceIdStr += "0"
	}
	if _, err := hex.Decode(traceId[:], []byte(traceIdStr)); err != nil {
		copy(traceId[:], traceIdStr)
	}
	return trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceId,
		Remote:  true,
	}))
}
