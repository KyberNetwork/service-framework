package client

import (
	"time"

	"github.com/KyberNetwork/kyber-trace-go/pkg/metric"
	"github.com/KyberNetwork/kyber-trace-go/pkg/tracer"
	"github.com/KyberNetwork/logger"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

const RedisCloseDelay = time.Minute

type RedisCfg struct {
	redis.UniversalOptions `mapstructure:",squash"`
	C                      redis.UniversalClient
}

func (*RedisCfg) OnUpdate(old, new *RedisCfg) {
	new.C = redis.NewUniversalClient(&new.UniversalOptions)
	if metric.Provider() != nil {
		if err := redisotel.InstrumentMetrics(new.C); err != nil {
			logger.Errorf("RedisCfg.OnUpdate|redisotel.InstrumentMetrics failed|err=%v", err)
		}
	}
	if tracer.Provider() != nil {
		if err := redisotel.InstrumentTracing(new.C); err != nil {
			logger.Errorf("RedisCfg.OnUpdate|redisotel.InstrumentTracing failed|err=%v", err)
		}
	}
	if old != nil && old.C != nil {
		time.AfterFunc(RedisCloseDelay, func() {
			if err := old.C.Close(); err != nil {
				logger.Errorf("RedisCfg.OnUpdate|old.C.Close() failed|err=%v", err)
			}
		})
	}
}
