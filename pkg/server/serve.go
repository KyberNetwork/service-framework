package server

import (
	"context"
	"runtime/debug"

	"github.com/KyberNetwork/kutils"
	"github.com/KyberNetwork/kutils/klog"
	kybermetric "github.com/KyberNetwork/kyber-trace-go/pkg/metric"
	kybertracer "github.com/KyberNetwork/kyber-trace-go/pkg/tracer"
	_ "github.com/KyberNetwork/kyber-trace-go/tools"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/validator"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	"github.com/KyberNetwork/service-framework/pkg/common"
	"github.com/KyberNetwork/service-framework/pkg/observe"
	"github.com/KyberNetwork/service-framework/pkg/observe/kmetric"
	"github.com/KyberNetwork/service-framework/pkg/server/grpcserver"
	"github.com/KyberNetwork/service-framework/pkg/server/middleware/logging"
	"github.com/KyberNetwork/service-framework/pkg/server/middleware/trace"
)

var internalServerErr = status.New(codes.Internal, "Internal server error")

// Serve starts gRPC server and HTTP grpc gateway server. It blocks until os.Interrupt or syscall.SIGTERM.
// Example usage:
//
//	server.Serve(ctx, cfg, service1, service2, server.WithLogger(myLoggerFactory))
func Serve(ctx context.Context, cfg grpcserver.Config, opts ...grpcserver.Opt) {
	defer shutdownKyberTrace(kutils.CtxWithoutCancel(ctx))

	cfg = cfg.Apply(opts...)

	isDevMode := cfg.Mode == grpcserver.Development

	loggingLogger := cfg.LoggingInterceptor()
	recoveryOpt := recovery.WithRecoveryHandler(func(err any) error {
		klog.WithFields(ctx, klog.Fields{"error": err}).Errorf("recovered from:\n%s", string(debug.Stack()))
		kmetric.IncPanicTotal(context.Background())
		return internalServerErr.Err()
	})

	otelGrpcStatHandler := getOtelGrpcStatsHandler()
	unaryOpts := []grpc.UnaryServerInterceptor{
		unaryHealthSkip(trace.UnaryServerInterceptor(isDevMode, internalServerErr)),
		unaryHealthSkip(logging.UnaryServerInterceptor(loggingLogger)),
		validator.UnaryServerInterceptor(),
		recovery.UnaryServerInterceptor(recoveryOpt),
	}
	streamOpts := []grpc.StreamServerInterceptor{
		streamHealthSkip(logging.StreamServerInterceptor(loggingLogger)),
		validator.StreamServerInterceptor(),
		recovery.StreamServerInterceptor(recoveryOpt),
	}

	serverOptions := append([]grpc.ServerOption{
		grpc.StatsHandler(otelGrpcStatHandler),
		grpc.ChainUnaryInterceptor(unaryOpts...),
		grpc.ChainStreamInterceptor(streamOpts...),
	}, cfg.GRPCServerOptions()...)

	s := grpcserver.NewServer(&cfg, serverOptions...)

	if err := s.Register(cfg.Services()...); err != nil {
		klog.Fatalf(ctx, "Error register servers %v", err)
	}

	if err := s.Serve(ctx); err != nil {
		klog.Fatalf(ctx, "Error start server %v", err)
	}
}

var healthSkipMatchFunc = selector.MatchFunc(func(_ context.Context, c interceptors.CallMeta) bool {
	return c.FullMethod() != healthv1.Health_Check_FullMethodName
})

func unaryHealthSkip(interceptor grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return selector.UnaryServerInterceptor(interceptor, healthSkipMatchFunc)
}

func streamHealthSkip(interceptor grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	return selector.StreamServerInterceptor(interceptor, healthSkipMatchFunc)
}

func getOtelGrpcStatsHandler() stats.Handler {
	observe.EnsureTracerProvider()
	propagator := otel.GetTextMapPropagator()
	propagator = &requestIdExtractor{propagator}
	return &OtelServerHandler{otelgrpc.NewServerHandler(otelgrpc.WithPropagators(propagator))}
}

type requestIdExtractor struct {
	propagation.TextMapPropagator
}

func (r *requestIdExtractor) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	ctx = r.TextMapPropagator.Extract(ctx, carrier)
	if _, ok := common.TraceIdFromCtx(ctx); ok {
		return ctx
	}
	requestIds := metadata.ValueFromIncomingContext(ctx, common.HeaderXRequestId)
	if len(requestIds) == 0 {
		return ctx
	}
	return common.CtxWithTraceId(ctx, requestIds[0])
}

type OtelServerHandler struct {
	stats.Handler
}

type ctxKeySkipHealth struct{}

func (s *OtelServerHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	if info.FullMethodName == healthv1.Health_Check_FullMethodName ||
		info.FullMethodName == healthv1.Health_Watch_FullMethodName {
		return context.WithValue(ctx, ctxKeySkipHealth{}, struct{}{})
	}
	return s.Handler.TagRPC(ctx, info)
}

func (s *OtelServerHandler) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	if ctx.Value(ctxKeySkipHealth{}) != nil {
		return
	}
	s.Handler.HandleRPC(ctx, rs)
}

func shutdownKyberTrace(ctx context.Context) {
	shutdownTracer(ctx)
	shutdownMetric(ctx)
}

func shutdownTracer(ctx context.Context) {
	if kybertracer.Provider() != nil {
		err := kybertracer.Flush(ctx)
		if err != nil {
			klog.Errorf(ctx, "Failed to flush tracer: %v", err)
		}
		klog.Info(ctx, "start shutdown tracer")
		err = kybertracer.Shutdown(ctx)
		if err != nil {
			klog.Errorf(ctx, "Failed to shutdown tracer: %v", err)
		}
	}
}

func shutdownMetric(ctx context.Context) {
	if kybermetric.Provider() != nil {
		err := kybermetric.Flush(ctx)
		if err != nil {
			klog.Errorf(ctx, "Failed to flush metric: %v", err)
		}
		klog.Info(ctx, "start shutdown metric")
		err = kybermetric.Shutdown(ctx)
		if err != nil {
			klog.Errorf(ctx, "Failed to shutdown metric: %v", err)
		}
	}
}
