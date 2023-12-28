package grpcserver

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
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
		*cfg = *DefaultConfig()
	}
	return &Server{
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

		cfg:     cfg,
		AppMode: appMode,
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
func (s *Server) Serve() error {
	stop := make(chan os.Signal, 1)
	errCh := make(chan error)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	httpMux := http.NewServeMux()
	httpMux.Handle("/", s.mux)

	httpServer := http.Server{
		Addr:    s.cfg.HTTP.String(),
		Handler: httpMux,
	}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			errCh <- err
		}
	}()
	go func() {
		listener, err := net.Listen("tcp", s.cfg.GRPC.String())
		if err != nil {
			errCh <- err
			return
		}
		if err := s.gRPC.Serve(listener); err != nil {
			errCh <- err
		}
	}()
	for {
		select {
		case <-stop:
			ctx := context.Background()
			if err := httpServer.Shutdown(ctx); err != nil {
				return err
			}
			s.gRPC.GracefulStop()
			return nil
		case err := <-errCh:
			return err
		}
	}
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
