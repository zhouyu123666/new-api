package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMonitorSetting_ChannelTestEnabledEnvOverridesEnabledConfig(t *testing.T) {
	orig := monitorSetting
	t.Cleanup(func() { monitorSetting = orig })

	t.Setenv("CHANNEL_TEST_ENABLED", "false")
	t.Setenv("CHANNEL_TEST_FREQUENCY", "5")
	monitorSetting = MonitorSetting{
		AutoTestChannelEnabled: true,
		AutoTestChannelMinutes: 20,
	}

	setting := GetMonitorSetting()

	require.NotNil(t, setting)
	assert.False(t, setting.AutoTestChannelEnabled)
	assert.Equal(t, float64(5), setting.AutoTestChannelMinutes)
}

func TestGetMonitorSetting_ChannelTestEnabledEnvCanEnableDisabledConfig(t *testing.T) {
	orig := monitorSetting
	t.Cleanup(func() { monitorSetting = orig })

	t.Setenv("CHANNEL_TEST_ENABLED", "true")
	monitorSetting = MonitorSetting{
		AutoTestChannelEnabled: false,
		AutoTestChannelMinutes: 12,
	}

	setting := GetMonitorSetting()

	require.NotNil(t, setting)
	assert.True(t, setting.AutoTestChannelEnabled)
	assert.Equal(t, float64(12), setting.AutoTestChannelMinutes)
}

func TestGetMonitorSettingPreservesAutoBanOnlyMode(t *testing.T) {
	orig := monitorSetting
	t.Cleanup(func() { monitorSetting = orig })

	t.Setenv("CHANNEL_TEST_ENABLED", "")
	t.Setenv("CHANNEL_TEST_FREQUENCY", "")
	monitorSetting = MonitorSetting{ChannelTestMode: ChannelTestModeAutoBanOnly}

	setting := GetMonitorSetting()

	require.NotNil(t, setting)
	assert.Equal(t, ChannelTestModeAutoBanOnly, setting.ChannelTestMode)
}

func TestGetMonitorSettingNormalizesChannelTestConcurrency(t *testing.T) {
	orig := monitorSetting
	t.Cleanup(func() { monitorSetting = orig })

	tests := []struct {
		name        string
		concurrency int
		want        int
	}{
		{name: "missing uses safe default", concurrency: 0, want: DefaultChannelTestConcurrency},
		{name: "configured value is preserved", concurrency: 8, want: 8},
		{name: "oversized value is capped", concurrency: MaxChannelTestConcurrency + 1, want: MaxChannelTestConcurrency},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			monitorSetting = MonitorSetting{ChannelTestConcurrency: test.concurrency}

			setting := GetMonitorSetting()

			require.NotNil(t, setting)
			assert.Equal(t, test.want, setting.ChannelTestConcurrency)
		})
	}
}

func TestValidateChannelTestConcurrency(t *testing.T) {
	require.NoError(t, ValidateChannelTestConcurrency("1"))
	require.NoError(t, ValidateChannelTestConcurrency("32"))
	assert.Error(t, ValidateChannelTestConcurrency("0"))
	assert.Error(t, ValidateChannelTestConcurrency("33"))
	assert.Error(t, ValidateChannelTestConcurrency("1.5"))
}

func TestChannelModelCircuitBreakerSettingIsReadable(t *testing.T) {
	original := monitorSetting
	t.Cleanup(func() { monitorSetting = original })
	monitorSetting.ChannelModelCircuitBreakerEnabled = true

	assert.True(t, IsChannelModelCircuitBreakerEnabled())
}

func TestChannelModelCircuitBreakerDefaultsAreOptIn(t *testing.T) {
	assert.False(t, DefaultChannelModelCircuitBreakerEnabled)
	assert.False(t, DefaultChannelModelRecoveryEnabled)
}

func TestParseChannelModelExcludedChannelIDsNormalizesAndRejectsInvalidValues(t *testing.T) {
	ids, normalized, err := ParseChannelModelExcludedChannelIDs("10，2;10, 3")
	require.NoError(t, err)
	assert.Equal(t, []int{2, 3, 10}, ids)
	assert.Equal(t, "2,3,10", normalized)

	_, _, err = ParseChannelModelExcludedChannelIDs("1,invalid")
	assert.Error(t, err)
	_, _, err = ParseChannelModelExcludedChannelIDs("0")
	assert.Error(t, err)
}
