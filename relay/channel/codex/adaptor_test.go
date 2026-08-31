package codex

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLAlphaSearch(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodex,
			ChannelBaseUrl: "https://chatgpt.com",
		},
		RelayMode: relayconstant.RelayModeAlphaSearch,
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://chatgpt.com/backend-api/codex/alpha/search", url)
}

// The Codex backend rejects these fields, so the adaptor clears them rather
// than forwarding what the client sent.
func TestConvertOpenAIResponsesRequestDropsPenalties(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeCodex},
		RelayMode:   relayconstant.RelayModeResponses,
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:            "gpt-5-codex",
		Input:            json.RawMessage(`"hello"`),
		MaxOutputTokens:  lo.ToPtr(uint(128)),
		Temperature:      lo.ToPtr(1.0),
		FrequencyPenalty: json.RawMessage(`1.5`),
		PresencePenalty:  json.RawMessage(`1.5`),
		ServiceTier:      "fast",
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Nil(t, request.MaxOutputTokens)
	assert.Nil(t, request.Temperature)
	assert.Nil(t, request.FrequencyPenalty)
	assert.Nil(t, request.PresencePenalty)
	assert.Empty(t, request.ServiceTier)
}

func TestConvertOpenAIResponsesRequestForwardsFastWhenGlobalPolicyAllows(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	originalPolicy := settings.GPTRequestFastPolicy
	t.Cleanup(func() {
		settings.GPTRequestFastPolicy = originalPolicy
	})
	settings.GPTRequestFastPolicy = model_setting.GPTRequestFastPolicyAllow

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeCodex,
		},
		RelayMode: relayconstant.RelayModeResponses,
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:       "gpt-5.5",
		Input:       json.RawMessage(`"hello"`),
		ServiceTier: "fast",
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Equal(t, "priority", request.ServiceTier)
}

func TestConvertOpenAIResponsesRequestCapsReasoningAtHigh(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	originalPolicy := settings.GPTRequestReasoningPolicy
	t.Cleanup(func() {
		settings.GPTRequestReasoningPolicy = originalPolicy
	})
	settings.GPTRequestReasoningPolicy = model_setting.GPTRequestReasoningPolicyCapHigh

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType: constant.ChannelTypeCodex,
	}}
	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:     "gpt-5.5",
		Reasoning: &dto.Reasoning{Effort: "max"},
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.NotNil(t, request.Reasoning)
	assert.Equal(t, "high", request.Reasoning.Effort)
}

func TestConvertOpenAIResponsesRequestCapsReasoningAtXHigh(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	originalPolicy := settings.GPTRequestReasoningPolicy
	t.Cleanup(func() {
		settings.GPTRequestReasoningPolicy = originalPolicy
	})
	settings.GPTRequestReasoningPolicy = model_setting.GPTRequestReasoningPolicyCapXHigh

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType: constant.ChannelTypeCodex,
	}}
	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:     "gpt-5.5",
		Reasoning: &dto.Reasoning{Effort: "max"},
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.NotNil(t, request.Reasoning)
	assert.Equal(t, "xhigh", request.Reasoning.Effort)
}
