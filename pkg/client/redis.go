package client

import (
	"context"
	"time"

	"github.com/KyberNetwork/kutils/klog"
	"github.com/KyberNetwork/kyber-trace-go/pkg/metric"
	"github.com/KyberNetwork/kyber-trace-go/pkg/tracer"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/KyberNetwork/service-framework/pkg/client/redis/reconnectable"
)

const RedisCloseDelay = time.Minute

// RedisCfg is hotcfg for redis client. On update, it
// creates FailoverClusterClient with RouteRandomly for sentinel redis by default
// as well as instrumenting the client for metrics and tracing.
type RedisCfg struct {
	redis.UniversalOptions `mapstructure:",squash"`
	DisableRouteRandomly   bool
	C                      redis.UniversalClient
}

func (*RedisCfg) OnUpdate(old, new *RedisCfg) {
	ctx := context.Background()
	if old != nil && old.C != nil {
		oldC := old.C
		time.AfterFunc(RedisCloseDelay, func() {
			if err := oldC.Close(); err != nil {
				klog.Errorf(ctx, "RedisCfg.OnUpdate|old.C.Close() failed|err=%v", err)
			}
		})
	}
	new.RouteRandomly = new.RouteRandomly || !new.DisableRouteRandomly
	new.C = NewRedisClient(ctx, &new.UniversalOptions)
}

func NewRedisClient(ctx context.Context, opts *redis.UniversalOptions) redis.UniversalClient {
	if opts.MasterName == "" {
		return reconredis.New(func() redis.UniversalClient {
			return instrumentRedisOtel(ctx, redis.NewUniversalClient(opts))
		})
	}
	failoverOpts := opts.Failover()
	failoverOpts.RouteByLatency = opts.RouteByLatency
	failoverOpts.RouteRandomly = opts.RouteRandomly
	return instrumentRedisOtel(ctx, redis.NewFailoverClusterClient(failoverOpts))
}

func instrumentRedisOtel(ctx context.Context, client redis.UniversalClient) redis.UniversalClient {
	if metric.Provider() != nil {
		if err := redisotel.InstrumentMetrics(client); err != nil {
			klog.Errorf(ctx, "instrumentRedisOtel|redisotel.InstrumentMetrics failed|err=%v", err)
		}
	}
	if tracer.Provider() != nil {
		if err := redisotel.InstrumentTracing(client); err != nil {
			klog.Errorf(ctx, "instrumentRedisOtel|redisotel.InstrumentTracing failed|err=%v", err)
		}
	}
	return client
}
