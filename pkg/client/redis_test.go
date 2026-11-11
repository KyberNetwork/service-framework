package client

import (
	"fmt"
	"testing"

	"github.com/KyberNetwork/kutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRedisPassword = "test-redis-password"

// A service config that embeds RedisCfg, as services log it with "cfg=%+v".
type testServiceCfg struct {
	Redis RedisCfg
}

func TestRedisCfgRedactsPasswordsWhenPrinted(t *testing.T) {
	var cfg RedisCfg
	cfg.Addrs = []string{"redis:6379"}
	cfg.Password = testRedisPassword
	cfg.SentinelPassword = testRedisPassword

	for name, v := range map[string]any{"value": cfg, "pointer": &cfg, "service config": testServiceCfg{Redis: cfg}} {
		for _, out := range []string{
			fmt.Sprintf("%+v", v), fmt.Sprintf("%v", v), fmt.Sprintf("%#v", v), fmt.Sprintf("%s", v),
		} {
			assert.NotContains(t, out, testRedisPassword, "%s: %s", name, out)
			assert.Contains(t, out, redacted, "%s must still show that a password is set", name)
			assert.Contains(t, out, "redis:6379", "%s: other fields stay visible", name)
		}
	}
	assert.Equal(t, testRedisPassword, cfg.Password, "printing must not change the config")
}

func TestRedisCfgEmptyPasswordPrintsEmpty(t *testing.T) {
	var cfg RedisCfg
	assert.NotContains(t, fmt.Sprintf("%+v", &cfg), redacted, "a missing password must stay visible as empty")
}

// hotcfg reloads from ks-setting decode JSON onto a copy of the initial config.
func TestRedisCfgDecodesConfig(t *testing.T) {
	var cfg RedisCfg
	cfg.Password = testRedisPassword
	require.NoError(t, kutils.DecodeConfig([]byte(`{"addrs":["a:6379"],"masterName":"mymaster"}`), &cfg))
	assert.Equal(t, []string{"a:6379"}, cfg.Addrs)
	assert.Equal(t, "mymaster", cfg.MasterName)
	assert.Equal(t, testRedisPassword, cfg.Password, "a key the payload omits must survive")
}

func TestRedisCfgOnUpdateBuildsClient(t *testing.T) {
	cfg := &RedisCfg{}
	cfg.Addrs = []string{"localhost:6379"}
	cfg.Password = testRedisPassword
	cfg.OnUpdate(nil, cfg)

	require.NotNil(t, cfg.C)
	t.Cleanup(func() { _ = cfg.C.Close() })
	assert.True(t, cfg.RouteRandomly)
	assert.Equal(t, testRedisPassword, cfg.Password, "the client gets the real password")
}
