package common_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This optional integration test uses an isolated Redis Unix socket.
func TestRedisJSONFieldsRoundTrip(t *testing.T) {
	socket := os.Getenv("FIXED_RESPONSE_TEST_REDIS_SOCKET")
	if socket == "" {
		t.Skip("set FIXED_RESPONSE_TEST_REDIS_SOCKET for the Redis integration test")
	}
	client := redis.NewClient(&redis.Options{Network: "unix", Addr: socket})
	previous := common.RDB
	common.RDB = client
	t.Cleanup(func() { common.RDB = previous; assert.NoError(t, client.Close()) })
	require.NoError(t, client.Ping(context.Background()).Err())
	type cachedToken struct {
		FixedResponse *types.FixedResponseConfig `gorm:"serializer:json;type:text"`
		RemainQuota   int
	}
	key := "fixed-response-test:" + common.GetUUID()
	defer client.Del(context.Background(), key)
	for _, config := range []*types.FixedResponseConfig{
		{Enabled: true, MinDelayMS: 1, MaxDelayMS: 200, Content: "固定\n\"reply\""},
		{Enabled: false, Content: "retained content"}, nil,
	} {
		expected := cachedToken{FixedResponse: config, RemainQuota: 1234}
		require.NoError(t, common.RedisHSetObj(key, &expected, time.Minute))
		var actual cachedToken
		require.NoError(t, common.RedisHGetObj(key, &actual))
		assert.Equal(t, expected, actual)
	}
	// Rolling upgrades must read old hashes without the newly added field.
	require.NoError(t, client.HDel(context.Background(), key, "FixedResponse").Err())
	var legacy cachedToken
	require.NoError(t, common.RedisHGetObj(key, &legacy))
	assert.Nil(t, legacy.FixedResponse)
	assert.Equal(t, 1234, legacy.RemainQuota)
}
