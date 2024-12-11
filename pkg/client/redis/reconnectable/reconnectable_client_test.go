package reconnectable

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshClientWhenServerCrash(t *testing.T) {
	// Step 1: Start the mock Redis server
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("could not start mock Redis server: %v", err)
	}
	defer srv.Close()

	// Step 2: Create the Redis client with the mock server's address
	cfg := &redis.UniversalOptions{
		Addrs: []string{srv.Addr()},
	}
	rc := New(cfg)
	oldClient := rc.UniversalClient

	// Step 3: Simulate server failure by stopping the Redis server
	srv.Close()

	// Step 4: Perform concurrent Set operations that should fail due to the server being down
	var wg sync.WaitGroup
	for i := 1; i <= 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rc.Set(context.Background(), fmt.Sprint(i), "1", 0) // This should fail due to the server being down
		}(i)
	}
	wg.Wait()

	// Step 5: Ensure the client was refreshed after the error (server down)
	newClient := rc.UniversalClient

	// Check that the client refresh was triggered and the refresh time is updated
	require.NotNil(t, rc.lastRefreshTime)
	lastRefresh := rc.lastRefreshTime.Load().(time.Time)
	assert.True(t, time.Since(lastRefresh) < time.Second, "Refresh time should be updated")

	// Verify that the client refresh process was completed (no ongoing refresh)
	assert.False(t, rc.refreshInProgress.Load())

	// Ensure that a new client instance was created
	assert.NotEqual(t, oldClient, newClient, "UniversalClient should be updated")

	// Step 6: Restart the Redis server to simulate recovery
	srv.Start()

	// Step 7: Verify that the client can now successfully ping the server
	require.NoError(t, rc.Ping(context.Background()).Err(), "Ping should succeed after client refresh and server recovery")

	// Verify that the refresh time hasn't been updated since the recovery (no new refresh required)
	assert.True(t, lastRefresh == rc.lastRefreshTime.Load().(time.Time), "Refresh time shouldn't be updated")

	// Ensure that the client is still the refreshed one (not the old client)
	assert.Equal(t, newClient, rc.UniversalClient, "UniversalClient should still be the updated one")
}

func BenchmarkSetCommandWithWrappedClient(b *testing.B) {
	srv, err := miniredis.Run()
	if err != nil {
		b.Fatalf("could not start mock Redis server: %v", err)
	}
	defer srv.Close()

	cfg := &redis.UniversalOptions{
		Addrs: []string{srv.Addr()},
	}

	rc := New(cfg)
	b.ResetTimer()

	b.Run("SetCommand", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := rc.Set(context.Background(), "key", "value", 0).Err()
			if err != nil {
				b.Fatalf("Failed to set key: %v", err)
			}
		}
	})
}

func BenchmarkGetCommandWithWrappedClient(b *testing.B) {
	srv, err := miniredis.Run()
	if err != nil {
		b.Fatalf("could not start mock Redis server: %v", err)
	}
	defer srv.Close()

	cfg := &redis.UniversalOptions{
		Addrs: []string{srv.Addr()},
	}

	rc := New(cfg)
	rc.Set(context.Background(), "key", "value", 0)

	b.ResetTimer()

	b.Run("GetCommand", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := rc.Get(context.Background(), "key").Result()
			if err != nil {
				b.Fatalf("Failed to get key: %v", err)
			}
		}
	})
}

func BenchmarkGetCommandWithOriginalClient(b *testing.B) {
	srv, err := miniredis.Run()
	if err != nil {
		b.Fatalf("could not start mock Redis server: %v", err)
	}
	defer srv.Close()

	cfg := &redis.UniversalOptions{
		Addrs: []string{srv.Addr()},
	}

	rc := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:            cfg.Addrs,
		MasterName:       cfg.MasterName,
		Password:         cfg.Password,
		SentinelPassword: cfg.SentinelPassword,
		DB:               cfg.DB,
		ReadTimeout:      cfg.ReadTimeout,
		WriteTimeout:     cfg.WriteTimeout,
	})
	rc.Set(context.Background(), "key", "value", 0)

	b.ResetTimer()

	b.Run("GetCommand", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := rc.Get(context.Background(), "key").Result()
			if err != nil {
				b.Fatalf("Failed to get key: %v", err)
			}
		}
	})
}

func BenchmarkSetCommandWithOriginalClient(b *testing.B) {
	srv, err := miniredis.Run()
	if err != nil {
		b.Fatalf("could not start mock Redis server: %v", err)
	}
	defer srv.Close()

	cfg := &redis.UniversalOptions{
		Addrs: []string{srv.Addr()},
	}

	rc := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:            cfg.Addrs,
		MasterName:       cfg.MasterName,
		Password:         cfg.Password,
		SentinelPassword: cfg.SentinelPassword,
		DB:               cfg.DB,
		ReadTimeout:      cfg.ReadTimeout,
		WriteTimeout:     cfg.WriteTimeout,
	})

	b.ResetTimer()

	b.Run("SetCommand", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := rc.Set(context.Background(), "key", "value", 0).Err()
			if err != nil {
				b.Fatalf("Failed to set key: %v", err)
			}
		}
	})
}
