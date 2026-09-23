package model_setting

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type ChatCompletionsToResponsesPolicy struct {
	Enabled       bool     `json:"enabled"`
	AllChannels   bool     `json:"all_channels"`
	ChannelIDs    []int    `json:"channel_ids,omitempty"`
	ChannelTypes  []int    `json:"channel_types,omitempty"`
	ModelPatterns []string `json:"model_patterns,omitempty"`
}

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
	PassThroughRequestEnabled bool     `json:"pass_through_request_enabled"`
	ThinkingModelBlacklist    []string `json:"thinking_model_blacklist"`
	// EffortTailModelIDs lists real model IDs that sit inside the GPT/o-series
	// family whitelist but whose names already end in an effort word.
	EffortTailModelIDs               []string                         `json:"effort_tail_model_ids"`
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
	EffortTailModelIDs: []string{
		"gpt-5.1-codex-max",
		"qwen-image-edit-max",
		"qwen-max",
		"stable-diffusion-3-medium",
		"yi-medium",
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

func (s *GlobalSettings) AllowsGPTFast(_ ...string) bool {
	return s != nil && s.GPTRequestFastPolicy == GPTRequestFastPolicyAllow
}

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

func (s *GlobalSettings) CapsGPTReasoningAtHigh(_ ...string) bool {
	return s.GPTReasoningEffortCap() == "high"
}

const thinkingBlacklistRegexPrefix = "re:"

type thinkingBlacklistCompiled struct {
	source  string
	exact   []string
	regexes []*regexp.Regexp
}

var (
	thinkingBlacklistMu    sync.RWMutex
	thinkingBlacklistCache thinkingBlacklistCompiled
)

func thinkingBlacklistSourceKey(entries []string) string {
	return strings.Join(entries, "\x00")
}

func compiledThinkingBlacklist() ([]string, []*regexp.Regexp) {
	entries := globalSettings.ThinkingModelBlacklist
	key := thinkingBlacklistSourceKey(entries)

	thinkingBlacklistMu.RLock()
	if thinkingBlacklistCache.source == key {
		exact, regexes := thinkingBlacklistCache.exact, thinkingBlacklistCache.regexes
		thinkingBlacklistMu.RUnlock()
		return exact, regexes
	}
	thinkingBlacklistMu.RUnlock()

	thinkingBlacklistMu.Lock()
	defer thinkingBlacklistMu.Unlock()
	if thinkingBlacklistCache.source == key {
		return thinkingBlacklistCache.exact, thinkingBlacklistCache.regexes
	}

	exact := make([]string, 0, len(entries))
	var regexes []*regexp.Regexp
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if after, ok := strings.CutPrefix(entry, thinkingBlacklistRegexPrefix); ok {
			pattern := after
			if pattern == "" {
				common.SysError(fmt.Sprintf("invalid thinking_model_blacklist regex %q: pattern is empty", entry))
				continue
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				common.SysError(fmt.Sprintf("invalid thinking_model_blacklist regex %q: %v", entry, err))
				continue
			}
			regexes = append(regexes, re)
			continue
		}
		exact = append(exact, entry)
	}
	thinkingBlacklistCache = thinkingBlacklistCompiled{source: key, exact: exact, regexes: regexes}
	return exact, regexes
}

// ShouldPreserveThinkingSuffix reports whether the full model name is exempt
// from host thinking-suffix and @-modifier parsing. Exact blacklist entries
// match the complete name; entries prefixed with re: are Go regular expressions
// matched with MatchString against the same full name.
func ShouldPreserveThinkingSuffix(modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}

	exact, regexes := compiledThinkingBlacklist()
	if slices.Contains(exact, target) {
		return true
	}
	for _, re := range regexes {
		if re.MatchString(target) {
			return true
		}
	}
	return false
}

// ShouldPreserveEffortTail reports whether modelName is a real model ID whose
// name already ends in an effort word. Entries match the complete name and the
// de-namespaced bare name.
func ShouldPreserveEffortTail(modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}
	bare := target
	if slash := strings.LastIndex(target, "/"); slash >= 0 {
		bare = target[slash+1:]
	}

	for _, entry := range globalSettings.EffortTailModelIDs {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if entry == target || entry == bare {
			return true
		}
	}
	return false
}
