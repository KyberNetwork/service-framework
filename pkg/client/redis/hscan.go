package kredis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

const LuaHscanOnlyKeys = `
	local results = {}
	local scan_result = redis.call('HSCAN', KEYS[1], unpack(ARGV))
	for i = 1, #scan_result[2], 2 do
		table.insert(results, scan_result[2][i])
	end
	return {scan_result[1], results}
`

// HscanOnlyKeys scans only keys (no values) from a redis hash
func HscanOnlyKeys(ctx context.Context, client redis.UniversalClient, key string, cursor uint64, match string,
	count int64) *redis.ScanCmd {
	args := []any{"eval", LuaHscanOnlyKeys, 1, key, cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	c := client.Process
	cmd := redis.NewScanCmd(ctx, c, args...)
	_ = c(ctx, cmd)
	return cmd
}

const LuaHscanOnlyValues = `
	local results = {}
	local scan_result = redis.call('HSCAN', KEYS[1], unpack(ARGV))
	for i = 2, #scan_result[2], 2 do
		table.insert(results, scan_result[2][i])
	end
	return {scan_result[1], results}
`

// HscanOnlyValues scans only values (no keys) from a redis hash
func HscanOnlyValues(ctx context.Context, client redis.UniversalClient, key string, cursor uint64, match string,
	count int64) *redis.ScanCmd {
	args := []any{"eval", LuaHscanOnlyValues, 1, key, cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	c := client.Process
	cmd := redis.NewScanCmd(ctx, c, args...)
	_ = c(ctx, cmd)
	return cmd
}
