package kredis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

type TestSuite struct {
	suite.Suite
	mockRedis *miniredis.Miniredis
	client    redis.UniversalClient
}

func (ts *TestSuite) SetupSuite() {
	ts.mockRedis = miniredis.RunT(ts.T())
	ts.client = redis.NewClient(&redis.Options{
		Addr: ts.mockRedis.Addr(),
	})

	// Populate mock Redis with some data
	ts.mockRedis.HSet("myhash", "key1", "value1", "key2", "value2", "key3", "value3")
}

func (ts *TestSuite) TestHscanOnlyKeys() {
	ctx := context.Background()

	// Test HscanOnlyKeys
	scanCmd := HscanOnlyKeys(ctx, ts.client, "myhash", 0, "", 0)
	keys, cursor, err := scanCmd.Result()

	ts.Nil(err)
	ts.Equal(uint64(0), cursor)
	ts.ElementsMatch([]string{"key1", "key2", "key3"}, keys)

	// Test with match pattern
	scanCmd = HscanOnlyKeys(ctx, ts.client, "myhash", 0, "key[1-2]", 0)
	keys, cursor, err = scanCmd.Result()

	ts.Nil(err)
	ts.Equal(uint64(0), cursor)
	ts.ElementsMatch([]string{"key1", "key2"}, keys)
}

func (ts *TestSuite) TestHscanOnlyValues() {
	ctx := context.Background()

	// Test HscanOnlyValues
	scanCmd := HscanOnlyValues(ctx, ts.client, "myhash", 0, "", 0)
	values, cursor, err := scanCmd.Result()

	ts.Nil(err)
	ts.Equal(uint64(0), cursor)
	ts.ElementsMatch([]string{"value1", "value2", "value3"}, values)

	// Test with match pattern
	scanCmd = HscanOnlyValues(ctx, ts.client, "myhash", 0, "key[1-2]", 0)
	values, cursor, err = scanCmd.Result()

	ts.Nil(err)
	ts.Equal(uint64(0), cursor)
	ts.ElementsMatch([]string{"value1", "value2"}, values)
}
