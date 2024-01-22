package server

import (
	"context"

	"google.golang.org/grpc"

	"github.com/KyberNetwork/service-framework/pkg/server/grpcserver"
	"github.com/KyberNetwork/service-framework/pkg/server/middleware/logging"
)

type (
	Opt   grpcserver.Opt
	OptFn Opt // backwards compatibility
)

// WithServices add services to serve
func WithServices(services ...grpcserver.Service) Opt {
	return grpcserver.WithServices(services...)
}

// WithLoggingInterceptor overrides the default logging interceptor
func WithLoggingInterceptor(interceptor logging.InterceptorLogger) Opt {
	return grpcserver.WithLoggingInterceptor(interceptor)
}

// WithLogger overrides the default logger used by the default logging interceptor
func WithLogger(logger func(ctx context.Context) logging.Logger) Opt {
	return grpcserver.WithLogger(logger)
}

// WithGRPCServerOptions allows user to add custom grpc server options
func WithGRPCServerOptions(options ...grpc.ServerOption) Opt {
	return grpcserver.WithGRPCServerOptions(options...)
}
