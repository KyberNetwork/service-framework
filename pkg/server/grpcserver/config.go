package grpcserver

import (
	"net"
	"strconv"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

// DefaultConfig return a default server config
func DefaultConfig() *Config {
	return &Config{
		GRPC: Listen{
			Host: "0.0.0.0",
			Port: 9080,
		},
		HTTP: Listen{
			Host: "0.0.0.0",
			Port: 8080,
		},
	}
}

type (
	Server struct {
		gRPC *grpc.Server
		mux  *runtime.ServeMux
		cfg  *Config
		AppMode
	}

	// Config hold http/grpc server config
	Config struct {
		Mode string
		GRPC Listen
		HTTP Listen
		Flag Flag
	}

	Flag struct {
		DisableLogRequest  bool
		DisableLogResponse bool
	}

	// Listen config for host/port socket listener
	Listen struct {
		Host string
		Port int
	}
)

// String return socket listen DSN
func (l *Listen) String() string {
	return net.JoinHostPort(l.Host, strconv.Itoa(l.Port))
}
