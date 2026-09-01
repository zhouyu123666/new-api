package model

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// ProviderRouting constrains channel selection to a caller-provided provider
// order. The order is evaluated before the existing channel priority/weight
// selection inside each provider.
type ProviderRouting struct {
	Order             []string `json:"order"`
	AllowFallbacks    bool     `json:"allow_fallbacks"`
	HasAllowFallbacks bool     `json:"-"`
}

type ProviderSummary struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

func BuildProviderSummaries(abilities []AbilityWithChannel) map[string][]ProviderSummary {
	channelIDs := make([]int, 0, len(abilities))
	seenChannelIDs := make(map[int]struct{}, len(abilities))
	for _, ability := range abilities {
		if _, exists := seenChannelIDs[ability.ChannelId]; exists {
			continue
		}
		seenChannelIDs[ability.ChannelId] = struct{}{}
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	var channels []Channel
	if len(channelIDs) > 0 {
		_ = DB.Where("id IN ?", channelIDs).Find(&channels).Error
	}
	channelMap := make(map[int]*Channel, len(channels))
	for i := range channels {
		channelMap[channels[i].Id] = &channels[i]
	}
	type providerKey struct {
		model string
		slug  string
	}
	counts := make(map[providerKey]map[int]struct{})
	for _, ability := range abilities {
		channel := channelMap[ability.ChannelId]
		if channel == nil || channel.Status != common.ChannelStatusEnabled {
			continue
		}
		slug := channel.GetProviderSlug()
		if slug == "" {
			continue
		}
		key := providerKey{model: ability.Model, slug: slug}
		if counts[key] == nil {
			counts[key] = make(map[int]struct{})
		}
		counts[key][channel.Id] = struct{}{}
	}
	result := make(map[string][]ProviderSummary)
	providerMetadata := GetProviderMetadataMap()
	for key, channels := range counts {
		metadata := providerMetadata[key.slug]
		name := strings.TrimSpace(metadata.DisplayName)
		if name == "" {
			name = key.slug
		}
		result[key.model] = append(result[key.model], ProviderSummary{
			Slug:      key.slug,
			Name:      name,
			Available: len(channels) > 0 && (metadata.Slug == "" || metadata.Status == 1),
		})
	}
	for modelName := range result {
		sort.Slice(result[modelName], func(i, j int) bool {
			left := result[modelName][i]
			right := result[modelName][j]
			if left.Name != right.Name {
				return left.Name < right.Name
			}
			return left.Slug < right.Slug
		})
	}
	return result
}

func normalizeProviderSlug(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeProviderOrder(order []string) []string {
	seen := make(map[string]struct{}, len(order))
	result := make([]string, 0, len(order))
	for _, value := range order {
		value = normalizeProviderSlug(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (routing *ProviderRouting) Normalized() *ProviderRouting {
	if routing == nil {
		return nil
	}
	order := normalizeProviderOrder(routing.Order)
	if len(order) == 0 {
		return nil
	}
	allowFallbacks := routing.AllowFallbacks
	if !routing.HasAllowFallbacks {
		allowFallbacks = true
	}
	return &ProviderRouting{Order: order, AllowFallbacks: allowFallbacks, HasAllowFallbacks: true}
}

func providerChannelsFromCache(group, modelName, requestPath string, filters []dto.ChannelFilter) []*Channel {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	channels := group2model2channels[group][modelName]
	if len(channels) == 0 {
		channels = group2model2channels[group][ratio_setting.FormatMatchingModelName(modelName)]
	}
	filters = withRequestPathFilter(filters, requestPath)
	channels, _ = filterCandidateIDs(channels, modelName, filters)
	result := make([]*Channel, 0, len(channels))
	seen := make(map[int]struct{}, len(channels))
	for _, channelID := range channels {
		if _, exists := seen[channelID]; exists {
			continue
		}
		channel, exists := channelsIDM[channelID]
		if !exists || channel.Status != common.ChannelStatusEnabled {
			continue
		}
		seen[channelID] = struct{}{}
		result = append(result, channel)
	}
	return result
}

func providerChannelsFromDB(group, modelName, requestPath string, filters []dto.ChannelFilter) ([]*Channel, error) {
	var abilities []Ability
	groupColumn := commonGroupCol
	if groupColumn == "" {
		groupColumn = "`group`"
	}
	if err := DB.Where(groupColumn+" = ? AND model = ? AND enabled = ?", group, modelName, true).Find(&abilities).Error; err != nil {
		return nil, err
	}
	if len(abilities) == 0 {
		normalized := ratio_setting.FormatMatchingModelName(modelName)
		if normalized != modelName {
			if err := DB.Where(groupColumn+" = ? AND model = ? AND enabled = ?", group, normalized, true).Find(&abilities).Error; err != nil {
				return nil, err
			}
		}
	}
	filters = withRequestPathFilter(filters, requestPath)
	abilities = filterAbilitiesByConstraints(abilities, modelName, filters)
	if len(abilities) == 0 {
		return []*Channel{}, nil
	}

	channelIDs := make([]int, 0, len(abilities))
	seen := make(map[int]struct{}, len(abilities))
	for _, ability := range abilities {
		if _, exists := seen[ability.ChannelId]; exists {
			continue
		}
		seen[ability.ChannelId] = struct{}{}
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	var channels []*Channel
	if err := DB.Where("id IN ? AND status = ?", channelIDs, common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

func getProviderChannels(group, modelName, requestPath string, filters []dto.ChannelFilter) ([]*Channel, error) {
	if common.MemoryCacheEnabled {
		return providerChannelsFromCache(group, modelName, requestPath, filters), nil
	}
	return providerChannelsFromDB(group, modelName, requestPath, filters)
}

// GetAvailableProviderSlugs returns providers that can serve the model in the
// requested group. Providers are ordered by their highest configured channel
// priority, then alphabetically for deterministic fallback behavior.
func GetAvailableProviderSlugs(group, modelName, requestPath string) ([]string, error) {
	return GetAvailableProviderSlugsWithFilters(group, modelName, requestPath, nil)
}

// GetAvailableProviderSlugsWithFilters returns providers that satisfy the
// request path and any per-request channel constraints.
func GetAvailableProviderSlugsWithFilters(group, modelName, requestPath string, filters []dto.ChannelFilter) ([]string, error) {
	channels, err := getProviderChannels(group, modelName, requestPath, filters)
	if err != nil {
		return nil, err
	}
	type providerPriority struct {
		slug     string
		priority int64
	}
	priorities := make(map[string]int64)
	providerMetadata := GetProviderMetadataMap()
	for _, channel := range channels {
		slug := channel.GetProviderSlug()
		if slug == "" {
			continue
		}
		if provider, exists := providerMetadata[slug]; exists && provider.Status != 1 {
			continue
		}
		priority := channel.GetPriority()
		if previous, exists := priorities[slug]; !exists || priority > previous {
			priorities[slug] = priority
		}
	}
	result := make([]providerPriority, 0, len(priorities))
	for slug, priority := range priorities {
		result = append(result, providerPriority{slug: slug, priority: priority})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].priority != result[j].priority {
			return result[i].priority > result[j].priority
		}
		return result[i].slug < result[j].slug
	})
	slugs := make([]string, 0, len(result))
	for _, item := range result {
		slugs = append(slugs, item.slug)
	}
	return slugs, nil
}

// GetRandomSatisfiedChannelForProvider selects the best available channel for
// one provider and preserves the existing priority/weight behavior within it.
func GetRandomSatisfiedChannelForProvider(group, modelName, requestPath, provider string) (*Channel, error) {
	return GetRandomSatisfiedChannelForProviderExcluding(
		group,
		modelName,
		requestPath,
		provider,
		nil,
	)
}

// GetRandomSatisfiedChannelForProviderExcluding selects a channel inside one
// provider while skipping channels already attempted by this request.
func GetRandomSatisfiedChannelForProviderExcluding(
	group, modelName, requestPath, provider string,
	excluded map[int]struct{},
) (*Channel, error) {
	return GetRandomSatisfiedChannelForProviderExcludingWithFilters(
		group,
		modelName,
		requestPath,
		provider,
		excluded,
		nil,
	)
}

// GetRandomSatisfiedChannelForProviderExcludingWithFilters selects a channel
// inside one provider while applying the request's channel constraints.
func GetRandomSatisfiedChannelForProviderExcludingWithFilters(
	group, modelName, requestPath, provider string,
	excluded map[int]struct{},
	filters []dto.ChannelFilter,
) (*Channel, error) {
	provider = normalizeProviderSlug(provider)
	if provider == "" {
		return nil, errors.New("provider is empty")
	}
	channels, err := getProviderChannels(group, modelName, requestPath, filters)
	if err != nil {
		return nil, err
	}
	filtered := make([]*Channel, 0, len(channels))
	var targetPriority int64
	hasPriority := false
	for _, channel := range channels {
		if channel.GetProviderSlug() != provider {
			continue
		}
		if _, alreadyTried := excluded[channel.Id]; alreadyTried {
			continue
		}
		priority := channel.GetPriority()
		if !hasPriority || priority > targetPriority {
			targetPriority = priority
			hasPriority = true
			filtered = filtered[:0]
		}
		if priority == targetPriority {
			filtered = append(filtered, channel)
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}

	weightSum := uint(0)
	for _, channel := range filtered {
		weightSum += uint(channel.GetWeight() + 10)
	}
	weight := common.GetRandomInt(int(weightSum))
	for _, channel := range filtered {
		weight -= channel.GetWeight() + 10
		if weight <= 0 {
			return channel, nil
		}
	}
	return filtered[0], nil
}

func withRequestPathFilter(filters []dto.ChannelFilter, requestPath string) []dto.ChannelFilter {
	if requestPath == "" {
		return filters
	}
	for _, filter := range filters {
		if filter.Kind == dto.FilterRequestPath && filter.RequestPath == requestPath {
			return filters
		}
	}
	result := make([]dto.ChannelFilter, 0, len(filters)+1)
	result = append(result, filters...)
	result = append(result, dto.ChannelFilter{
		Kind:        dto.FilterRequestPath,
		RequestPath: requestPath,
	})
	return result
}
