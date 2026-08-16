package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useInflightMiniRedis swaps common.RDB for a miniredis instance for the
// duration of the test and restores the originals on cleanup.
func useInflightMiniRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	prevEnabled := common.RedisEnabled
	prevClient := common.RDB
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, client.Ping(context.Background()).Err())
	common.RedisEnabled = true
	common.RDB = client
	t.Cleanup(func() {
		_ = client.Close()
		common.RedisEnabled = prevEnabled
		common.RDB = prevClient
	})
	return server
}

func TestStartFinishList(t *testing.T) {
	useInflightMiniRedis(t)

	requestID := "req-001"
	ctx := context.Background()

	// Simulate what Start does — we test Start's Redis-level behaviour
	// directly because Start requires a *gin.Context with a real *http.Request.
	startTs := time.Now().Unix()
	fields := map[string]interface{}{
		"request_id":   requestID,
		"channel_id":   "0",
		"model_name":   "test-model",
		"start_time":   startTs,
		"request_path": "/v1/chat/completions",
	}
	require.NoError(t, common.RDB.HSet(ctx, inflightKeyPrefix+requestID, fields).Err())
	require.NoError(t, common.RDB.Expire(ctx, inflightKeyPrefix+requestID, time.Duration(inflightTTLSeconds)*time.Second).Err())
	require.NoError(t, common.RDB.ZAdd(ctx, inflightSortedKey, &redis.Z{Score: float64(startTs), Member: requestID}).Err())

	// List should return one entry.
	items, err := List(1, 20)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, requestID, items[0].RequestID)
	assert.Equal(t, "test-model", items[0].ModelName)
	assert.Equal(t, "/v1/chat/completions", items[0].RequestPath)
	assert.Equal(t, startTs, items[0].StartTime)

	// Count should be 1.
	n, err := Count()
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// Finish removes the entry.
	Finish(requestID)
	n, err = Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)

	items, err = List(1, 20)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestListEmptyWhenRedisDisabled(t *testing.T) {
	prev := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = prev })

	items, err := List(1, 20)
	require.NoError(t, err)
	assert.Empty(t, items)

	n, err := Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

func TestUpdateChannel(t *testing.T) {
	useInflightMiniRedis(t)
	ctx := context.Background()
	requestID := "req-upd"

	// Seed an entry.
	require.NoError(t, common.RDB.HSet(ctx, inflightKeyPrefix+requestID,
		"request_id", requestID,
		"channel_id", "0",
		"model_name", "m",
		"start_time", "100",
		"request_path", "/p",
	).Err())
	common.RDB.ZAdd(ctx, inflightSortedKey, &redis.Z{Score: 100, Member: requestID})

	UpdateChannel(requestID, 42)

	val, err := common.RDB.HGet(ctx, inflightKeyPrefix+requestID, "channel_id").Result()
	require.NoError(t, err)
	assert.Equal(t, "42", val)
}

func TestStartNoopWithoutRedis(t *testing.T) {
	prev := common.RedisEnabled
	common.RDB = nil
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = prev })

	// These should be no-ops, not panics.
	Start("", nil, nil)
	Finish("")
	UpdateChannel("", 0)
}
