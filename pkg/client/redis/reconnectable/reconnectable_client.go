package reconredis

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	redis.UniversalClient
	redisFactory func() redis.UniversalClient

	refreshCooldown   time.Duration
	refreshInProgress atomic.Bool
	lastRefreshTime   time.Time
}

func New(redisFactory func() redis.UniversalClient) *Client {
	return (&Client{
		redisFactory:    redisFactory,
		refreshCooldown: 10 * time.Second,
	}).refreshClient()
}

func (r *Client) refreshClient() *Client {
	client := r.redisFactory()
	client.AddHook(r)
	r.UniversalClient = client
	return r
}

func (r *Client) canRefreshClient() bool {
	return time.Since(r.lastRefreshTime) >= r.refreshCooldown
}

func (r *Client) attemptRefreshClient() {
	if r.refreshInProgress.Load() || !r.refreshInProgress.CompareAndSwap(false, true) {
		return
	}
	defer r.refreshInProgress.Store(false)

	if !r.canRefreshClient() {
		return
	}
	defer func() {
		r.lastRefreshTime = time.Now()
	}()

	oldClient := r.UniversalClient
	r.refreshClient()
	go func() {
		if oldClient != nil {
			_ = oldClient.Close()
		}
	}()
}

func shouldRefreshClient(err error) bool {
	return err != nil && strings.Contains(err.Error(), "connection refused")
}

func (r *Client) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (r *Client) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		if shouldRefreshClient(err) {
			r.attemptRefreshClient()
		}

		return err
	}
}

func (r *Client) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		err := next(ctx, cmds)
		if shouldRefreshClient(err) {
			r.attemptRefreshClient()
		}

		return err
	}
}
