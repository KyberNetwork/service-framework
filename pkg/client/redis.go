package client

import (
	"context"
	"time"

	"github.com/KyberNetwork/kyber-trace-go/pkg/metric"
	"github.com/KyberNetwork/kyber-trace-go/pkg/tracer"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/KyberNetwork/service-framework/pkg/observe/klog"
)

const RedisCloseDelay = time.Minute

type RedisCfg struct {
	redis.UniversalOptions `mapstructure:",squash"`
	C                      redis.UniversalClient
}

func (*RedisCfg) OnUpdate(old, new *RedisCfg) {
	ctx := context.Background()
	new.C = redis.NewUniversalClient(&new.UniversalOptions)
	if metric.Provider() != nil {
		if err := redisotel.InstrumentMetrics(new.C); err != nil {
			klog.Errorf(ctx, "RedisCfg.OnUpdate|redisotel.InstrumentMetrics failed|err=%v", err)
		}
	}
	if tracer.Provider() != nil {
		if err := redisotel.InstrumentTracing(new.C); err != nil {
			klog.Errorf(ctx, "RedisCfg.OnUpdate|redisotel.InstrumentTracing failed|err=%v", err)
		}
	}
	if old != nil && old.C != nil {
		time.AfterFunc(RedisCloseDelay, func() {
			if err := old.C.Close(); err != nil {
				klog.Errorf(ctx, "RedisCfg.OnUpdate|old.C.Close() failed|err=%v", err)
			}
		})
	}
}
