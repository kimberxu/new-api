package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// modelLevelKeywords are error message fragments that identify a model-level
// failure on 400/422 responses. 404 responses are judged more loosely: any
// message mentioning "model" counts (see IsModelLevelError).
var modelLevelKeywords = []string{
	"model not found", "model_not_found", "model does not exist", "model not exist",
	"no such model", "unknown model", "invalid model", "unsupported model",
	"missing model", "model is not supported", "model not supported",
	"模型不存在", "未知模型", "不支持的模型", "模型不可用", "模型已下线",
}

// IsModelLevelError reports whether the error indicates a model-level problem
// (a specific model is unavailable) as opposed to a channel-level failure.
//
// Loose band (user-confirmed): 404 whose message mentions "model" counts as
// model-level; 400/422 require a keyword match. Other status codes never count.
func IsModelLevelError(err *types.NewAPIError) bool {
	if err == nil || err.StatusCode < 100 || err.StatusCode > 599 {
		return false
	}
	msg := strings.ToLower(err.Error())
	if err.StatusCode == http.StatusNotFound {
		return strings.Contains(msg, "model")
	}
	if err.StatusCode == http.StatusBadRequest || err.StatusCode == http.StatusUnprocessableEntity {
		for _, kw := range modelLevelKeywords {
			if strings.Contains(msg, kw) {
				return true
			}
		}
	}
	return false
}

// [personal] channelLevelBalanceKeywords are error message fragments that
// identify a channel-level upstream account/quota problem (key invalid, out
// of balance). Such errors disable the whole channel, not a single model.
var channelLevelBalanceKeywords = []string{
	"insufficient_balance", "insufficient balance", "payment required",
	"quota exhausted", "insufficient quota", "余额不足", "配额不足", "已欠费",
	"账户余额", "balance is insufficient",
}

var channelLevelAuthKeywords = []string{
	"invalid api key", "authentication", "unauthorized", "permission",
	"key 无效", "密钥无效",
}

// [personal] IsChannelLevelError reports whether the error indicates a
// channel-level problem (upstream key/balance), which disables the whole
// channel. It is checked BEFORE IsModelLevelError in processChannelError so
// balance-deficit messages mentioning "model" are never swallowed by the
// model-level branch. Semantics:
//   - 402 always channel-level;
//   - 403 with auth or balance keywords channel-level;
//   - any other status code with a balance keyword channel-level.
func IsChannelLevelError(err *types.NewAPIError) bool {
	if err == nil || err.StatusCode < 100 || err.StatusCode > 599 {
		return false
	}
	msg := strings.ToLower(err.Error())
	if err.StatusCode == http.StatusPaymentRequired {
		return true
	}
	hasBalance := false
	for _, kw := range channelLevelBalanceKeywords {
		if strings.Contains(msg, kw) {
			hasBalance = true
			break
		}
	}
	if hasBalance {
		return true
	}
	if err.StatusCode == http.StatusForbidden {
		for _, kw := range channelLevelAuthKeywords {
			if strings.Contains(msg, kw) {
				return true
			}
		}
	}
	return false
}

const channelModelDisableWindowRedisNamespace = "channelModelDisableWindow"

var (
	channelModelDisableWindowMemoryLimiterOnce sync.Once
	channelModelDisableWindowMemoryLimiter     *common.InMemoryRateLimiter
)

func getChannelModelDisableWindowMemoryLimiter() *common.InMemoryRateLimiter {
	channelModelDisableWindowMemoryLimiterOnce.Do(func() {
		l := &common.InMemoryRateLimiter{}
		l.Init(10 * time.Minute)
		channelModelDisableWindowMemoryLimiter = l
	})
	return channelModelDisableWindowMemoryLimiter
}

func channelModelDisableWindowRedisKey(channelID int, modelName string, statusCode int, tier string) string {
	return fmt.Sprintf("%s:%d:%s:%d:%s", channelModelDisableWindowRedisNamespace, channelID, strings.ToLower(modelName), statusCode, tier)
}

// channelModelDisableWindowLuaScript atomically pushes a timestamp, trims to
// the threshold, sets expiry, and returns the current count.
const channelModelDisableWindowLuaScript = `
local count = redis.call('LPUSH', KEYS[1], ARGV[1])
if count > tonumber(ARGV[2]) then
  redis.call('LTRIM', KEYS[1], 0, tonumber(ARGV[2]) - 1)
  count = tonumber(ARGV[2])
end
redis.call('EXPIRE', KEYS[1], ARGV[3])
return count
`

var channelModelDisableWindowLuaSha string

func getChannelModelDisableWindowLuaSha() string {
	if channelModelDisableWindowLuaSha != "" {
		return channelModelDisableWindowLuaSha
	}
	ctx := context.Background()
	sha, err := common.RDB.ScriptLoad(ctx, channelModelDisableWindowLuaScript).Result()
	if err != nil {
		return ""
	}
	channelModelDisableWindowLuaSha = sha
	return sha
}

func channelModelDisableWindowRedisTake(channelID int, modelName string, statusCode int, tier string, threshold int, windowSec int64) bool {
	ctx := context.Background()
	key := channelModelDisableWindowRedisKey(channelID, modelName, statusCode, tier)
	now := time.Now().Unix()

	var count int64
	var err error
	sha := getChannelModelDisableWindowLuaSha()
	if sha != "" {
		count, err = common.RDB.EvalSha(ctx, sha, []string{key}, now, threshold, windowSec).Int64()
	} else {
		count, err = common.RDB.Eval(ctx, channelModelDisableWindowLuaScript, []string{key}, now, threshold, windowSec).Int64()
	}
	if err != nil {
		// Allow on error — same fail-open policy as the channel-level window.
		return false
	}
	return count >= int64(threshold)
}

// CheckAndRecordDisableModel records one model-level error and returns true if
// the sliding-window threshold has been reached (i.e. the model on this
// channel should be disabled). The identity key is
// channelID:modelName:statusCode:tier, so different models, status codes and
// tiers are counted independently.
//
// Model-level errors are always counted on the strict (configured) tier, since
// they are explicit errors.
func CheckAndRecordDisableModel(channelID int, modelName string, statusCode int, isConfiguredError bool) bool {
	var threshold int
	var windowSec int64
	var tier string

	if isConfiguredError {
		threshold = common.ConfiguredDisableThreshold
		windowSec = common.ConfiguredDisableWindowSeconds
		tier = "configured"
	} else {
		threshold = common.UnconfiguredDisableThreshold
		windowSec = common.UnconfiguredDisableWindowSeconds
		tier = "unconfigured"
	}

	if threshold <= 0 {
		// Threshold of 0 means "never disable" — safety guard.
		return false
	}

	if common.RedisEnabled && common.RDB != nil {
		return channelModelDisableWindowRedisTake(channelID, modelName, statusCode, tier, threshold, windowSec)
	}

	// In-memory sliding window: allow threshold-1 within the window and
	// trigger on the threshold-th error. threshold=1 triggers immediately.
	if threshold == 1 {
		return true
	}
	key := channelModelDisableWindowRedisKey(channelID, modelName, statusCode, tier)
	return !getChannelModelDisableWindowMemoryLimiter().Request(key, threshold-1, windowSec)
}

// [personal] modelBanAutoDuration is how long an auto model-level ban lasts
// before the periodic recovery probe re-tests the model.
const modelBanAutoDuration = 30 * time.Minute

// DisableChannelModel disables a single model on a channel (source=auto) and
// rebuilds the channel cache so routing excludes the pair immediately. Auto
// bans expire after modelBanAutoDuration (BannedUntil); manual bans are
// permanent (BannedUntil=0).
func DisableChannelModel(channelID int, modelName string, reason string) error {
	common.SysLog(fmt.Sprintf("通道 #%d 模型 %s 发生模型级错误，准备禁用该模型，原因：%s", channelID, modelName, common.LocalLogPreview(reason)))

	now := time.Now()
	bannedUntil := now.Add(modelBanAutoDuration).Unix()
	if err := model.AddChannelDisabledModels(channelID, []string{modelName}, "auto", reason); err != nil {
		common.SysLog(fmt.Sprintf("failed to add channel disabled model: channel_id=%d, model=%s, error=%v", channelID, modelName, err))
		return err
	}
	// Set the ban deadline on the record (AutoMigrate already added the
	// column; existing records keep BannedUntil=0 = permanent until now).
	if err := model.SetChannelDisabledModelBannedUntil(channelID, modelName, bannedUntil); err != nil {
		common.SysLog(fmt.Sprintf("failed to set banned_until: channel_id=%d, model=%s, error=%v", channelID, modelName, err))
		return err
	}
	model.InitChannelCache()

	channel, err := model.GetChannelById(channelID, false)
	if err == nil && channel != nil {
		subject := fmt.Sprintf("通道「%s」（#%d）模型 %s 已被禁用", channel.Name, channelID, modelName)
		content := fmt.Sprintf("通道「%s」（#%d）模型 %s 已被禁用，原因：%s", channel.Name, channelID, modelName, reason)
		NotifyRootUser(formatNotifyType(channelID, common.ChannelStatusAutoDisabled), subject, content)
	}
	return nil
}

// EnableChannelModel re-enables a single model on a channel. An empty source
// clears any source; "auto" only clears auto-sourced disables.
func EnableChannelModel(channelID int, modelName string, source string) error {
	if err := model.EnableChannelModelDisabled(channelID, modelName, source); err != nil {
		common.SysLog(fmt.Sprintf("failed to enable channel disabled model: channel_id=%d, model=%s, error=%v", channelID, modelName, err))
		return err
	}
	model.InitChannelCache()
	return nil
}

// ExtendChannelModelBan renews the ban deadline of an auto model-level
// disable record (used when the recovery probe still fails). Does not touch
// the sliding-window counters: the recovery probe is a single targeted
// request, not a repeat error storm.
func ExtendChannelModelBan(channelID int, modelName string) error {
	if err := model.SetChannelDisabledModelBannedUntil(channelID, modelName, time.Now().Add(modelBanAutoDuration).Unix()); err != nil {
		common.SysLog(fmt.Sprintf("failed to extend channel disabled model ban: channel_id=%d, model=%s, error=%v", channelID, modelName, err))
		return err
	}
	return nil
}
