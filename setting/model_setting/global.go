package model_setting

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type ChatCompletionsToResponsesPolicy struct {
	Enabled       bool     `json:"enabled"`
	AllChannels   bool     `json:"all_channels"`
	ChannelIDs    []int    `json:"channel_ids,omitempty"`
	ChannelTypes  []int    `json:"channel_types,omitempty"`
	ModelPatterns []string `json:"model_patterns,omitempty"`
}

// GPT request policy values are kept in the global settings namespace so the
// admin UI can manage GPT request parameters across GPT channels.
type GPTRequestFastPolicy string

const (
	GPTRequestFastPolicyDisabled GPTRequestFastPolicy = "disabled"
	GPTRequestFastPolicyAllow    GPTRequestFastPolicy = "allow"
)

type GPTRequestReasoningPolicy string

const (
	GPTRequestReasoningPolicyClient   GPTRequestReasoningPolicy = "client"
	GPTRequestReasoningPolicyCapHigh  GPTRequestReasoningPolicy = "cap_high"
	GPTRequestReasoningPolicyCapXHigh GPTRequestReasoningPolicy = "cap_xhigh"
)

func (p ChatCompletionsToResponsesPolicy) IsChannelEnabled(channelID int, channelType int) bool {
	if !p.Enabled {
		return false
	}
	if p.AllChannels {
		return true
	}

	if channelID > 0 && len(p.ChannelIDs) > 0 && slices.Contains(p.ChannelIDs, channelID) {
		return true
	}
	if channelType > 0 && len(p.ChannelTypes) > 0 && slices.Contains(p.ChannelTypes, channelType) {
		return true
	}
	return false
}

type GlobalSettings struct {
	PassThroughRequestEnabled        bool                             `json:"pass_through_request_enabled"`
	ThinkingModelBlacklist           []string                         `json:"thinking_model_blacklist"`
	ChatCompletionsToResponsesPolicy ChatCompletionsToResponsesPolicy `json:"chat_completions_to_responses_policy"`
	// GPTRequestPolicyTags is retained for backwards-compatible option
	// migration. GPT request policies now apply to every supported GPT channel.
	GPTRequestPolicyTags      string                    `json:"gpt_request_policy.tags"`
	GPTRequestFastPolicy      GPTRequestFastPolicy      `json:"gpt_request_policy.fast_policy"`
	GPTRequestReasoningPolicy GPTRequestReasoningPolicy `json:"gpt_request_policy.reasoning_policy"`
}

// 默认配置
var defaultOpenaiSettings = GlobalSettings{
	PassThroughRequestEnabled: false,
	ThinkingModelBlacklist: []string{
		"moonshotai/kimi-k2-thinking",
		"kimi-k2-thinking",
	},
	ChatCompletionsToResponsesPolicy: ChatCompletionsToResponsesPolicy{
		Enabled:     false,
		AllChannels: true,
	},
	GPTRequestPolicyTags:      "codex2api",
	GPTRequestFastPolicy:      GPTRequestFastPolicyDisabled,
	GPTRequestReasoningPolicy: GPTRequestReasoningPolicyClient,
}

// 全局实例
var globalSettings = defaultOpenaiSettings

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("global", &globalSettings)
}

func GetGlobalSettings() *GlobalSettings {
	return &globalSettings
}

// MatchesGPTRequestPolicyTag is retained for compatibility with older callers.
// It is no longer used to activate the global GPT request policies.
func (s *GlobalSettings) MatchesGPTRequestPolicyTag(channelTag string) bool {
	if s == nil {
		return false
	}

	channelTag = strings.TrimSpace(channelTag)
	if channelTag == "" {
		return false
	}

	for _, configuredTag := range strings.Split(s.GPTRequestPolicyTags, ",") {
		if strings.TrimSpace(configuredTag) == channelTag {
			return true
		}
	}
	return false
}

// AllowsGPTFast reports whether the global policy allows forwarding the fast
// service tier for GPT requests. The legacy GPTRequestPolicyTags setting is
// intentionally not consulted; it remains stored for backwards compatibility.
// The optional tag argument is accepted for source compatibility and ignored.
func (s *GlobalSettings) AllowsGPTFast(_ ...string) bool {
	return s != nil && s.GPTRequestFastPolicy == GPTRequestFastPolicyAllow
}

// GPTReasoningEffortCap returns the configured maximum reasoning effort for
// GPT requests. An empty result means client values are preserved.
func (s *GlobalSettings) GPTReasoningEffortCap(_ ...string) string {
	if s == nil {
		return ""
	}
	switch s.GPTRequestReasoningPolicy {
	case GPTRequestReasoningPolicyCapHigh:
		return "high"
	case GPTRequestReasoningPolicyCapXHigh:
		return "xhigh"
	default:
		return ""
	}
}

// CapsGPTReasoningAtHigh reports whether reasoning effort above high should be
// lowered for GPT requests. The legacy GPTRequestPolicyTags setting is
// intentionally not consulted; it remains stored for backwards compatibility.
// The optional tag argument is accepted for source compatibility and ignored.
func (s *GlobalSettings) CapsGPTReasoningAtHigh(_ ...string) bool {
	return s.GPTReasoningEffortCap() == "high"
}

// ShouldPreserveThinkingSuffix 判断模型是否配置为保留 thinking/-nothinking/-low/-high/-medium 后缀
func ShouldPreserveThinkingSuffix(modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}

	for _, entry := range globalSettings.ThinkingModelBlacklist {
		if strings.TrimSpace(entry) == target {
			return true
		}
	}
	return false
}
