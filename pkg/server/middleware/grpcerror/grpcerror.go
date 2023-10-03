package grpcerror

import (
	"context"
	"errors"

	"github.com/KyberNetwork/logger"
	"github.com/golang/protobuf/proto"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	keyXTraceID = "x-trace-id"
)

// UnaryServerInterceptor returns a new unary server interceptor that wraps output error.
//
// Output error will be converted to GRPC error before sending to clients.
func UnaryServerInterceptor(development bool, internalServerErr error) grpc.UnaryServerInterceptor {
	w := grpcErrorWrapper{development: development, internalServerErr: internalServerErr}
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {
		if span := trace.SpanContextFromContext(ctx); span.IsValid() {
			_ = grpc.SetHeader(ctx, metadata.Pairs(keyXTraceID, span.TraceID().String()))
		}

		res, err := handler(ctx, req)
		if err != nil {
			return nil, w.GRPCError(err)
		}
		return res, nil
	}
}

// StreamServerInterceptor returns a new streaming server interceptor that wraps outcome error.
//
// The stage at which invalid messages will be rejected with `InvalidArgument` varies based on the
// type of the RPC. For `ServerStream` (1:m) requests, it will happen before reaching any user space
// handlers. For `ClientStream` (n:1) or `BidiStream` (n:m) RPCs, the messages will be rejected on
// calls to `stream.Recv()`.
// func StreamServerInterceptor() grpc.StreamServerInterceptor {
//	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
//		wrapper := &recvWrapper{stream}
//		return handler(srv, wrapper)
//	}
// }

// grpcErrorWrapper is wrapper that convert app level error to GRPC error
type grpcErrorWrapper struct {
	development       bool
	internalServerErr error
}

// GRPCError converts original error to GRPC error which will then be converted to HTTP error by grpc-gateway.
// Error may be wrapped, so must unwrap it to retrieve original error.
func (w grpcErrorWrapper) GRPCError(err error) error {
	wrappedErr := unwrapErr(err)
	if wrappedErr == context.Canceled || wrappedErr == context.DeadlineExceeded {
		return status.FromContextError(wrappedErr).Err()
	}
	stt, ok := status.FromError(wrappedErr)
	if !ok {
		return status.FromContextError(wrappedErr).Err()
	}
	if de, ok := wrappedErr.(interface {
		Details() []proto.Message
	}); ok {
		if s, err := stt.WithDetails(de.Details()...); err == nil {
			stt = s
		}
	}

	// In development mode, return raw error message.
	if w.development {
		logger.WithFields(logger.Fields{"error": err}).Warn("getting error...")
		return status.Error(stt.Code(), err.Error())
	}

	if ok {
		return stt.Err()
	}
	logger.WithFields(logger.Fields{"error": err}).Error("unexpected error...")
	return w.internalServerErr
}

func unwrapErr(err error) error {
	wrappedErr := errors.Unwrap(err)
	if wrappedErr != nil {
		return wrappedErr
	}
	return err
}
