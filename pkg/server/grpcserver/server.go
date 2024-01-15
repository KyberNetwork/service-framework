package grpcserver

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/KyberNetwork/kutils/klog"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/KyberNetwork/service-framework/pkg/common"
)

type (
	Service interface {
		RegServer(s grpc.ServiceRegistrar)
		RegServiceHandlerFromEndpoint(ctx context.Context, mux *runtime.ServeMux, endpoint string,
			opts []grpc.DialOption) (err error)
	}

	ServiceImpl[T any] struct {
		srv                           T
		regServiceServer              func(s grpc.ServiceRegistrar, srv T)
		regServiceHandlerFromEndpoint func(ctx context.Context, mux *runtime.ServeMux, endpoint string,
			opts []grpc.DialOption) (err error)
	}
)

func (s *ServiceImpl[T]) RegServer(serviceRegistrar grpc.ServiceRegistrar) {
	s.regServiceServer(serviceRegistrar, s.srv)
}

func (s *ServiceImpl[T]) RegServiceHandlerFromEndpoint(ctx context.Context, mux *runtime.ServeMux, endpoint string,
	opts []grpc.DialOption) (err error) {
	return s.regServiceHandlerFromEndpoint(ctx, mux, endpoint, opts)
}

func NewService[T any](srv T,
	regServiceServer func(s grpc.ServiceRegistrar, srv T),
	regServiceHandlerFromEndpoint func(ctx context.Context, mux *runtime.ServeMux, endpoint string,
		opts []grpc.DialOption) (err error)) Service {
	return &ServiceImpl[T]{
		srv:                           srv,
		regServiceServer:              regServiceServer,
		regServiceHandlerFromEndpoint: regServiceHandlerFromEndpoint,
	}
}

// NewServer return a new grpc server
func NewServer(cfg *Config, appMode AppMode, opt ...grpc.ServerOption) *Server {
	if cfg.GRPC.Host == "" && cfg.GRPC.Port == 0 {
		cfg.GRPC = DefaultGRPC
	}
	if cfg.HTTP.Host == "" && cfg.HTTP.Port == 0 {
		cfg.HTTP = DefaultHTTP
	}
	return &Server{
		cfg:     cfg,
		AppMode: appMode,

		gRPC: grpc.NewServer(opt...),
		mux: runtime.NewServeMux(
			runtime.WithIncomingHeaderMatcher(CustomHeaderMatcher),
			runtime.WithOutgoingHeaderMatcher(CustomHeaderMatcher),
			runtime.WithMarshalerOption(runtime.MIMEWildcard,
				&runtime.JSONPb{
					MarshalOptions: protojson.MarshalOptions{
						UseProtoNames:   false,
						UseEnumNumbers:  false,
						EmitUnpopulated: true,
					},
					UnmarshalOptions: protojson.UnmarshalOptions{
						DiscardUnknown: true,
					},
				})),
	}
}

func (s *Server) Register(services ...Service) error {
	for _, service := range services {
		service.RegServer(s.gRPC)
		if err := service.RegServiceHandlerFromEndpoint(context.Background(), s.mux, s.cfg.GRPC.String(),
			[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}); err != nil {
			return err
		}
	}

	healthv1.RegisterHealthServer(s.gRPC, health.NewServer())
	return nil
}

// Serve server listen for HTTP and GRPC
func (s *Server) Serve(ctx context.Context) (err error) {
	stop := make(chan os.Signal, 1)
	errCh := make(chan error)
	signal.Notify(stop, os.Interrupt, os.Kill, syscall.SIGTERM)

	go func() {
		listener, err := net.Listen("tcp", s.cfg.GRPC.String())
		if err != nil {
			errCh <- err
			return
		}
		errCh <- s.gRPC.Serve(listener)
	}()
	defer s.gRPC.GracefulStop()

	httpMux := http.NewServeMux()
	basePath := normalizeBasePath(s.cfg.BasePath)
	httpMux.Handle(basePath+"/", stripBasePath(s.mux, basePath))
	h2s := &http2.Server{}
	httpServer := &http.Server{
		Addr:    s.cfg.HTTP.String(),
		Handler: h2c.NewHandler(httpMux, h2s),
	}
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()
	defer func() {
		if err := httpServer.Shutdown(ctx); err != nil {
			klog.Errorf(ctx, "failed to shutdown http server: %v", err)
		}
	}()

	klog.WithFields(ctx, klog.Fields{
		"grpc_addr": s.cfg.GRPC.String(),
		"http_addr": s.cfg.HTTP.String()}).Info("Starting server...")
	select {
	case sig := <-stop:
		klog.Infof(ctx, "Received %s signal, stopping server...", sig.String())
		return nil
	case err = <-errCh:
		klog.Infof(ctx, "Received fatal error %v, stopping server...", err)
		return err
	}
}

func normalizeBasePath(path string) string {
	if path == "" {
		return ""
	}
	if path[0] != '/' {
		path = "/" + path
	}
	if path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}

type Handler func(http.ResponseWriter, *http.Request)

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h(w, r)
}

func stripBasePath(mux *runtime.ServeMux, path string) http.Handler {
	return Handler(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, path)
		r.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, path)
		mux.ServeHTTP(w, r)
	})
}

var passThruHeaders = map[string]struct{}{
	common.HeaderXForwardedFor: {},
	common.HeaderXClientId:     {},
	common.HeaderXTraceId:      {},
	common.HeaderXRequestId:    {},
}

func CustomHeaderMatcher(key string) (string, bool) {
	if _, ok := passThruHeaders[strings.ToLower(key)]; ok {
		return key, true
	}
	return runtime.DefaultHeaderMatcher(key)
}
