package client

import (
	"context"
	"fmt"
	"time"

	"github.com/KyberNetwork/kutils/klog"
	"github.com/KyberNetwork/kyber-trace-go/pkg/metric"
	"github.com/KyberNetwork/kyber-trace-go/pkg/tracer"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	reconredis "github.com/KyberNetwork/service-framework/pkg/client/redis/reconnectable"
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

const redacted = "[REDACTED]"

// redisCfgFields has the fields of RedisCfg and none of its methods, so
// formatting it does not call back into RedisCfg.String. A field added to
// RedisCfg is printed too.
type redisCfgFields RedisCfg

// String hides the passwords. Services log their config with %+v, and hotcfg
// logs every update with "Cfg.Update|from=%+v,to=%+v".
func (c RedisCfg) String() string { return c.format("%+v") }

// GoString hides the passwords for %#v.
func (c RedisCfg) GoString() string { return c.format("%#v") }

func (c RedisCfg) format(verb string) string {
	fields := redisCfgFields(c)
	fields.Password = redactString(fields.Password)
	fields.SentinelPassword = redactString(fields.SentinelPassword)
	return fmt.Sprintf(verb, fields)
}

// redactString keeps an empty value empty, so a missing password stays visible.
func redactString(s string) string {
	if s == "" {
		return ""
	}
	return redacted
}

func NewRedisClient(ctx context.Context, opts *redis.UniversalOptions) redis.UniversalClient {
	if opts.MasterName == "" {
		return reconredis.New(func() redis.UniversalClient {
			return InstrumentRedisOtel(ctx, redis.NewUniversalClient(opts))
		})
	}
	failoverOpts := opts.Failover()
	failoverOpts.RouteByLatency = opts.RouteByLatency
	failoverOpts.RouteRandomly = opts.RouteRandomly
	return InstrumentRedisOtel(ctx, redis.NewFailoverClusterClient(failoverOpts))
}

func InstrumentRedisOtel(ctx context.Context, client redis.UniversalClient) redis.UniversalClient {
	if metric.Provider() != nil {
		if err := redisotel.InstrumentMetrics(client); err != nil {
			klog.Errorf(ctx, "InstrumentRedisOtel|redisotel.InstrumentMetrics failed|err=%v", err)
		}
	}
	if tracer.Provider() != nil {
		if err := redisotel.InstrumentTracing(client); err != nil {
			klog.Errorf(ctx, "InstrumentRedisOtel|redisotel.InstrumentTracing failed|err=%v", err)
		}
	}
	return client
}
