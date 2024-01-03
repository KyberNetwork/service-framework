package trace

import (
	"context"

	"github.com/KyberNetwork/kutils/klog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/KyberNetwork/service-framework/pkg/common"
	"github.com/KyberNetwork/service-framework/pkg/observe/kmetric"
)

// UnaryServerInterceptor returns a new unary server interceptor that copies span trace id to response, inject trace log
// to ctx, wraps output error, and records incoming request metrics.
func UnaryServerInterceptor(isDevMode bool, internalServerErr *status.Status) grpc.UnaryServerInterceptor {
	wrapper := grpcStatusWrapper{development: isDevMode, internalServerErr: internalServerErr}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (res any,
		err error) {
		if traceId, ok := common.TraceIdFromCtx(ctx); ok {
			traceIdStr := traceId.String()
			_ = grpc.SetHeader(ctx, metadata.Pairs(common.HeaderXTraceId, traceIdStr))
			ctx = klog.CtxWithLogger(ctx,
				klog.WithFields(ctx, klog.Fields{common.LogFieldTraceId: traceIdStr}))
		}

		clientId, code := clientIdFromCtx(ctx), codes.OK
		defer func() {
			kmetric.IncIncomingRequest(ctx, clientId, code)
		}()
		res, err = handler(ctx, req)
		if err != nil {
			st := wrapper.GrpcStatus(err)
			code = st.Code()
			return nil, st.Err()
		}
		return res, nil
	}
}

func clientIdFromCtx(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	header := md[common.HeaderXClientId]
	if len(header) == 0 {
		return common.ClientIdUnknown
	}
	return header[0]
}

// StreamServerInterceptor returns a new streaming server interceptor that wraps outcome error.
//
// The stage at which invalid messages will be rejected with `InvalidArgument` varies based on the
// type of the RPC. For `ServerStream` (1:m) requests, it will happen before reaching any user space
// handlers. For `ClientStream` (n:1) or `BidiStream` (n:m) RPCs, the messages will be rejected on
// calls to `stream.Recv()`.
// func StreamServerInterceptor() grpc.StreamServerInterceptor {
//	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
//		wrapper := &recvWrapper{stream}
//		return handler(srv, wrapper)
//	}
// }

// grpcStatusWrapper is wrapper that convert app level error to GRPC error
type grpcStatusWrapper struct {
	development       bool
	internalServerErr *status.Status
}

// GrpcStatus converts original error to GRPC error which will then be converted to HTTP error by grpc-gateway.
func (w grpcStatusWrapper) GrpcStatus(err error) *status.Status {
	if st, ok := status.FromError(err); ok {
		return st
	}
	if st := status.FromContextError(err); st.Code() != codes.Unknown {
		return st
	}
	if w.development { // in development mode, return raw error message.
		return status.New(codes.Internal, err.Error())
	}
	return w.internalServerErr
}
