package observe

import (
	"context"

	"github.com/KyberNetwork/kyber-trace-go/pkg/constant"
	kybertracer "github.com/KyberNetwork/kyber-trace-go/pkg/tracer"
	"github.com/KyberNetwork/kyber-trace-go/pkg/util/env"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
)

func EnsureTracerProvider() {
	if kybertracer.Provider() != nil {
		return
	}
	resources := resource.Default()
	extraResources, err := resource.New(context.Background(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(env.StringFromEnv(constant.EnvKeyOtelServiceName, constant.OtelDefaultServiceName)),
			semconv.ServiceVersion(env.StringFromEnv(constant.EnvKeyOtelServiceVersion,
				constant.OtelDefaultServiceVersion)),
		))
	if err == nil {
		resources, err = resource.Merge(resources, extraResources)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resources),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(nil)),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
}
