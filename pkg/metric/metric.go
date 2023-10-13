package metric

import (
	"context"
	"time"

	kybermetric "github.com/KyberNetwork/kyber-trace-go/pkg/metric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	PanicTotal            = "grpc_req_panics_recovered_total"
	ClientRequestTotal    = "client_request_total"
	ExternalRequestTotal  = "external_request_total"
	TaskExecutionDuration = "task_execution_duration"
)

var (
	panicTotalCounter              metric.Int64Counter
	externalRequestTotalCounter    metric.Int64Counter
	clientRequestTotalCounter      metric.Int64Counter
	taskExecutionDurationHistogram metric.Float64Histogram
)

func init() {
	panicTotalCounter, _ = kybermetric.Meter().Int64Counter(PanicTotal,
		metric.WithDescription("Total number of gRPC requests recovered from internal panic."))
	externalRequestTotalCounter, _ = kybermetric.Meter().Int64Counter(ExternalRequestTotal,
		metric.WithDescription("Total number of external requests"))
	clientRequestTotalCounter, _ = kybermetric.Meter().Int64Counter(ClientRequestTotal,
		metric.WithDescription("Total number of incoming requests"))
	taskExecutionDurationHistogram, _ = kybermetric.Meter().Float64Histogram(TaskExecutionDuration,
		metric.WithUnit("ms"), metric.WithDescription("The duration of task execution."))
}

func IncPanicTotal(ctx context.Context) {
	panicTotalCounter.Add(ctx, 1)
}

func IncExternalRequestTotal(ctx context.Context, tags map[string]string) {
	attributes := make([]attribute.KeyValue, 0)
	for k, v := range tags {
		attributes = append(attributes, attribute.String(k, v))
	}
	externalRequestTotalCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

func IncClientRequestTotal(ctx context.Context, clientID string) {
	attributes := make([]attribute.KeyValue, 0)
	attributes = append(attributes, attribute.String("client_id", clientID))
	clientRequestTotalCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

func PushTaskExecutionDuration(ctx context.Context, duration time.Duration, tags map[string]string) {
	attributes := make([]attribute.KeyValue, 0)
	for k, v := range tags {
		attributes = append(attributes, attribute.String(k, v))
	}

	taskExecutionDurationHistogram.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attributes...))
}
