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
}

func TestGlobalSettingsGPTRequestPolicyDisabledDoesNotAllowFast(t *testing.T) {
	settings := &GlobalSettings{
		GPTRequestFastPolicy: GPTRequestFastPolicyDisabled,
	}

	assert.False(t, settings.AllowsGPTFast())
}
