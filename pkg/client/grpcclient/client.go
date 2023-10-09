package grpcclient

import (
	"context"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	defaultGRPCBaseURL = "localhost:9080"
	ChainIDHeaderKey   = "X-Chain-ID"
	ClientIDHeaderKey  = "X-Client-ID"
)

type Config struct {
	BaseURL            string
	ReconnectionPeriod time.Duration
	IsBlockConnect     bool
	GRPCCredentials    credentials.TransportCredentials
	Insecure           bool
	Compression        Compression
	Headers            map[string]string
	ClientID           string
	Timeout            time.Duration
	DialOptions        []grpc.DialOption
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
	grpcConn, err := grpc.Dial(cfg.BaseURL, dialOpts...)
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

	if c.ReconnectionPeriod != 0 {
		p := grpc.ConnectParams{
			Backoff:           backoff.DefaultConfig,
			MinConnectTimeout: c.ReconnectionPeriod,
		}
		dialOpts = append(dialOpts, grpc.WithConnectParams(p))
	}

	if c.IsBlockConnect {
		dialOpts = append(dialOpts, grpc.WithBlock())
	}

	if c.Timeout != 0 {
		dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(TimeoutInterceptor(c.Timeout)))
	}

	dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(RequestHeadersInterceptor(c.requestHeaders())))

	return append(dialOpts, c.DialOptions...)
}

// requestHeaders generates the request headers based on the provided configuration.
func (c *Config) requestHeaders() map[string]string {
	headers := c.Headers
	if headers == nil {
		headers = make(map[string]string)
	}
	if c.ClientID != "" {
		headers[ClientIDHeaderKey] = c.ClientID
	}
	if headers[ClientIDHeaderKey] == "" {
		hostname, _ := os.Hostname()
		headers[ClientIDHeaderKey] = hostname
	}
	return headers
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

// RequestHeadersInterceptor intercepts gRPC unary client
// invocations and adds custom headers to the outgoing request.
func RequestHeadersInterceptor(header map[string]string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md := metadata.New(header)
		ctx = metadata.NewOutgoingContext(ctx, md)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
