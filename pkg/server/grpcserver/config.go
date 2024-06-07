package grpcserver

import (
	"context"
	"net"
	"strconv"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	"github.com/KyberNetwork/service-framework/pkg/server/middleware/logging"
)

var (
	DefaultGRPC = Listen{
		Host: "0.0.0.0",
		Port: 9080,
	}
	DefaultHTTP = Listen{
		Host: "0.0.0.0",
		Port: 8080,
	}
)

type (
	Server struct {
		gRPC *grpc.Server
		mux  *runtime.ServeMux
		cfg  *Config
	}

	// Config hold http/grpc server config
	Config struct {
		Mode     AppMode
		GRPC     Listen
		HTTP     Listen
		BasePath string
		Log      Log

		services           []Service                            // services to register
		loggingInterceptor logging.InterceptorLogger            // to override default interceptor logger
		logger             func(context.Context) logging.Logger // to override logger used by default interceptor logger
		grpcServerOptions  []grpc.ServerOption                  // additional grpc server options
		passThruHeaders    struct {                             // headers to pass through from http to grpc
			incoming []string // incoming headers (from requests)
			outgoing []string // outgoing headers (in responses)
		}
		httpMarshalerOptions HttpMarshalerOptions
	}

	Log struct {
		IgnoreReq  []string
		IgnoreResp []string
	}

	// Listen config for host/port socket listener
	Listen struct {
		Host string
		Port int
	}

	// HttpMarshalerOptions config for http marshaler
	HttpMarshalerOptions struct { // http marshaler options. see google.golang.org/protobuf/encoding/protojson
		DisallowUnknown  bool   // disallow unknown fields in request
		AllowPartialReq  bool   // allow missing required fields in request
		AllowPartialResp bool   // allow missing required fields in response
		Multiline        bool   // multiline response
		Indent           string // indent for multiline
		UseProtoNames    bool   // use proto names instead of lowerCamelCase
		UseEnumNumbers   bool   // use enum number instead of name
		EmitUnpopulated  bool   // emit unpopulated fields with zero values
	}
)

// String return socket listen DSN
func (l *Listen) String() string {
	return net.JoinHostPort(l.Host, strconv.Itoa(l.Port))
}

// Apply config options
func (c Config) Apply(opts ...Opt) Config {
	for _, opt := range opts {
		opt.opt(&c)
	}
	return c
}

func (c Config) Services() []Service {
	return c.services
}

func (c Config) LoggingInterceptor() logging.InterceptorLogger {
	loggingLogger := c.loggingInterceptor
	if loggingLogger == nil {
		loggingLogger = logging.DefaultLogger(c.logger,
			logging.IgnoreReq(c.Log.IgnoreReq...), logging.IgnoreResp(c.Log.IgnoreResp...))
	}
	return loggingLogger
}

func (c Config) GRPCServerOptions() []grpc.ServerOption {
	return c.grpcServerOptions
}

// Opt is an option for server config
type Opt interface {
	opt(*Config)
}

// OptFn implements Opt by calling itself
type OptFn func(*Config)

// opt implements Opt
func (o OptFn) opt(c *Config) {
	o(c)
}

// WithServices add services to serve
func WithServices(services ...Service) Opt {
	return OptFn(func(c *Config) {
		c.services = append(c.services, services...)
	})
}

// WithLoggingInterceptor overrides the default logging interceptor
func WithLoggingInterceptor(interceptor logging.InterceptorLogger) Opt {
	return OptFn(func(c *Config) {
		c.loggingInterceptor = interceptor
	})
}

// WithLogger overrides the default logger used by the default logging interceptor
func WithLogger(logger func(ctx context.Context) logging.Logger) Opt {
	return OptFn(func(c *Config) {
		c.logger = logger
	})
}

// WithGRPCServerOptions allows user to add custom grpc server options
func WithGRPCServerOptions(options ...grpc.ServerOption) Opt {
	return OptFn(func(c *Config) {
		c.grpcServerOptions = append(c.grpcServerOptions, options...)
	})
}

// WithPassThruIncomingHeaders adds incoming headers to pass through from http to grpc
func WithPassThruIncomingHeaders(headers ...string) Opt {
	return OptFn(func(c *Config) {
		c.passThruHeaders.incoming = append(c.passThruHeaders.incoming, headers...)
	})
}

// WithPassThruOutgoingHeaders adds outgoing headers to pass through from grpc to http
func WithPassThruOutgoingHeaders(headers ...string) Opt {
	return OptFn(func(c *Config) {
		c.passThruHeaders.outgoing = append(c.passThruHeaders.outgoing, headers...)
	})
}

// WithHTTPMarshalerOptions allows user to add custom http marshaler options
func WithHTTPMarshalerOptions(options HttpMarshalerOptions) Opt {
	return OptFn(func(c *Config) {
		c.httpMarshalerOptions = options
	})
}
