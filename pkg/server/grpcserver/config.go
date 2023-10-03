package grpcserver

import (
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

// DefaultConfig return a default server config
func DefaultConfig() *Config {
	return &Config{
		GRPC: Listen{
			Host: "0.0.0.0",
			Port: 10443,
		},
		HTTP: Listen{
			Host: "0.0.0.0",
			Port: 10080,
		},
	}
}

type (
	Server struct {
		gRPC      *grpc.Server
		mux       *runtime.ServeMux
		cfg       *Config
		isDevMode bool
	}

	// Config hold http/grpc server config
	Config struct {
		Mode string
		GRPC Listen
		HTTP Listen
		Flag Flag
	}

	Flag struct {
		GRPC GRPCFlag
	}

	GRPCFlag struct {
		DisableLogRequest  bool
		DisableLogResponse bool
	}

	// Listen config for host/port socket listener
	Listen struct {
		Host string
		Port int
	}
)
