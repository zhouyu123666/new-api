package common

import (
	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// IsGPTRequestPolicyChannel reports whether the global GPT request policy
// applies to this channel. Both direct OpenAI API channels and ChatGPT
// Subscription (Codex) channels expose the GPT service tier and reasoning
// controls used by this policy.
func IsGPTRequestPolicyChannel(info *RelayInfo) bool {
	return info != nil && info.ChannelMeta != nil &&
		(info.ChannelType == constant.ChannelTypeOpenAI || info.ChannelType == constant.ChannelTypeCodex)
}

// ShouldForwardGPTServiceTier reports whether the global policy allows
// forwarding the client's service_tier field to an OpenAI GPT backend.
func ShouldForwardGPTServiceTier(info *RelayInfo) bool {
	if !IsGPTRequestPolicyChannel(info) {
		return false
	}
	return model_setting.GetGlobalSettings().AllowsGPTFast()
}

// ApplyGPTReasoningPolicy applies the configured reasoning effort cap. Missing,
// unknown, and lower effort values remain client-controlled.
func ApplyGPTReasoningPolicy(info *RelayInfo, reasoning *dto.Reasoning) {
	if !IsGPTRequestPolicyChannel(info) || reasoning == nil {
		return
	}
	cap := model_setting.GetGlobalSettings().GPTReasoningEffortCap()
	if cap == "" {
		return
	}

	if capped := CapGPTReasoningEffortValue(reasoning.Effort, cap); capped != reasoning.Effort {
		reasoning.Effort = capped
		info.SetReasoningEffort(capped)
	}
}

// ApplyGPTChatRequestPolicy applies the same global policy to the Chat
// Completions request shape used by direct OpenAI channels.
func ApplyGPTChatRequestPolicy(info *RelayInfo, request *dto.GeneralOpenAIRequest) {
	if !IsGPTRequestPolicyChannel(info) || request == nil {
		return
	}

	if !ShouldForwardGPTServiceTier(info) {
		request.ServiceTier = nil
	} else if len(request.ServiceTier) > 0 {
		value := gjson.ParseBytes(request.ServiceTier)
		if value.Type == gjson.String {
			request.ServiceTier, _ = rootcommon.Marshal(NormalizeGPTServiceTierValue(value.String()))
		}
	}

	cap := model_setting.GetGlobalSettings().GPTReasoningEffortCap()
	if cap == "" {
		return
	}
	if capped := CapGPTReasoningEffortValue(request.ReasoningEffort, cap); capped != request.ReasoningEffort {
		request.ReasoningEffort = capped
		info.SetReasoningEffort(capped)
	}
	if len(request.Reasoning) > 0 {
		if capped, err := CapGPTReasoningEffort(request.Reasoning, cap); err == nil {
			request.Reasoning = capped
		}
	}
}

// ApplyGPTRequestPolicyJSON applies the policy to a pass-through request body.
// It covers both Responses reasoning.effort and Chat Completions
// reasoning_effort shapes.
func ApplyGPTRequestPolicyJSON(info *RelayInfo, jsonData []byte) ([]byte, error) {
	if !IsGPTRequestPolicyChannel(info) {
		return jsonData, nil
	}

	var err error
	if !ShouldForwardGPTServiceTier(info) {
		jsonData, err = RemoveGPTServiceTier(jsonData)
	} else {
		jsonData, err = NormalizeGPTServiceTier(jsonData)
	}
	if err != nil {
		return nil, err
	}
	cap := model_setting.GetGlobalSettings().GPTReasoningEffortCap()
	if cap == "" {
		return jsonData, nil
	}
	jsonData, err = CapGPTReasoningEffort(jsonData, cap)
	if err != nil {
		return nil, err
	}
	if value := gjson.GetBytes(jsonData, "reasoning_effort"); value.Exists() && value.Type == gjson.String {
		capped := CapGPTReasoningEffortValue(value.String(), cap)
		if capped != value.String() {
			jsonData, err = sjson.SetBytes(jsonData, "reasoning_effort", capped)
		}
	}
	return jsonData, err
}

// ApplyGPTReasoningEffortCap keeps RelayInfo aligned with a rewritten request
// body so billing and usage logs reflect the effort sent upstream.
func ApplyGPTReasoningEffortCap(info *RelayInfo) {
	if !IsGPTRequestPolicyChannel(info) {
		return
	}
	cap := model_setting.GetGlobalSettings().GPTReasoningEffortCap()
	if cap == "" {
		return
	}
	if capped := CapGPTReasoningEffortValue(info.GetReasoningEffort(), cap); capped != info.GetReasoningEffort() {
		info.SetReasoningEffort(capped)
	}
}

// Compatibility wrappers for the former Codex-specific helper names.
func ShouldForwardCodexServiceTier(info *RelayInfo) bool {
	return IsGPTRequestPolicyChannel(info) && info.ChannelType == constant.ChannelTypeCodex && ShouldForwardGPTServiceTier(info)
}

func ApplyCodexReasoningPolicy(info *RelayInfo, reasoning *dto.Reasoning) {
	if info == nil || info.ChannelType != constant.ChannelTypeCodex {
		return
	}
	ApplyGPTReasoningPolicy(info, reasoning)
}
