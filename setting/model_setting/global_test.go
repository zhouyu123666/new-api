package model_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalSettingsGPTRequestPolicyAppliesWithoutChannelTags(t *testing.T) {
	settings := &GlobalSettings{
		GPTRequestFastPolicy: GPTRequestFastPolicyAllow,
	}

	assert.True(t, settings.AllowsGPTFast())

	settings.GPTRequestReasoningPolicy = GPTRequestReasoningPolicyCapHigh
	assert.True(t, settings.CapsGPTReasoningAtHigh())
	assert.Equal(t, "high", settings.GPTReasoningEffortCap())

	settings.GPTRequestReasoningPolicy = GPTRequestReasoningPolicyCapXHigh
	assert.False(t, settings.CapsGPTReasoningAtHigh())
	assert.Equal(t, "xhigh", settings.GPTReasoningEffortCap())

	settings.GPTRequestReasoningPolicy = GPTRequestReasoningPolicyClient
	assert.Empty(t, settings.GPTReasoningEffortCap())
}

func TestGlobalSettingsGPTRequestPolicyDisabledDoesNotAllowFast(t *testing.T) {
	settings := &GlobalSettings{
		GPTRequestFastPolicy: GPTRequestFastPolicyDisabled,
	}

	assert.False(t, settings.AllowsGPTFast())
}
