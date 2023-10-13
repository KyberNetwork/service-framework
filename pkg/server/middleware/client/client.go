package client

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/KyberNetwork/service-framework/pkg/metric"
)

var xClientIDKey = "x-client-id"
var clientIDUnknown = "unknown"

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (any, error) {
		clientID := extractClientIDFromContext(ctx)
		metric.IncClientRequestTotal(ctx, clientID)
		return handler(ctx, req)
	}
}

func extractClientIDFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return clientIDUnknown
	}
	header, ok := md[xClientIDKey]
	if !ok || len(header) == 0 {
		return clientIDUnknown
	}
	return header[0]
}
