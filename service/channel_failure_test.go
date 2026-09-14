package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupChannelFailureTest(t *testing.T) {
	t.Helper()
	previousEnabled := common.AutomaticDisableChannelEnabled
	previousRedisEnabled := common.RedisEnabled
	previousSetting := *operation_setting.GetMonitorSetting()
	previousStates := channelFailureStates.values

	common.AutomaticDisableChannelEnabled = true
	common.RedisEnabled = false
	setting := operation_setting.GetMonitorSetting()
	setting.ChannelErrorWindowMinutes = 5
	setting.ChannelErrorThreshold = 20
	setting.ChannelErrorConsecutiveThreshold = 10
	setting.ChannelErrorStatusCodes = operation_setting.DefaultChannelErrorStatusCodes
	setting.ChannelErrorKeywords = operation_setting.DefaultChannelErrorKeywords
	channelFailureStates.values = make(map[int]channelFailureState)

	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = previousEnabled
		common.RedisEnabled = previousRedisEnabled
		*operation_setting.GetMonitorSetting() = previousSetting
		channelFailureStates.values = previousStates
	})
}

func TestIsChannelFailureMatchRequiresStatusCodeAndKeyword(t *testing.T) {
	setupChannelFailureTest(t)

	cases := []struct {
		name       string
		message    string
		statusCode int
		want       bool
	}{
		{
			name:       "upstream account pool exhausted",
			message:    "无可用账号，请稍后重试",
			statusCode: 503,
			want:       true,
		},
		{
			name:       "upstream usage window exhausted",
			message:    "Codex 账号用量窗口已达上限",
			statusCode: 429,
			want:       true,
		},
		{
			name:       "kiro account pool empty",
			message:    "No available accounts",
			statusCode: 503,
			want:       true,
		},
		{
			// A per-key quota rejection shares the pool-failure status code but
			// names the API key. The channel itself is healthy, so disabling it
			// would take a working upstream out of rotation.
			name:       "per-key rate limit is not a channel failure",
			message:    "API key rate limit exceeded: 60 requests per minute",
			statusCode: 429,
			want:       false,
		},
		{
			name:       "per-key token limit is not a channel failure",
			message:    "token limit exceeded",
			statusCode: 429,
			want:       false,
		},
		{
			// The keyword alone must not be enough: an unrelated status code
			// means the request did not fail the way the rule describes.
			name:       "pool keyword with unmatched status code",
			message:    "无可用账号，请稍后重试",
			statusCode: 400,
			want:       false,
		},
		{
			name:       "unrelated error",
			message:    "bad request",
			statusCode: 400,
			want:       false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := types.NewOpenAIError(errors.New(testCase.message), types.ErrorCodeBadResponseStatusCode, testCase.statusCode)
			assert.Equal(t, testCase.want, IsChannelFailureMatch(err))
		})
	}
}

func TestIsChannelFailureMatchUsesUpstreamStatusCodeAfterMapping(t *testing.T) {
	setupChannelFailureTest(t)

	// A channel may remap 429 to a code clients will not retry. The rule
	// describes upstream behavior, so the mapping must not hide the failure.
	err := types.NewOpenAIError(errors.New("Codex 账号用量窗口已达上限"), types.ErrorCodeBadResponseStatusCode, 429)
	ResetStatusCode(err, `{"429": "500"}`)
	require.Equal(t, 500, err.StatusCode)
	require.Equal(t, 429, err.UpstreamStatusCode())
	assert.True(t, IsChannelFailureMatch(err))
}

func TestIsChannelFailureMatchWithSingleSidedConfiguration(t *testing.T) {
	setupChannelFailureTest(t)
	setting := operation_setting.GetMonitorSetting()

	poolError := types.NewOpenAIError(errors.New("无可用账号，请稍后重试"), types.ErrorCodeBadResponseStatusCode, 503)
	keyError := types.NewOpenAIError(errors.New("API key rate limit exceeded: 60 requests per minute"), types.ErrorCodeBadResponseStatusCode, 429)

	// Keywords only: the status code no longer constrains matching.
	setting.ChannelErrorStatusCodes = ""
	assert.True(t, IsChannelFailureMatch(poolError))
	assert.False(t, IsChannelFailureMatch(keyError))

	// Status codes only: every listed code counts, keywords are not consulted.
	setting.ChannelErrorStatusCodes = "429,503"
	setting.ChannelErrorKeywords = ""
	assert.True(t, IsChannelFailureMatch(poolError))
	assert.True(t, IsChannelFailureMatch(keyError))

	// Neither side configured: the rule is inert rather than matching everything.
	setting.ChannelErrorStatusCodes = ""
	assert.False(t, IsChannelFailureMatch(poolError))
	assert.False(t, IsChannelFailureMatch(keyError))
}

func TestRecordChannelFailureTriggersConfiguredThresholds(t *testing.T) {
	setupChannelFailureTest(t)
	setting := operation_setting.GetMonitorSetting()
	setting.ChannelErrorThreshold = 3
	setting.ChannelErrorConsecutiveThreshold = 0

	assert.False(t, RecordChannelFailure(1001))
	assert.False(t, RecordChannelFailure(1001))
	assert.True(t, RecordChannelFailure(1001))
	assert.True(t, RecordChannelFailure(1001), "the threshold remains reached until counters are cleared")

	ClearChannelFailureCounters(1001)
	assert.False(t, RecordChannelFailure(1001))
}

func TestRecordChannelFailureConsecutiveReset(t *testing.T) {
	setupChannelFailureTest(t)
	setting := operation_setting.GetMonitorSetting()
	setting.ChannelErrorThreshold = 0
	setting.ChannelErrorConsecutiveThreshold = 3

	assert.False(t, RecordChannelFailure(1002))
	ResetChannelFailureConsecutive(1002)
	assert.False(t, RecordChannelFailure(1002))
	assert.False(t, RecordChannelFailure(1002))
	assert.True(t, RecordChannelFailure(1002))
}
