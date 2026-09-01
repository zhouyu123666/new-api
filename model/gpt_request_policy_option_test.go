package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateGPTRequestPolicyOptionValues(t *testing.T) {
	for _, test := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "fast policy", key: "global.gpt_request_policy.fast_policy", value: "invalid"},
		{name: "reasoning policy", key: "global.gpt_request_policy.reasoning_policy", value: "force_high"},
		{name: "retired high reasoning policy", key: "global.gpt_request_policy.reasoning_policy", value: "cap_high"},
		{name: "tags too long", key: "global.gpt_request_policy.tags", value: strings.Repeat("x", 513)},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Error(t, validateOptionValue(test.key, test.value))
		})
	}

	require.NoError(t, validateOptionValue("global.gpt_request_policy.tags", ""))
	require.NoError(t, validateOptionValue("global.gpt_request_policy.fast_policy", "allow"))
	require.NoError(t, validateOptionValue("global.gpt_request_policy.reasoning_policy", "client"))
	require.NoError(t, validateOptionValue("global.gpt_request_policy.reasoning_policy", "cap_xhigh"))
}

func TestNormalizeGPTRequestPolicyOptionValues(t *testing.T) {
	assert.Equal(t, "codex2api, sub2api", normalizeOptionValue(
		"global.gpt_request_policy.tags", "  codex2api, sub2api  "))
	assert.Equal(t, "allow", normalizeOptionValue(
		"global.gpt_request_policy.fast_policy", " ALLOW "))
	assert.Equal(t, "cap_high", normalizeOptionValue(
		"global.gpt_request_policy.reasoning_policy", " CAP_HIGH "))
	assert.Equal(t, "cap_xhigh", normalizeOptionValue(
		"global.gpt_request_policy.reasoning_policy", " CAP_XHIGH "))
}

func TestUpdateOptionPersistsNormalizedGPTRequestPolicyValue(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	previousMap := common.OptionMap
	previousSettings := *model_setting.GetGlobalSettings()
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.OptionMap = previousMap
		*model_setting.GetGlobalSettings() = previousSettings
	})

	require.NoError(t, UpdateOption(
		"global.gpt_request_policy.fast_policy", " ALLOW "))
	assert.Equal(t, "allow", requireOptionValue(
		t, db, "global.gpt_request_policy.fast_policy"))
	common.OptionMapRWMutex.RLock()
	assert.Equal(t, "allow", common.OptionMap["global.gpt_request_policy.fast_policy"])
	common.OptionMapRWMutex.RUnlock()
}
