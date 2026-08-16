package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/go-redis/redis/v8"
)

// InflightInfo is the data stored for each ongoing request.
type InflightInfo struct {
	RequestID   string `json:"request_id"`
	ChannelID   int    `json:"channel_id"`
	ModelName   string `json:"model_name"`
	StartTime   int64  `json:"start_time"`
	RequestPath string `json:"request_path"`
}

const (
	inflightKeyPrefix  = "inflight:"        // per-request hash key
	inflightSortedKey  = "inflight:sorted"  // ZSET ordered by start timestamp
	inflightTTLSeconds = 600                // 10-minute safety window
)

// inflightEnabled reports whether Redis is available for tracking.
func inflightEnabled() bool {
	return common.RedisEnabled && common.RDB != nil
}

// Start records a new in-flight request.
// Idempotent: if the request is already recorded it is overwritten.
func Start(requestID string, c *gin.Context, info *relaycommon.RelayInfo) {
	if requestID == "" || !inflightEnabled() {
		return
	}
	startTs := time.Now().Unix()
	fields := map[string]interface{}{
		"request_id":   requestID,
		"channel_id":   strconv.Itoa(info.GetChannelID()),
		"model_name":   info.GetOriginModelName(),
		"start_time":   strconv.FormatInt(startTs, 10),
		"request_path": c.Request.URL.Path,
	}
	ctx := context.Background()
	hashKey := inflightKeyPrefix + requestID
	ttl := time.Duration(inflightTTLSeconds) * time.Second

	pipe := common.RDB.TxPipeline()
	pipe.HSet(ctx, hashKey, fields)
	pipe.Expire(ctx, hashKey, ttl)
	pipe.ZAdd(ctx, inflightSortedKey, &redis.Z{Score: float64(startTs), Member: requestID})
	pipe.Expire(ctx, inflightSortedKey, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		// Tracking is best-effort; never block the request on failure.
		common.SysError("inflight tracker: start failed: " + err.Error())
	}
}

// Finish removes the in-flight entry.
// Errors are ignored intentionally — cleanup must not block the response.
func Finish(requestID string) {
	if requestID == "" || !inflightEnabled() {
		return
	}
	ctx := context.Background()
	common.RDB.Del(ctx, inflightKeyPrefix+requestID)
	common.RDB.ZRem(ctx, inflightSortedKey, requestID)
}

// UpdateChannel updates the channel for an existing in-flight entry.
// Called when a channel is selected (or re-selected on retry).
func UpdateChannel(requestID string, channelID int) {
	if requestID == "" || !inflightEnabled() {
		return
	}
	ctx := context.Background()
	common.RDB.HSet(ctx, inflightKeyPrefix+requestID, "channel_id", strconv.Itoa(channelID))
}

// List returns a page of in-flight entries ordered newest-first.
func List(page, size int) ([]InflightInfo, error) {
	if !inflightEnabled() {
		return []InflightInfo{}, nil
	}
	if size <= 0 {
		size = 20
	}
	if page < 1 {
		page = 1
	}
	start := int64((page - 1) * size)
	stop := start + int64(size) - 1
	ctx := context.Background()

	ids, err := common.RDB.ZRevRange(ctx, inflightSortedKey, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("inflight tracker: ZRevRange: %w", err)
	}
	infos := make([]InflightInfo, 0, len(ids))
	for _, id := range ids {
		m, err := common.RDB.HGetAll(ctx, inflightKeyPrefix+id).Result()
		if err != nil || len(m) == 0 {
			continue // skip stale / corrupted entry
		}
		chanID, _ := strconv.Atoi(m["channel_id"])
		startTs, _ := strconv.ParseInt(m["start_time"], 10, 64)
		infos = append(infos, InflightInfo{
			RequestID:   m["request_id"],
			ChannelID:   chanID,
			ModelName:   m["model_name"],
			StartTime:   startTs,
			RequestPath: m["request_path"],
		})
	}
	return infos, nil
}

// Count returns the total number of in-flight entries.
func Count() (int64, error) {
	if !inflightEnabled() {
		return 0, nil
	}
	ctx := context.Background()
	n, err := common.RDB.ZCard(ctx, inflightSortedKey).Result()
	if err != nil {
		return 0, fmt.Errorf("inflight tracker: ZCard: %w", err)
	}
	return n, nil
}
