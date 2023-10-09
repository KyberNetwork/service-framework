package server

import (
	"context"
	"runtime/debug"
	"time"

	kybermetric "github.com/KyberNetwork/kyber-trace-go/pkg/metric"
	kybertracer "github.com/KyberNetwork/kyber-trace-go/pkg/tracer"
	_ "github.com/KyberNetwork/kyber-trace-go/tools"
	"github.com/KyberNetwork/logger"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/validator"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"github.com/KyberNetwork/service-framework/pkg/metric"
	"github.com/KyberNetwork/service-framework/pkg/server/grpcserver"
	"github.com/KyberNetwork/service-framework/pkg/server/middleware/client"
	"github.com/KyberNetwork/service-framework/pkg/server/middleware/grpcerror"
	logginpkg "github.com/KyberNetwork/service-framework/pkg/server/middleware/logging"
)

var internalServerErr = status.Error(codes.Internal, "Something went wrong in our side.")

const pushPanicMetricTimeout = time.Second

func Serve(cfg *grpcserver.Config, services ...grpcserver.Service) {
	defer shutdownKyberTrace()
	zapLogger, err := logger.GetDesugaredZapLoggerDelegate(logger.Get())
	if err != nil {
		logger.Fatalf("Error when getting zapLogger cause by %v", err)
	}

	recoveryOpt := recovery.WithRecoveryHandler(func(err any) error {
		logger.WithFields(logger.Fields{"error": err}).Errorf("recovered from:\n%s", string(debug.Stack()))
		ctx, cancel := context.WithTimeout(context.Background(), pushPanicMetricTimeout)
		defer cancel()
		metric.IncPanicTotal(ctx)
		return internalServerErr
	})

	isDevMode := grpcserver.GetAppMode(cfg.Mode) == grpcserver.Development

	unaryOpts := []grpc.UnaryServerInterceptor{
		grpcerror.UnaryServerInterceptor(isDevMode, internalServerErr),
		client.UnaryServerInterceptor(),
	}
	otelGRPCOptions := getOTELGRPCOptions()

	var streamOpts []grpc.StreamServerInterceptor

	if isEnabledOTEL() {
		streamOpts = append(streamOpts, otelgrpc.StreamServerInterceptor(
			otelGRPCOptions...,
		))
		unaryOpts = append(unaryOpts, otelgrpc.UnaryServerInterceptor(
			otelGRPCOptions...,
		))
	}
	loggingOpts := getLoggingOptions(cfg.Flag.GRPC)
	streamOpts = append(streamOpts,
		selector.StreamServerInterceptor(logging.StreamServerInterceptor(logginpkg.InterceptorLogger(zapLogger),
			loggingOpts...), selector.MatchFunc(healthSkip)),
		logging.StreamServerInterceptor(logginpkg.InterceptorLogger(zapLogger), loggingOpts...),
		validator.StreamServerInterceptor(),
		recovery.StreamServerInterceptor(recoveryOpt))
	unaryOpts = append(unaryOpts,
		selector.UnaryServerInterceptor(logging.UnaryServerInterceptor(logginpkg.InterceptorLogger(zapLogger),
			loggingOpts...), selector.MatchFunc(healthSkip)),
		validator.UnaryServerInterceptor(),
		recovery.UnaryServerInterceptor(recoveryOpt),
	)

	s := grpcserver.NewServer(cfg, isDevMode,
		grpc.ChainUnaryInterceptor(unaryOpts...),
		grpc.ChainStreamInterceptor(streamOpts...),
	)

	if err := s.Register(services...); err != nil {
		logger.Fatalf("Error register servers %v", err)
	}

	logger.WithFields(logger.Fields{
		"grpc_addr": cfg.GRPC.Host,
		"grpc_port": cfg.GRPC.Port,
		"http_addr": cfg.HTTP.Host,
		"http_port": cfg.HTTP.Port}).Info("Starting server...")
	if err := s.Serve(); err != nil {
		logger.Fatalf("Error start server %v", err)
	}
}

func NewService[T any](srv T,
	regServiceServer func(s grpc.ServiceRegistrar, srv T),
	regServiceHandlerFromEndpoint func(ctx context.Context, mux *runtime.ServeMux, endpoint string,
		opts []grpc.DialOption) (err error)) grpcserver.Service {
	return grpcserver.NewService(srv, regServiceServer, regServiceHandlerFromEndpoint)
}

func healthSkip(_ context.Context, c interceptors.CallMeta) bool {
	return c.FullMethod() != healthv1.Health_Check_FullMethodName
}

func filterTrace() otelgrpc.Filter {
	return func(ii *otelgrpc.InterceptorInfo) bool {
		if ii != nil && isHealthCheckCall(ii) {
			return false
		}
		return true
	}
}

func isHealthCheckCall(ii *otelgrpc.InterceptorInfo) bool {
	return ii.UnaryServerInfo != nil && ii.UnaryServerInfo.FullMethod == healthv1.Health_Check_FullMethodName || ii.StreamServerInfo != nil && ii.StreamServerInfo.FullMethod == healthv1.Health_Check_FullMethodName
}

func getOTELGRPCOptions() []otelgrpc.Option {
	return []otelgrpc.Option{
		otelgrpc.WithInterceptorFilter(filterTrace()),
		otelgrpc.WithPropagators(otel.GetTextMapPropagator()),
		otelgrpc.WithMeterProvider(kybermetric.Provider()),
		otelgrpc.WithTracerProvider(kybertracer.Provider()),
	}
}

func shutdownKyberTrace() {
	ctx := context.Background()
	shutdownTracer(ctx)
	shutdownMetric(ctx)
}

func shutdownTracer(ctx context.Context) {
	if kybertracer.Provider() != nil {
		err := kybertracer.Flush(ctx)
		if err != nil {
			logger.Errorf("Failed to flush tracer: %v", err)
		}
		logger.Info("start shutdown tracer")
		err = kybertracer.Shutdown(ctx)
		if err != nil {
			logger.Errorf("Failed to shutdown tracer: %v", err)
		}
	}
}

func shutdownMetric(ctx context.Context) {
	if kybermetric.Provider() != nil {
		err := kybermetric.Flush(ctx)
		if err != nil {
			logger.Errorf("Failed to flush metric: %v", err)
		}
		logger.Info("start shutdown metric")
		err = kybermetric.Shutdown(ctx)
		if err != nil {
			logger.Errorf("Failed to shutdown metric: %v", err)
		}
	}
}

func isEnabledOTEL() bool {
	return kybertracer.Provider() != nil && kybermetric.Provider() != nil
}

func getLoggingOptions(grpc grpcserver.GRPCFlag) []logging.Option {
	logTraceID := func(ctx context.Context) logging.Fields {
		if span := trace.SpanContextFromContext(ctx); span.IsValid() {
			return logging.Fields{"traceID", span.TraceID().String()}
		}
		return nil
	}

	opts := []logging.Option{
		logging.WithFieldsFromContext(logTraceID),
	}
	loggableEvents := []logging.LoggableEvent{logging.FinishCall}

	if !grpc.DisableLogRequest {
		loggableEvents = append(loggableEvents, logging.PayloadReceived)
	}
	if !grpc.DisableLogResponse {
		loggableEvents = append(loggableEvents, logging.PayloadSent)
	}

	opts = append(opts, logging.WithLogOnEvents(loggableEvents...))
	return opts
}
