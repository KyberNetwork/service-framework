package logging

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// UnaryServerInterceptor is a gRPC server-side interceptor that provides logging for Unary RPCs.
func UnaryServerInterceptor(log InterceptorLogger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		startTime := time.Now()
		resp, err := handler(ctx, req)
		log.Log(ctx, CallMeta{info.FullMethod, UnaryMethodType}, req, resp, err, time.Since(startTime))
		return resp, err
	}
}

// StreamServerInterceptor is a gRPC server-side interceptor that provides logging for Streaming RPCs.
func StreamServerInterceptor(log InterceptorLogger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		meta := CallMeta{info.FullMethod, StreamMethodType}
		err := handler(srv, &monitoredServerStream{ServerStream: ss, log: log, meta: meta})
		log.Log(ss.Context(), meta, nil, nil, err, time.Since(startTime))
		return err
	}
}

type CallMeta struct {
	FullMethod string
	MethodType MethodType
}

type MethodType string

const (
	UnaryMethodType  MethodType = "unary"
	StreamMethodType MethodType = "stream"
)

// monitoredStream wraps grpc.ServerStream allowing each Sent/Recv of message to report.
type monitoredServerStream struct {
	grpc.ServerStream
	log  InterceptorLogger
	meta CallMeta
}

func (s *monitoredServerStream) SendMsg(m any) error {
	startTime := time.Now()
	err := s.ServerStream.SendMsg(m)
	s.log.Log(s.ServerStream.Context(), s.meta, nil, m, err, time.Since(startTime))
	return err
}

func (s *monitoredServerStream) RecvMsg(m any) error {
	startTime := time.Now()
	err := s.ServerStream.RecvMsg(m)
	s.log.Log(s.ServerStream.Context(), s.meta, m, nil, err, time.Since(startTime))
	return err
}
