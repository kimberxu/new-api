package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type ChannelRateLimitSetting struct {
	Enabled    bool    `json:"enabled"`
	DefaultRPM float64 `json:"default_rpm"`
}

var channelRateLimitSetting = ChannelRateLimitSetting{
	Enabled:    false,
	DefaultRPM: 60,
}

func init() {
	config.GlobalConfig.Register("channel_rate_limit_setting", &channelRateLimitSetting)
}

func GetChannelRateLimitSetting() *ChannelRateLimitSetting {
	return &channelRateLimitSetting
}