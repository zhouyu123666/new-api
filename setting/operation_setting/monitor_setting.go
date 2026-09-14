package operation_setting

import (
	"fmt"
	"os"
	"strconv"

	"github.com/QuantumNous/new-api/setting/config"
)

type MonitorSetting struct {
	AutoTestChannelEnabled           bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes           float64 `json:"auto_test_channel_minutes"`
	ChannelTestMode                  string  `json:"channel_test_mode"`
	ChannelTestConcurrency           int     `json:"channel_test_concurrency"`
	ChannelErrorWindowMinutes        int     `json:"channel_error_window_minutes"`
	ChannelErrorThreshold            int     `json:"channel_error_threshold"`
	ChannelErrorConsecutiveThreshold int     `json:"channel_error_consecutive_threshold"`
	ChannelErrorStatusCodes          string  `json:"channel_error_status_codes"`
	ChannelErrorKeywords             string  `json:"channel_error_keywords"`
}

const (
	ChannelTestModeScheduledAll    = "scheduled_all"
	ChannelTestModeAutoBanOnly     = "auto_ban_only"
	ChannelTestModePassiveRecovery = "passive_recovery"

	ChannelTestConcurrencyOptionKey         = "monitor_setting.channel_test_concurrency"
	DefaultChannelTestConcurrency           = 1
	MaxChannelTestConcurrency               = 32
	DefaultChannelErrorWindowMinutes        = 5
	DefaultChannelErrorThreshold            = 20
	DefaultChannelErrorConsecutiveThreshold = 10
	DefaultChannelErrorStatusCodes          = "429,503"
	// DefaultChannelErrorKeywords lists upstream account-pool exhaustion messages.
	// Each entry is a pool-level failure that no retry on the same channel can
	// recover from, unlike per-key quota rejections which share the same status
	// codes but name the API key in their message.
	DefaultChannelErrorKeywords = "Codex 账号用量窗口已达上限\n无可用账号，请稍后重试\n账号池额度已耗尽\n账号池暂无可用账号\nNo available accounts"
)

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled:           false,
	AutoTestChannelMinutes:           10,
	ChannelTestMode:                  ChannelTestModeScheduledAll,
	ChannelTestConcurrency:           DefaultChannelTestConcurrency,
	ChannelErrorWindowMinutes:        DefaultChannelErrorWindowMinutes,
	ChannelErrorThreshold:            DefaultChannelErrorThreshold,
	ChannelErrorConsecutiveThreshold: DefaultChannelErrorConsecutiveThreshold,
	ChannelErrorStatusCodes:          DefaultChannelErrorStatusCodes,
	ChannelErrorKeywords:             DefaultChannelErrorKeywords,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting() *MonitorSetting {
	if os.Getenv("CHANNEL_TEST_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_TEST_FREQUENCY"))
		if err == nil && frequency > 0 {
			monitorSetting.AutoTestChannelEnabled = true
			monitorSetting.AutoTestChannelMinutes = float64(frequency)
			monitorSetting.ChannelTestMode = ChannelTestModeScheduledAll
		}
	}
	if enabled, ok := os.LookupEnv("CHANNEL_TEST_ENABLED"); ok {
		parsed, err := strconv.ParseBool(enabled)
		if err == nil {
			monitorSetting.AutoTestChannelEnabled = parsed
		}
	}
	switch monitorSetting.ChannelTestMode {
	case ChannelTestModeAutoBanOnly, ChannelTestModePassiveRecovery:
	default:
		monitorSetting.ChannelTestMode = ChannelTestModeScheduledAll
	}
	monitorSetting.ChannelTestConcurrency = NormalizeChannelTestConcurrency(monitorSetting.ChannelTestConcurrency)
	if monitorSetting.ChannelErrorWindowMinutes < 1 {
		monitorSetting.ChannelErrorWindowMinutes = DefaultChannelErrorWindowMinutes
	}
	if monitorSetting.ChannelErrorThreshold < 0 {
		monitorSetting.ChannelErrorThreshold = 0
	}
	if monitorSetting.ChannelErrorConsecutiveThreshold < 0 {
		monitorSetting.ChannelErrorConsecutiveThreshold = 0
	}
	return &monitorSetting
}

func NormalizeChannelTestConcurrency(concurrency int) int {
	if concurrency < 1 {
		return DefaultChannelTestConcurrency
	}
	if concurrency > MaxChannelTestConcurrency {
		return MaxChannelTestConcurrency
	}
	return concurrency
}

func ValidateChannelTestConcurrency(value string) error {
	concurrency, err := strconv.Atoi(value)
	if err != nil || concurrency < 1 || concurrency > MaxChannelTestConcurrency {
		return fmt.Errorf("channel test concurrency must be between 1 and %d", MaxChannelTestConcurrency)
	}
	return nil
}
