package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const channelFailureKeyPrefix = "channel_disable_failure:v1"

type channelFailureState struct {
	failureTimes     []time.Time
	consecutiveCount int
}

var channelFailureStates = struct {
	sync.Mutex
	values map[int]channelFailureState
}{values: make(map[int]channelFailureState)}

// IsChannelFailureMatch reports whether an error belongs to the aggregate
// channel-failure rule. Matching is intentionally independent of the provider
// so it works for Codex2API, Kiro-Go, and other upstream adapters.
//
// Status code and keyword are combined with AND when both are configured. An
// upstream account pool that is exhausted reports both a pool-level status code
// and a pool-level message, whereas a per-key quota rejection reuses the same
// status code with a key-level message. Requiring both keeps a healthy channel
// from being disabled because the gateway API key hit its own rate limit.
// When only one side is configured that side decides alone, so a partially
// filled configuration still takes effect instead of silently never matching.
func IsChannelFailureMatch(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	setting := operation_setting.GetMonitorSetting()
	if !common.AutomaticDisableChannelEnabled || (setting.ChannelErrorThreshold <= 0 && setting.ChannelErrorConsecutiveThreshold <= 0) {
		return false
	}

	statusCodes, parseErr := operation_setting.ParseHTTPStatusCodeRanges(setting.ChannelErrorStatusCodes)
	if parseErr != nil {
		statusCodes = nil
	}
	keywords := channelFailureKeywords(setting.ChannelErrorKeywords)
	if len(statusCodes) == 0 && len(keywords) == 0 {
		return false
	}

	if len(statusCodes) > 0 {
		// Compare against the upstream status code: a channel status-code mapping
		// may already have rewritten StatusCode, and the rule describes upstream
		// behavior rather than what the client eventually sees.
		if !operation_setting.MatchStatusCodeRanges(statusCodes, err.UpstreamStatusCode()) {
			return false
		}
	}
	if len(keywords) == 0 {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	for _, keyword := range keywords {
		if strings.Contains(lowerMessage, keyword) {
			return true
		}
	}
	return false
}

// channelFailureKeywords splits the configured keyword list into lowercase
// entries, dropping blank lines so an empty configuration is detectable.
func channelFailureKeywords(configured string) []string {
	lines := strings.Split(configured, "\n")
	keywords := make([]string, 0, len(lines))
	for _, keyword := range lines {
		if keyword = strings.TrimSpace(strings.ToLower(keyword)); keyword != "" {
			keywords = append(keywords, keyword)
		}
	}
	return keywords
}

// RecordChannelFailure records one matching error for a channel and returns
// true when either configured threshold has been reached.
func RecordChannelFailure(channelID int) bool {
	setting := operation_setting.GetMonitorSetting()
	if channelID <= 0 || !common.AutomaticDisableChannelEnabled ||
		(setting.ChannelErrorThreshold <= 0 && setting.ChannelErrorConsecutiveThreshold <= 0) {
		return false
	}
	window := time.Duration(setting.ChannelErrorWindowMinutes) * time.Minute
	if window <= 0 {
		window = time.Duration(operation_setting.DefaultChannelErrorWindowMinutes) * time.Minute
	}

	if common.RedisEnabled && common.RDB != nil {
		reached, err := recordRedisChannelFailure(channelID, window, setting.ChannelErrorThreshold, setting.ChannelErrorConsecutiveThreshold)
		if err == nil {
			return reached
		}
		common.SysLog(fmt.Sprintf("failed to record channel failure in Redis: channel_id=%d, error=%v", channelID, err))
	}

	return recordMemoryChannelFailure(channelID, window, setting.ChannelErrorThreshold, setting.ChannelErrorConsecutiveThreshold)
}

// ResetChannelFailureConsecutive breaks the consecutive-failure sequence while
// retaining the rolling-window count.
func ResetChannelFailureConsecutive(channelID int) {
	if channelID <= 0 {
		return
	}
	if common.RedisEnabled && common.RDB != nil {
		if err := common.RedisDel(redisChannelFailureKey(channelID, "consecutive")); err != nil {
			common.SysLog(fmt.Sprintf("failed to reset channel failure sequence in Redis: channel_id=%d, error=%v", channelID, err))
		}
	}

	channelFailureStates.Lock()
	if state, ok := channelFailureStates.values[channelID]; ok {
		state.consecutiveCount = 0
		channelFailureStates.values[channelID] = state
	}
	channelFailureStates.Unlock()
}

// ClearChannelFailureCounters clears both counters after a successful health
// probe or after a channel has been disabled, preventing stale failures from
// affecting a later recovery cycle.
func ClearChannelFailureCounters(channelID int) {
	if channelID <= 0 {
		return
	}
	channelFailureStates.Lock()
	delete(channelFailureStates.values, channelID)
	channelFailureStates.Unlock()

	if common.RedisEnabled && common.RDB != nil {
		for _, kind := range []string{"window", "consecutive"} {
			if err := common.RedisDel(redisChannelFailureKey(channelID, kind)); err != nil {
				common.SysLog(fmt.Sprintf("failed to clear channel failure counter in Redis: channel_id=%d, kind=%s, error=%v", channelID, kind, err))
			}
		}
	}
}

func redisChannelFailureKey(channelID int, kind string) string {
	return fmt.Sprintf("%s:%d:%s", channelFailureKeyPrefix, channelID, kind)
}

func recordRedisChannelFailure(channelID int, window time.Duration, windowThreshold int, consecutiveThreshold int) (bool, error) {
	now := time.Now()
	windowMilliseconds := window.Milliseconds()
	windowCount, err := common.RDB.Eval(
		context.Background(),
		`redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[3])
redis.call('PEXPIRE', KEYS[1], ARGV[4])
return redis.call('ZCARD', KEYS[1])`,
		[]string{redisChannelFailureKey(channelID, "window")},
		now.Add(-window).UnixMilli(),
		now.UnixMilli(),
		fmt.Sprintf("%d-%s", now.UnixNano(), common.GetRandomString(8)),
		windowMilliseconds,
	).Int64()
	if err != nil {
		return false, err
	}
	consecutiveCount, err := common.RDB.Eval(
		context.Background(),
		`local count = redis.call('INCR', KEYS[1])
redis.call('PEXPIRE', KEYS[1], ARGV[1])
return count`,
		[]string{redisChannelFailureKey(channelID, "consecutive")},
		windowMilliseconds,
	).Int64()
	if err != nil {
		return false, err
	}
	return (windowThreshold > 0 && windowCount >= int64(windowThreshold)) ||
		(consecutiveThreshold > 0 && consecutiveCount >= int64(consecutiveThreshold)), nil
}

func recordMemoryChannelFailure(channelID int, window time.Duration, windowThreshold int, consecutiveThreshold int) bool {
	now := time.Now()
	channelFailureStates.Lock()
	defer channelFailureStates.Unlock()

	state := channelFailureStates.values[channelID]
	cutoff := now.Add(-window)
	firstRetained := 0
	for firstRetained < len(state.failureTimes) && !state.failureTimes[firstRetained].After(cutoff) {
		firstRetained++
	}
	if firstRetained > 0 {
		state.failureTimes = append([]time.Time(nil), state.failureTimes[firstRetained:]...)
	}
	state.failureTimes = append(state.failureTimes, now)
	state.consecutiveCount++
	reached := (windowThreshold > 0 && len(state.failureTimes) >= windowThreshold) ||
		(consecutiveThreshold > 0 && state.consecutiveCount >= consecutiveThreshold)
	channelFailureStates.values[channelID] = state
	return reached
}
