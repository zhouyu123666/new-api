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
	setting.ChannelModelCircuitBreakerEnabled = true
	channelFailureStates.values = make(map[channelModelFailureKey]channelFailureState)

	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = previousEnabled
		common.RedisEnabled = previousRedisEnabled
		*operation_setting.GetMonitorSetting() = previousSetting
		channelFailureStates.values = previousStates
	})
}

func TestIsChannelModelFailureMatchUsesOnlyConfiguredUpstreamStatusCode(t *testing.T) {
	setupChannelFailureTest(t)

	cases := []struct {
		name       string
		message    string
		statusCode int
		want       bool
	}{
		{name: "rate limit", message: "API key rate limit exceeded", statusCode: 429, want: true},
		{name: "account pool unavailable", message: "No available accounts", statusCode: 503, want: true},
		{name: "unconfigured status", message: "bad request", statusCode: 400, want: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := types.NewOpenAIError(errors.New(testCase.message), types.ErrorCodeBadResponseStatusCode, testCase.statusCode)
			assert.Equal(t, testCase.want, IsChannelModelFailureMatch(err))
		})
	}
}

func TestIsChannelModelFailureMatchUsesUpstreamStatusCodeAfterMapping(t *testing.T) {
	setupChannelFailureTest(t)

	err := types.NewOpenAIError(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, 429)
	ResetStatusCode(err, `{"429": "500"}`)
	require.Equal(t, 500, err.StatusCode)
	require.Equal(t, 429, err.UpstreamStatusCode())
	assert.True(t, IsChannelModelFailureMatch(err))
}

func TestRecordChannelModelFailureKeepsModelsIndependent(t *testing.T) {
	setupChannelFailureTest(t)
	setting := operation_setting.GetMonitorSetting()
	setting.ChannelErrorThreshold = 2
	setting.ChannelErrorConsecutiveThreshold = 0

	assert.False(t, RecordChannelModelFailure(1001, "model-a"))
	assert.False(t, RecordChannelModelFailure(1001, "model-b"))
	assert.True(t, RecordChannelModelFailure(1001, "model-a"))
	assert.False(t, RecordChannelModelFailure(1002, "model-a"))

	ClearChannelModelFailureCounters(1001, "model-a")
	assert.False(t, RecordChannelModelFailure(1001, "model-a"))
}

func TestResetChannelModelFailureConsecutiveOnlyResetsRequestedPair(t *testing.T) {
	setupChannelFailureTest(t)
	setting := operation_setting.GetMonitorSetting()
	setting.ChannelErrorThreshold = 0
	setting.ChannelErrorConsecutiveThreshold = 3

	assert.False(t, RecordChannelModelFailure(1002, "model-a"))
	assert.False(t, RecordChannelModelFailure(1002, "model-b"))
	ResetChannelModelFailureConsecutive(1002, "model-a")
	assert.False(t, RecordChannelModelFailure(1002, "model-a"))
	assert.False(t, RecordChannelModelFailure(1002, "model-a"))
	assert.False(t, RecordChannelModelFailure(1002, "model-b"))
	assert.True(t, RecordChannelModelFailure(1002, "model-b"))
}

func TestRecordChannelModelFailureCapsConsecutiveCounter(t *testing.T) {
	setupChannelFailureTest(t)
	setting := operation_setting.GetMonitorSetting()
	setting.ChannelErrorThreshold = 0
	setting.ChannelErrorConsecutiveThreshold = 2

	assert.False(t, RecordChannelModelFailure(1003, "model-a"))
	assert.True(t, RecordChannelModelFailure(1003, "model-a"))
	assert.True(t, RecordChannelModelFailure(1003, "model-a"))

	state := channelFailureStates.values[channelModelFailureKey{channelID: 1003, modelName: "model-a"}]
	assert.Equal(t, 2, state.consecutiveCount)
}

func TestRecordChannelModelFailureCapsWindowEntries(t *testing.T) {
	setupChannelFailureTest(t)
	setting := operation_setting.GetMonitorSetting()
	setting.ChannelErrorThreshold = 2
	setting.ChannelErrorConsecutiveThreshold = 0

	assert.False(t, RecordChannelModelFailure(1004, "model-a"))
	assert.True(t, RecordChannelModelFailure(1004, "model-a"))
	assert.True(t, RecordChannelModelFailure(1004, "model-a"))

	state := channelFailureStates.values[channelModelFailureKey{channelID: 1004, modelName: "model-a"}]
	assert.Len(t, state.failureTimes, 2)
}

func TestDisabledChannelModelCircuitBreakerDoesNotMatchOrRecord(t *testing.T) {
	setupChannelFailureTest(t)
	setting := operation_setting.GetMonitorSetting()
	setting.ChannelModelCircuitBreakerEnabled = false
	err := types.NewOpenAIError(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, 429)

	assert.False(t, IsChannelModelFailureMatch(err))
	assert.False(t, RecordChannelModelFailure(1005, "model-a"))
	assert.Empty(t, channelFailureStates.values)
}

func TestClearAllChannelModelFailureCountersClearsEveryMemoryPair(t *testing.T) {
	setupChannelFailureTest(t)
	require.False(t, RecordChannelModelFailure(1006, "model-a"))
	require.False(t, RecordChannelModelFailure(1006, "model-b"))
	require.NotEmpty(t, channelFailureStates.values)

	require.NoError(t, ClearAllChannelModelFailureCounters())
	assert.Empty(t, channelFailureStates.values)
}

func TestExcludedChannelDoesNotAccumulateFailures(t *testing.T) {
	setupChannelFailureTest(t)
	operation_setting.GetMonitorSetting().ChannelModelExcludedChannelIDs = "1007"

	assert.False(t, RecordChannelModelFailure(1007, "model-a"))
	assert.Empty(t, channelFailureStates.values)
	assert.False(t, RecordChannelModelFailure(1008, "model-a"))
	assert.NotEmpty(t, channelFailureStates.values)
}
