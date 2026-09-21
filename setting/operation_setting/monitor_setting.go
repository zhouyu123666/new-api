package operation_setting

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/setting/config"
)

type MonitorSetting struct {
	AutoTestChannelEnabled               bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes               float64 `json:"auto_test_channel_minutes"`
	ChannelTestMode                      string  `json:"channel_test_mode"`
	ChannelTestConcurrency               int     `json:"channel_test_concurrency"`
	ChannelModelCircuitBreakerEnabled    bool    `json:"channel_model_circuit_breaker_enabled"`
	ChannelModelExcludedChannelIDs       string  `json:"channel_model_excluded_channel_ids"`
	ChannelErrorWindowMinutes            int     `json:"channel_error_window_minutes"`
	ChannelErrorThreshold                int     `json:"channel_error_threshold"`
	ChannelErrorConsecutiveThreshold     int     `json:"channel_error_consecutive_threshold"`
	ChannelErrorStatusCodes              string  `json:"channel_error_status_codes"`
	ChannelModelRecoveryEnabled          bool    `json:"channel_model_recovery_enabled"`
	ChannelModelRecoverySuccessThreshold int     `json:"channel_model_recovery_success_threshold"`
	ChannelModelRecoveryDelayMinutes     int     `json:"channel_model_recovery_delay_minutes"`
	ChannelModelRecoveryIntervalMinutes  int     `json:"channel_model_recovery_interval_minutes"`
	ChannelModelRecoveryConcurrency      int     `json:"channel_model_recovery_concurrency"`
	ChannelModelEventRetentionDays       int     `json:"channel_model_event_retention_days"`
	ChannelModelWeComBotEnabled          bool    `json:"channel_model_wecom_bot_enabled"`
	ChannelModelWeComBotURL              string  `json:"channel_model_wecom_bot_url"`
	ChannelModelWeComBotWebhookKey       string  `json:"channel_model_wecom_bot_webhook_key"`
	ChannelModelWeComBotContact          string  `json:"channel_model_wecom_bot_contact"`
}

const (
	ChannelTestModeScheduledAll    = "scheduled_all"
	ChannelTestModeAutoBanOnly     = "auto_ban_only"
	ChannelTestModePassiveRecovery = "passive_recovery"

	ChannelTestConcurrencyOptionKey             = "monitor_setting.channel_test_concurrency"
	DefaultChannelTestConcurrency               = 1
	MaxChannelTestConcurrency                   = 32
	DefaultChannelErrorWindowMinutes            = 5
	DefaultChannelErrorThreshold                = 20
	DefaultChannelErrorConsecutiveThreshold     = 10
	DefaultChannelErrorStatusCodes              = "429,503"
	DefaultChannelModelCircuitBreakerEnabled    = false
	DefaultChannelModelRecoveryEnabled          = false
	DefaultChannelModelRecoverySuccessThreshold = 2
	DefaultChannelModelRecoveryDelayMinutes     = 5
	DefaultChannelModelRecoveryIntervalMinutes  = 1
	DefaultChannelModelRecoveryConcurrency      = 4
	DefaultChannelModelEventRetentionDays       = 30
	DefaultChannelModelWeComBotURL              = "https://alertplus.intsig.net/api/alarm/receive/"
	MaxChannelModelRecoverySuccessThreshold     = 100
	MaxChannelModelRecoveryDelayMinutes         = 43200
	MaxChannelModelRecoveryIntervalMinutes      = 10080
	MaxChannelModelRecoveryConcurrency          = 32
	MaxChannelModelEventRetentionDays           = 3650
	MaxChannelModelExcludedChannelIDs           = 1000
)

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled:               false,
	AutoTestChannelMinutes:               10,
	ChannelTestMode:                      ChannelTestModeScheduledAll,
	ChannelTestConcurrency:               DefaultChannelTestConcurrency,
	ChannelModelCircuitBreakerEnabled:    DefaultChannelModelCircuitBreakerEnabled,
	ChannelModelExcludedChannelIDs:       "",
	ChannelErrorWindowMinutes:            DefaultChannelErrorWindowMinutes,
	ChannelErrorThreshold:                DefaultChannelErrorThreshold,
	ChannelErrorConsecutiveThreshold:     DefaultChannelErrorConsecutiveThreshold,
	ChannelErrorStatusCodes:              DefaultChannelErrorStatusCodes,
	ChannelModelRecoveryEnabled:          DefaultChannelModelRecoveryEnabled,
	ChannelModelRecoverySuccessThreshold: DefaultChannelModelRecoverySuccessThreshold,
	ChannelModelRecoveryDelayMinutes:     DefaultChannelModelRecoveryDelayMinutes,
	ChannelModelRecoveryIntervalMinutes:  DefaultChannelModelRecoveryIntervalMinutes,
	ChannelModelRecoveryConcurrency:      DefaultChannelModelRecoveryConcurrency,
	ChannelModelEventRetentionDays:       DefaultChannelModelEventRetentionDays,
	ChannelModelWeComBotEnabled:          false,
	ChannelModelWeComBotURL:              DefaultChannelModelWeComBotURL,
	ChannelModelWeComBotWebhookKey:       "",
	ChannelModelWeComBotContact:          "",
}

var excludedChannelIDCache = struct {
	sync.RWMutex
	raw string
	ids map[int]struct{}
}{ids: make(map[int]struct{})}

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
	if monitorSetting.ChannelModelRecoverySuccessThreshold < 1 {
		monitorSetting.ChannelModelRecoverySuccessThreshold = DefaultChannelModelRecoverySuccessThreshold
	} else if monitorSetting.ChannelModelRecoverySuccessThreshold > MaxChannelModelRecoverySuccessThreshold {
		monitorSetting.ChannelModelRecoverySuccessThreshold = MaxChannelModelRecoverySuccessThreshold
	}
	if monitorSetting.ChannelModelRecoveryDelayMinutes < 0 {
		monitorSetting.ChannelModelRecoveryDelayMinutes = DefaultChannelModelRecoveryDelayMinutes
	} else if monitorSetting.ChannelModelRecoveryDelayMinutes > MaxChannelModelRecoveryDelayMinutes {
		monitorSetting.ChannelModelRecoveryDelayMinutes = MaxChannelModelRecoveryDelayMinutes
	}
	if monitorSetting.ChannelModelRecoveryIntervalMinutes < 1 {
		monitorSetting.ChannelModelRecoveryIntervalMinutes = DefaultChannelModelRecoveryIntervalMinutes
	} else if monitorSetting.ChannelModelRecoveryIntervalMinutes > MaxChannelModelRecoveryIntervalMinutes {
		monitorSetting.ChannelModelRecoveryIntervalMinutes = MaxChannelModelRecoveryIntervalMinutes
	}
	monitorSetting.ChannelModelRecoveryConcurrency = NormalizeChannelModelRecoveryConcurrency(monitorSetting.ChannelModelRecoveryConcurrency)
	if monitorSetting.ChannelModelEventRetentionDays < 1 {
		monitorSetting.ChannelModelEventRetentionDays = DefaultChannelModelEventRetentionDays
	} else if monitorSetting.ChannelModelEventRetentionDays > MaxChannelModelEventRetentionDays {
		monitorSetting.ChannelModelEventRetentionDays = MaxChannelModelEventRetentionDays
	}
	return &monitorSetting
}

func IsChannelModelCircuitBreakerEnabled() bool {
	return GetMonitorSetting().ChannelModelCircuitBreakerEnabled
}

func ParseChannelModelExcludedChannelIDs(value string) ([]int, string, error) {
	value = strings.NewReplacer("，", ",", "；", ",", ";", ",").Replace(value)
	if strings.TrimSpace(value) == "" {
		return []int{}, "", nil
	}
	seen := make(map[int]struct{})
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		channelID, err := strconv.Atoi(token)
		if err != nil || channelID <= 0 {
			return nil, "", fmt.Errorf("invalid channel ID: %s", token)
		}
		seen[channelID] = struct{}{}
		if len(seen) > MaxChannelModelExcludedChannelIDs {
			return nil, "", fmt.Errorf("excluded channel IDs cannot exceed %d", MaxChannelModelExcludedChannelIDs)
		}
	}
	channelIDs := make([]int, 0, len(seen))
	for channelID := range seen {
		channelIDs = append(channelIDs, channelID)
	}
	sort.Ints(channelIDs)
	normalized := make([]string, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		normalized = append(normalized, strconv.Itoa(channelID))
	}
	return channelIDs, strings.Join(normalized, ","), nil
}

func IsChannelModelCircuitBreakerExcluded(channelID int) bool {
	if channelID <= 0 {
		return false
	}
	raw := GetMonitorSetting().ChannelModelExcludedChannelIDs
	excludedChannelIDCache.RLock()
	if excludedChannelIDCache.raw == raw {
		_, excluded := excludedChannelIDCache.ids[channelID]
		excludedChannelIDCache.RUnlock()
		return excluded
	}
	excludedChannelIDCache.RUnlock()

	channelIDs, _, err := ParseChannelModelExcludedChannelIDs(raw)
	nextIDs := make(map[int]struct{}, len(channelIDs))
	if err == nil {
		for _, id := range channelIDs {
			nextIDs[id] = struct{}{}
		}
	}
	excludedChannelIDCache.Lock()
	excludedChannelIDCache.raw = raw
	excludedChannelIDCache.ids = nextIDs
	_, excluded := nextIDs[channelID]
	excludedChannelIDCache.Unlock()
	return excluded
}

func NormalizeChannelModelRecoveryConcurrency(concurrency int) int {
	if concurrency < 1 {
		return DefaultChannelModelRecoveryConcurrency
	}
	if concurrency > MaxChannelModelRecoveryConcurrency {
		return MaxChannelModelRecoveryConcurrency
	}
	return concurrency
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
