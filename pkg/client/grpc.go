package client

import (
	"context"
	"time"

	"github.com/KyberNetwork/kutils/klog"
	"google.golang.org/grpc"

	"github.com/KyberNetwork/service-framework/pkg/client/grpcclient"
)

const GrpcCloseDelay = time.Minute

// GrpcCfg is hotcfg for grpc client. On update, it
// creates a new grpc client with the provided factory generated by grpc using service proto.
// The client has interceptors for adding client id header, validating requests, adding timeout, metrics and tracing.
type GrpcCfg[T any] struct {
	grpcclient.Config `mapstructure:",squash"`
	C                 T // the inner grpc client

	clientFactory func(grpc.ClientConnInterface) T // must be provided by user, generated from proto file
	grpcClient    *grpcclient.Client[T]            // wrapper grpc client to close connection
}

// WithFactory sets the config's clientFactory, generated by service proto.
func (c *GrpcCfg[T]) WithFactory(clientFactory func(grpc.ClientConnInterface) T) *GrpcCfg[T] {
	c.clientFactory = clientFactory
	return c
}

func (*GrpcCfg[T]) OnUpdate(old, new *GrpcCfg[T]) {
	ctx := context.Background()

	if old != nil {
		oldGrpcClient := old.grpcClient
		time.AfterFunc(GrpcCloseDelay, func() {
			if err := oldGrpcClient.Close(); err != nil {
				klog.Errorf(ctx, "GrpcCfg.OnUpdate|old.grpcClient.Close() failed|err=%v", err)
			}
		})
	}

	var err error
	new.grpcClient, err = grpcclient.New(new.clientFactory, grpcclient.WithConfig(&new.Config))
	if err != nil {
		klog.Errorf(ctx, "GrpcCfg.OnUpdate|grpcclient.New failed|err=%v", err)
		return
	}

	new.C = new.grpcClient.C
}
