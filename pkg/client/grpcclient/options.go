package grpcclient

import (
	"crypto/tls"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"
)

type Compression string

const (
	// NoCompression tells the driver to send payloads without
	// compression.
	NoCompression Compression = ""
	// GzipCompression tells the driver to send payloads after
	// compressing them with gzip.
	GzipCompression = gzip.Name
)

type ApplyOption func(*Config)

func WithConfig(cfg *Config) ApplyOption {
	return func(c *Config) {
		if cfg != nil {
			*c = *cfg
		}
	}
}

func WithClientID(clientID string) ApplyOption {
	return func(c *Config) {
		c.ClientID = clientID
	}
}

func WithHeaders(headers map[string]string) ApplyOption {
	return func(c *Config) {
		c.Headers = headers
	}
}

func AddHeader(key, value string) ApplyOption {
	return func(c *Config) {
		if c.Headers == nil {
			c.Headers = make(map[string]string)
		}
		c.Headers[key] = value
	}
}

func WithTimeout(timeout time.Duration) ApplyOption {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

func WithCompression(compression Compression) ApplyOption {
	return func(c *Config) {
		c.Compression = compression
	}
}

func WithBlock() ApplyOption {
	return func(c *Config) {
		c.IsBlockConnect = true
	}
}

func WithInsecure() ApplyOption {
	return func(c *Config) {
		c.Insecure = true
	}
}

func WithBaseURL(baseURL string) ApplyOption {
	return func(c *Config) {
		c.BaseURL = baseURL
	}
}

func WithTLS(tlsCfg *tls.Config) ApplyOption {
	return func(c *Config) {
		c.GRPCCredentials = credentials.NewTLS(tlsCfg)
	}
}

func WithReconnectionPeriod(reconnectionPeriod time.Duration) ApplyOption {
	return func(c *Config) {
		c.ReconnectionPeriod = reconnectionPeriod
	}
}

func WithDialOption(opts ...grpc.DialOption) ApplyOption {
	return func(c *Config) {
		c.DialOptions = opts
	}
}
