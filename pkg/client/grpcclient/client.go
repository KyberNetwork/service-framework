package grpcclient

import (
	"context"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/validator"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/KyberNetwork/service-framework/pkg/common"
	"github.com/KyberNetwork/service-framework/pkg/observe"
	"github.com/KyberNetwork/service-framework/pkg/observe/kmetric"
)

const (
	defaultGRPCBaseURL = "localhost:9080"
)

type Config struct {
	BaseURL           string
	MinConnectTimeout time.Duration
	ConnectBackoff    backoff.Config
	IsBlockConnect    bool // deprecated: see grpc.WithBlock
	GRPCCredentials   credentials.TransportCredentials
	Insecure          bool
	Compression       Compression
	Headers           map[string]string
	ClientID          string
	Timeout           time.Duration
	DialOptions       []grpc.DialOption
}

// Client wraps the created grpc connection and client.
type Client[T any] struct {
	C    T                // inner grpc client
	Cfg  *Config          // grpc connection dial config
	Conn *grpc.ClientConn // grpc connection
}

// New creates a new instance of the Client using the provided client factory function and apply options.
//
// Parameters:
// - clientFactory: A function that takes a grpc.ClientConnInterface and returns an instance of T.
// - applyOptions: Optional apply options to configure the Client.
//
// Returns:
// - *Client[T]: A pointer to the Client instance.
// - error: An error if there was a problem creating the Client.
func New[T any](clientFactory func(grpc.ClientConnInterface) T, applyOptions ...ApplyOption) (*Client[T],
	error) {
	cfg := &Config{}
	for _, apply := range applyOptions {
		apply(cfg)
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultGRPCBaseURL
		cfg.Insecure = true
	}

	dialOpts := cfg.dialOptions()
	grpcConn, err := grpc.NewClient(cfg.BaseURL, dialOpts...)
	if err != nil {
		return nil, err
	}

	return &Client[T]{
		C:    clientFactory(grpcConn),
		Cfg:  cfg,
		Conn: grpcConn,
	}, nil
}

// dialOptions returns the dial options from the Config struct.
//
// The function checks the Config fields and appends corresponding dial options to the returned slice.
func (c *Config) dialOptions() []grpc.DialOption {
	var dialOpts []grpc.DialOption
	if c.GRPCCredentials != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(c.GRPCCredentials))
	} else if c.Insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		creds := credentials.NewTLS(nil)
		c.GRPCCredentials = creds
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	}

	if c.Compression != NoCompression {
		dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.UseCompressor(string(c.Compression))))
	}

	connectParams := grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  1 * time.Second,
			Multiplier: 1.6,
			Jitter:     0.2,
			MaxDelay:   5 * time.Second,
		},
		MinConnectTimeout: 5 * time.Second,
	}
	if c.ConnectBackoff.BaseDelay != 0 {
		connectParams.Backoff.BaseDelay = c.ConnectBackoff.BaseDelay
	}
	if c.ConnectBackoff.Multiplier != 0 {
		connectParams.Backoff.Multiplier = c.ConnectBackoff.Multiplier
	}
	if c.ConnectBackoff.Jitter != 0 {
		connectParams.Backoff.Jitter = c.ConnectBackoff.Jitter
	}
	if c.ConnectBackoff.MaxDelay != 0 {
		connectParams.Backoff.MaxDelay = c.ConnectBackoff.MaxDelay
	}
	if c.MinConnectTimeout != 0 {
		connectParams.MinConnectTimeout = c.MinConnectTimeout
	}
	dialOpts = append(dialOpts, grpc.WithConnectParams(connectParams))

	requestHeaders := c.requestHeaders()
	unaryInterceptors := []grpc.UnaryClientInterceptor{
		validator.UnaryClientInterceptor(),
		RequestHeadersInterceptor(requestHeaders),
		MetricsInterceptor(),
	}
	if c.Timeout != 0 {
		unaryInterceptors = append(unaryInterceptors, TimeoutInterceptor(c.Timeout))
	}

	dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(unaryInterceptors...))

	observe.EnsureTracerProvider()
	dialOpts = append(dialOpts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()), grpc.WithDisableServiceConfig())

	return append(dialOpts, c.DialOptions...)
}

// requestHeaders generates the request headers based on the provided configuration.
func (c *Config) requestHeaders() map[string]string {
	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}
	if c.ClientID == "" {
		c.ClientID = common.GetServiceClientId()
	}
	c.Headers[common.HeaderXClientId] = c.ClientID
	return c.Headers
}

func (c *Client[_]) Close() error {
	return c.Conn.Close()
}

type TimeoutCallOption struct {
	grpc.EmptyCallOption

	forcedTimeout time.Duration
}

// WithForcedTimeout will be used to set the timeout for particular requests.
// Example:
// c.ListPrices(context.Background(), &v1.ListPricesRequest,
//
//	           WithForcedTimeout(time.Duration(10)*time.Second))
//	The timeout of ListPrices RPC call is 10 seconds
func WithForcedTimeout(forceTimeout time.Duration) TimeoutCallOption {
	return TimeoutCallOption{forcedTimeout: forceTimeout}
}

func getForcedTimeout(callOptions []grpc.CallOption) (time.Duration, bool) {
	for _, opt := range callOptions {
		if co, ok := opt.(TimeoutCallOption); ok {
			return co.forcedTimeout, true
		}
	}

	return 0, false
}

// RequestHeadersInterceptor intercepts gRPC unary client
// invocations and adds custom headers to the outgoing request.
func RequestHeadersInterceptor(header map[string]string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md := metadata.New(header)
		if existingMd, ok := metadata.FromOutgoingContext(ctx); ok {
			for k, v := range existingMd {
				md.Set(k, v...)
			}
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// MetricsInterceptor intercepts gRPC unary client invocations to record metrics.
func MetricsInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
		defer func() {
			code := codes.OK
			if reply, ok := reply.(interface{ GetCode() int32 }); ok {
				code = codes.Code(reply.GetCode())
			} else if err != nil {
				code = codes.Unknown
			}
			kmetric.IncOutgoingRequest(ctx, kmetric.AttrMethod, method, kmetric.AttrCode, code.String())
		}()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// TimeoutInterceptor intercepts unary client requests and adds a timeout to the context.
func TimeoutInterceptor(t time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		timeout := t
		if v, ok := getForcedTimeout(opts); ok {
			timeout = v
		}

		if timeout <= 0 {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
