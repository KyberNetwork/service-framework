package logging

import (
	"context"
	"time"

	"github.com/KyberNetwork/kutils/klog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/KyberNetwork/service-framework/pkg/common"
)

// InterceptorLogger requires Log method, allowing logging interceptor to be interoperable.
type InterceptorLogger interface {
	Log(ctx context.Context, meta CallMeta, req any, resp any, err error, duration time.Duration)
}

// LoggerFunc is a function that also implements InterceptorLogger interface.
type LoggerFunc func(ctx context.Context, meta CallMeta, req any, resp any, err error, duration time.Duration)

func (f LoggerFunc) Log(ctx context.Context, meta CallMeta, req any, resp any, err error, duration time.Duration) {
	f(ctx, meta, req, resp, err, duration)
}

type opt struct {
	ignoreReq  map[string]struct{}
	ignoreResp map[string]struct{}
}

func IgnoreReq(ignoreReq ...string) func(opt *opt) {
	return func(opt *opt) {
		for _, v := range ignoreReq {
			opt.ignoreReq[v] = struct{}{}
		}
	}
}

func IgnoreResp(ignoreResp ...string) func(opt *opt) {
	return func(opt *opt) {
		for _, v := range ignoreResp {
			opt.ignoreResp[v] = struct{}{}
		}
	}
}

const ignored = "<...>"

func DefaultLogger(loggerFromCtx func(context.Context) Logger, opts ...func(opt *opt)) LoggerFunc {
	opt := &opt{
		ignoreReq:  make(map[string]struct{}),
		ignoreResp: make(map[string]struct{}),
	}
	for _, o := range opts {
		o(opt)
	}
	return func(ctx context.Context, meta CallMeta, req any, resp any, err error, duration time.Duration) {
		code := status.Code(err)
		if _, ok := opt.ignoreReq[meta.FullMethod]; ok {
			req = ignored
		}
		if _, ok := opt.ignoreResp[meta.FullMethod]; ok {
			resp = ignored
		}
		from := metadata.ValueFromIncomingContext(ctx, common.HeaderXForwardedFor)
		if peerInfo, ok := peer.FromContext(ctx); ok {
			from = append(from, peerInfo.Addr.String())
		}
		Logf(loggerFromCtx(ctx), code,
			"cmd=%s|code=%d|err=%v|req=%+v|resp=%+v|dur=%s|from=%v",
			meta.FullMethod, code, err, req, resp, duration, from)
	}
}

func KlogLogger(ctx context.Context) Logger {
	return klog.LoggerFromCtx(ctx)
}

// Logger is a logger with infof/warnf/errorf methods.
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// Logf is the helper mapper that maps gRPC return codes to log levels for server side logging.
func Logf(log Logger, code codes.Code, format string, args ...any) {
	switch code {
	case codes.OK, codes.NotFound, codes.Canceled, codes.AlreadyExists, codes.InvalidArgument, codes.Unauthenticated:
		log.Infof(format, args...)

	case codes.DeadlineExceeded, codes.PermissionDenied, codes.ResourceExhausted, codes.FailedPrecondition, codes.Aborted,
		codes.OutOfRange, codes.Unavailable:
		log.Warnf(format, args...)

	case codes.Unknown, codes.Unimplemented, codes.Internal, codes.DataLoss:
		log.Errorf(format, args...)

	default:
		log.Errorf(format, args...)
	}
}
