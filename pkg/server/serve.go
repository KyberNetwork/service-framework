package server

import (
	"context"
	"runtime/debug"

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

func Serve(ctx context.Context, cfg grpcserver.Config, opts ...OptFn) {
	defer shutdownKyberTrace()

	opt := new(option).Build(opts...)

	appMode := grpcserver.GetAppMode(cfg.Mode)
	isDevMode := appMode == grpcserver.Development
	unaryOpts := []grpc.UnaryServerInterceptor{
		trace.UnaryServerInterceptor(isDevMode, internalServerErr),
	}
	var streamOpts []grpc.StreamServerInterceptor

	loggingLogger := opt.loggingInterceptor
	if loggingLogger == nil {
		loggingLogger = logging.DefaultLogger(opt.logger,
			logging.IgnoreReq(cfg.Log.IgnoreReq...), logging.IgnoreResp(cfg.Log.IgnoreResp...))
	}
	recoveryOpt := recovery.WithRecoveryHandler(func(err any) error {
		klog.WithFields(ctx, klog.Fields{"error": err}).Errorf("recovered from:\n%s", string(debug.Stack()))
		kmetric.IncPanicTotal(context.Background())
		return internalServerErr.Err()
	})
	unaryOpts = append(unaryOpts,
		selector.UnaryServerInterceptor(logging.UnaryServerInterceptor(loggingLogger),
			selector.MatchFunc(healthSkip)),
		validator.UnaryServerInterceptor(),
		recovery.UnaryServerInterceptor(recoveryOpt),
	)
	streamOpts = append(streamOpts,
		selector.StreamServerInterceptor(logging.StreamServerInterceptor(loggingLogger),
			selector.MatchFunc(healthSkip)),
		validator.StreamServerInterceptor(),
		recovery.StreamServerInterceptor(recoveryOpt))

	otelGrpcStatHandler := getOtelGrpcStatsHandler()
	serverOptions := []grpc.ServerOption{
		grpc.StatsHandler(otelGrpcStatHandler),
		grpc.ChainUnaryInterceptor(unaryOpts...),
		grpc.ChainStreamInterceptor(streamOpts...),
	}
	s := grpcserver.NewServer(&cfg, appMode, serverOptions...)

	if err := s.Register(opt.services...); err != nil {
		klog.Fatalf(ctx, "Error register servers %v", err)
	}

	if err := s.Serve(ctx); err != nil {
		klog.Fatalf(ctx, "Error start server %v", err)
	}
}

func healthSkip(_ context.Context, c interceptors.CallMeta) bool {
	return c.FullMethod() != healthv1.Health_Check_FullMethodName
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
	md, _ := metadata.FromIncomingContext(ctx)
	requestIds := md.Get(common.HeaderXRequestId)
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

func shutdownKyberTrace() {
	ctx := context.Background()
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
