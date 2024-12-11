package reconnectable

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	redis.UniversalClient

	opts *redis.UniversalOptions

	lastRefreshTime   atomic.Value
	refreshCooldown   time.Duration
	refreshInProgress atomic.Bool
}

func New(cfg *redis.UniversalOptions) *RedisClient {
	rc := &RedisClient{
		opts:            cfg,
		refreshCooldown: 10 * time.Second,
	}

	rc.UniversalClient = redis.NewUniversalClient(cfg)
	rc.UniversalClient.AddHook(rc)

	return rc
}

func (r *RedisClient) canRefreshClient() bool {
	lastRefresh, ok := r.lastRefreshTime.Load().(time.Time)
	if !ok {
		return true
	}
	return time.Since(lastRefresh) >= r.refreshCooldown
}

func (r *RedisClient) refreshClient() {
	if r.refreshInProgress.Load() || !r.refreshInProgress.CompareAndSwap(false, true) {
		return
	}

	defer r.refreshInProgress.Store(false)

	if !r.canRefreshClient() {
		return
	}

	newClient := redis.NewUniversalClient(r.opts)

	oldClient := r.UniversalClient
	r.UniversalClient = newClient
	r.UniversalClient.AddHook(r)
	r.lastRefreshTime.Store(time.Now())

	go func() {
		if oldClient != nil {
			_ = oldClient.Close()
		}
	}()
}

func shouldRefreshClient(err error) bool {
	return err != nil && strings.Contains(err.Error(), "connection refused")
}

func (r *RedisClient) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (r *RedisClient) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		if shouldRefreshClient(err) {
			r.refreshClient()
		}

		return err
	}
}

func (r *RedisClient) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		err := next(ctx, cmds)
		if shouldRefreshClient(err) {
			r.refreshClient()
		}

		return err
	}
}
