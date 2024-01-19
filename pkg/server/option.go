package server

import (
	"context"

	"google.golang.org/grpc"

	"github.com/KyberNetwork/service-framework/pkg/server/grpcserver"
	"github.com/KyberNetwork/service-framework/pkg/server/middleware/logging"
)

type option struct {
	services           []grpcserver.Service                     // services to register
	loggingInterceptor logging.InterceptorLogger                // to override default interceptor logger
	logger             func(ctx context.Context) logging.Logger // to override logger used by default interceptor logger
	grpcServerOptions  []grpc.ServerOption                      // additional grpc server options
}

func (o *option) Build(optFns ...OptFn) *option {
	if o == nil {
		*o = option{}
	}
	for _, opt := range optFns {
		opt(o)
	}
	return o
}

type OptFn func(*option)

func WithServices(services ...grpcserver.Service) OptFn {
	return func(o *option) {
		o.services = append(o.services, services...)
	}
}

func WithLoggingInterceptor(interceptor logging.InterceptorLogger) OptFn {
	return func(o *option) {
		o.loggingInterceptor = interceptor
	}
}

func WithLogger(logger func(ctx context.Context) logging.Logger) OptFn {
	return func(o *option) {
		o.logger = logger
	}
}

func WithGRPCServerOptions(options ...grpc.ServerOption) OptFn {
	return func(o *option) {
		o.grpcServerOptions = append(o.grpcServerOptions, options...)
	}
}
