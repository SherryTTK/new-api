package model

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenAutoGroupsRoundTripThroughRedisHashCache(t *testing.T) {
	useUserCacheMiniRedis(t)
	token := Token{
		Id:         42,
		UserId:     7,
		Key:        "token-auto-groups-cache-key",
		Name:       "auto-cache",
		Group:      "auto",
		AutoGroups: `["vip","default"]`,
	}

	require.NoError(t, cacheSetTokenForTest(token))
	cached, err := cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.Equal(t, token.AutoGroups, cached.AutoGroups)
	groups, err := cached.GetAutoGroups()
	require.NoError(t, err)
	assert.Equal(t, []string{"vip", "default"}, groups)
}

func TestTokenUpdateSynchronouslyNarrowsPreheatedAutoGroupsCache(t *testing.T) {
	truncateTables(t)
	useUserCacheMiniRedis(t)
	token := Token{
		UserId:          7,
		Key:             "token-auto-groups-update-cache-key",
		Name:            "auto-cache-update",
		Status:          common.TokenStatusEnabled,
		ExpiredTime:     -1,
		UnlimitedQuota:  true,
		Group:           "auto",
		CrossGroupRetry: true,
		AutoGroups:      `["default","vip"]`,
	}
	require.NoError(t, token.Insert())
	require.NoError(t, cacheSetTokenForTest(token))

	preheated, err := cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.JSONEq(t, `["default","vip"]`, preheated.AutoGroups)

	require.NoError(t, token.SetAutoGroups([]string{"vip"}))
	require.NoError(t, token.Update())
	// Update 是限制性变更：写库前删除缓存并设置 fence。缓存不再提供旧的
	// 宽分组值，下一次读取必须看到收紧后的分组。
	_, cacheErr := cacheGetTokenByKey(token.Key)
	require.Error(t, cacheErr, "the pre-update cache entry must be invalidated")
	reloaded, err := GetTokenByKey(token.Key, false)
	require.NoError(t, err)
	assert.JSONEq(t, `["vip"]`, reloaded.AutoGroups)
}

// cacheSetTokenForTest 以测试身份写入完整 token 缓存（含额度字段），
// 模拟“已水合”的缓存状态。
func cacheSetTokenForTest(token Token) error {
	_, err := cacheInitToken(token)
	return err
}

func TestTokenPrivateFeaturesSurviveCacheInitialization(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config *types.FixedResponseConfig
		record *bool
	}{
		{"enabled", &types.FixedResponseConfig{Enabled: true, Content: "固定\nreply", MinDelayMS: 1, MaxDelayMS: 2, CacheReadTokens: 3, CacheWriteTokens: 4}, common.GetPointer(true)},
		{"disabled", &types.FixedResponseConfig{Content: "saved reply"}, common.GetPointer(false)},
		{"unset", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useUserCacheMiniRedis(t)
			token := Token{Id: 42, UserId: 7, Key: "private-features", FixedResponse: tc.config, RecordLastRequest: tc.record}
			result, err := cacheInitToken(token)
			require.NoError(t, err)
			require.Equal(t, 1, result)
			cached, err := cacheGetTokenByKey(token.Key)
			require.NoError(t, err)
			assert.Equal(t, token.FixedResponse, cached.FixedResponse)
			assert.Equal(t, token.RecordLastRequest, cached.RecordLastRequest)
		})
	}
}

func TestTokenLegacyCacheBackfillsPrivateFeaturesWithoutResettingQuota(t *testing.T) {
	server := useUserCacheMiniRedis(t)
	ctx := context.Background()
	token := Token{Id: 42, UserId: 7, Key: "legacy-private-features", RemainQuota: 1000, UsedQuota: 10,
		FixedResponse: &types.FixedResponseConfig{Enabled: true, Content: "local reply"}, RecordLastRequest: common.GetPointer(true)}
	_, err := cacheInitToken(token)
	require.NoError(t, err)
	key := getTokenCacheKey(token.Key)
	require.NoError(t, common.RDB.HDel(ctx, key, "FixedResponse", "RecordLastRequest").Err())
	require.NoError(t, common.RDB.HSet(ctx, key, "RemainQuota", 700, "UsedQuota", 310).Err())
	_, err = cacheGetTokenByKey(token.Key)
	require.Error(t, err, "old hashes must fall back to the database")
	require.NoError(t, common.RDB.Set(ctx, getTokenCacheFenceKey(token.Key), 1, time.Second).Err())
	result, err := cacheInitToken(token)
	require.NoError(t, err)
	assert.Zero(t, result)
	_, err = cacheGetTokenByKey(token.Key)
	require.Error(t, err, "a mutation fence must prevent publishing a stale snapshot")
	server.FastForward(time.Second)
	result, err = cacheInitToken(token)
	require.NoError(t, err)
	assert.Equal(t, 2, result)
	cached, err := cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.Equal(t, token.FixedResponse, cached.FixedResponse)
	assert.Equal(t, token.RecordLastRequest, cached.RecordLastRequest)
	assert.Equal(t, 700, cached.RemainQuota)
	assert.Equal(t, 310, cached.UsedQuota)
	token.FixedResponse = &types.FixedResponseConfig{Enabled: true, Content: "stale reply"}
	token.RecordLastRequest = common.GetPointer(false)
	_, err = cacheInitToken(token)
	require.NoError(t, err)
	cached, err = cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.Equal(t, "local reply", cached.FixedResponse.Content)
	require.NotNil(t, cached.RecordLastRequest)
	assert.True(t, *cached.RecordLastRequest)
}
