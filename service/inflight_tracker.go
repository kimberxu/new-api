package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// InflightInfo is the data stored for each ongoing request.
type InflightInfo struct {
	RequestID         string `json:"request_id"`
	ChannelID         int    `json:"channel_id"`
	ChannelName       string `json:"channel_name,omitempty"`
	ModelName         string `json:"model_name"`
	UpstreamModelName string `json:"upstream_model,omitempty"`
	StartTime         int64  `json:"start_time"`
	EndTime           int64  `json:"end_time,omitempty"`
	Finished          bool   `json:"finished,omitempty"`
	RequestPath       string `json:"request_path"`
	ClientIP          string `json:"client_ip,omitempty"`
	KeyName           string `json:"key_name,omitempty"`
}

const (
	inflightKeyPrefix  = "inflight:"       // per-request hash key
	inflightSortedKey  = "inflight:sorted" // ZSET ordered by start timestamp
	inflightTTLSeconds = 600               // 10-minute safety window
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
	// token_name is the API key's display name (set by middleware/auth.go);
	// TokenKey is the raw key itself and must not be exposed.
	tokenName := c.GetString("token_name")
	fields := map[string]interface{}{
		"request_id":   requestID,
		"channel_id":   strconv.Itoa(info.GetChannelID()),
		"model_name":   info.GetOriginModelName(),
		"start_time":   strconv.FormatInt(startTs, 10),
		"request_path": c.Request.URL.Path,
		"client_ip":    c.ClientIP(),
		"key_name":     tokenName,
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
	// Mark as finished, retain for UI visibility.
	now := time.Now().Unix()
	// Update hash fields.
	common.RDB.HSet(ctx, inflightKeyPrefix+requestID, map[string]interface{}{
		"finished": "1",
		"end_time": strconv.FormatInt(now, 10),
	})
	// Keep sorted set entry for ordering; optionally could keep or remove.
	// No deletion to preserve entry for UI.
}

// UpdateChannel updates the channel for an existing in-flight entry.
// Called when a channel is selected (or re-selected on retry).
func UpdateChannel(requestID string, channelID int) {
	if requestID == "" || !inflightEnabled() {
		return
	}
	fields := map[string]interface{}{
		"channel_id": strconv.Itoa(channelID),
	}
	if ch, err := model.CacheGetChannel(channelID); err == nil && ch != nil {
		fields["channel_name"] = ch.Name
	}
	ctx := context.Background()
	common.RDB.HSet(ctx, inflightKeyPrefix+requestID, fields)
}

// UpdateUpstreamModel updates the upstream (model-mapped) model name once the
// relay handler has resolved it. Called after the handler runs, before the
// entry is finished, so the UI can show both the downstream and upstream model.
func UpdateUpstreamModel(requestID string, upstreamModel string) {
	if requestID == "" || !inflightEnabled() {
		return
	}
	ctx := context.Background()
	common.RDB.HSet(ctx, inflightKeyPrefix+requestID, "upstream_model", upstreamModel)
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
			RequestID:         m["request_id"],
			ChannelID:         chanID,
			ChannelName:       m["channel_name"],
			ModelName:         m["model_name"],
			UpstreamModelName: m["upstream_model"],
			StartTime:         startTs,
			EndTime:           func() int64 { et, _ := strconv.ParseInt(m["end_time"], 10, 64); return et }(),
			Finished:          m["finished"] == "1",
			RequestPath:       m["request_path"],
			ClientIP:          m["client_ip"],
			KeyName:           m["key_name"],
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
