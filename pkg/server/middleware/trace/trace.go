package trace

import (
	"context"
	"net/http"
	"strings"

	"github.com/KyberNetwork/kutils/klog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/KyberNetwork/service-framework/pkg/common"
	"github.com/KyberNetwork/service-framework/pkg/observe/kmetric"
	"github.com/KyberNetwork/service-framework/pkg/server/grpcserver"
)

const FieldNameRequestId = "request_id"

var internalServerErr = status.New(codes.Internal, http.StatusText(http.StatusInternalServerError))

// UnaryServerInterceptor returns a new unary server interceptor that copies span trace id to response, inject trace log
// to ctx, wraps output error, and records incoming request metrics.
func UnaryServerInterceptor(cfg grpcserver.Config) grpc.UnaryServerInterceptor {
	wrapper := grpcStatusWrapper{cfg: cfg}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (res any,
		err error) {
		if traceId, ok := common.TraceIdFromCtx(ctx); ok {
			traceIdStr := traceId.String()
			_ = grpc.SetHeader(ctx, metadata.Pairs(common.HeaderXTraceId, traceIdStr))
			ctx = klog.CtxWithLogger(ctx,
				klog.WithFields(ctx, klog.Fields{common.LogFieldTraceId: traceIdStr}))
			defer func() {
				if res, ok := res.(proto.Message); ok && res != nil {
					m := res.ProtoReflect()
					if fd := m.Descriptor().Fields().ByName(FieldNameRequestId); fd != nil &&
						fd.Kind() == protoreflect.StringKind && !m.Has(fd) {
						m.Set(fd, protoreflect.ValueOfString(traceIdStr))
					}
				}
			}()
		}

		clientId, code := clientIdFromCtx(ctx), codes.OK
		defer func() {
			method := info.FullMethod[strings.LastIndexByte(info.FullMethod, '/')+1:]
			kmetric.IncIncomingRequest(ctx, clientId, method, code)
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
	cfg grpcserver.Config
}

// GrpcStatus converts original error to GRPC error which will then be converted to HTTP error by grpc-gateway.
func (w grpcStatusWrapper) GrpcStatus(err error) *status.Status {
	if st, ok := status.FromError(err); ok {
		return st
	}
	if st := status.FromContextError(err); st.Code() != codes.Unknown {
		return st
	}
	if w.cfg.Mode != grpcserver.Production { // in development mode, return raw error message.
		return status.New(codes.Internal, err.Error())
	}
	return internalServerErr
}
