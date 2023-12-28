package kmetric

import (
	"context"
	"time"

	"github.com/KyberNetwork/kyber-trace-go/pkg/constant"
	kybermetric "github.com/KyberNetwork/kyber-trace-go/pkg/metric"
	"github.com/KyberNetwork/kyber-trace-go/pkg/util/env"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc/codes"
)

var (
	PanicCounter          = "panic"
	IncomingRequest       = "incoming_request"
	OutgoingRequest       = "outgoing_request"
	TaskExecutionDuration = "task_execution_duration"

	AttrServerName = "server.name"
	AttrClientName = "client.name"
	AttrMethod     = "method"
	AttrCode       = "code"
)

var (
	serviceName    = env.StringFromEnv(constant.EnvKeyOtelServiceName, constant.OtelDefaultServiceName)
	serverNameAttr = attribute.String(AttrServerName, serviceName)
	clientNameAttr = attribute.String(AttrClientName, serviceName)

	meter        = kybermetric.Meter()
	panicCounter = noErr(meter.Int64Counter(PanicCounter,
		metric.WithDescription("Counter of requests recovered from panic")))

	incomingRequestCounter = noErr(kybermetric.Meter().Int64Counter(IncomingRequest,
		metric.WithDescription("Counter of incoming requests")))
	outgoingRequestCounter = noErr(kybermetric.Meter().Int64Counter(OutgoingRequest,
		metric.WithDescription("Counter of outgoing requests")))
	taskExecutionDurationHistogram = noErr(kybermetric.Meter().Float64Histogram(TaskExecutionDuration,
		metric.WithUnit("ms"), metric.WithDescription("Histogram of task execution durations")))
)

func noErr[T any](t T, _ error) T {
	return t
}

func IncPanicTotal(ctx context.Context) {
	panicCounter.Add(ctx, 1, metric.WithAttributes(serverNameAttr))
}

func IncIncomingRequest(ctx context.Context, clientId string, code codes.Code) {
	outgoingRequestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String(AttrClientName, clientId), serverNameAttr, attribute.String(AttrCode, code.String())))
}

func IncOutgoingRequest(ctx context.Context, keyValues ...string) {
	attributes := make([]attribute.KeyValue, 1+len(keyValues)/2)
	attributes[0] = clientNameAttr
	for i := 1; i < len(keyValues); i += 2 {
		attributes[i/2+1] = attribute.String(keyValues[i-1], keyValues[i])
	}
	incomingRequestCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

func PushTaskExecutionDuration(ctx context.Context, duration time.Duration, keyValues ...string) {
	attributes := make([]attribute.KeyValue, 1+len(keyValues)/2)
	attributes[0] = serverNameAttr
	for i := 1; i < len(keyValues); i += 2 {
		attributes[i/2+1] = attribute.String(keyValues[i-1], keyValues[i])
	}
	taskExecutionDurationHistogram.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attributes...))
}
