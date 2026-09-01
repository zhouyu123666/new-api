package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Provider stores user-facing metadata for an upstream model provider.
// Channels reference providers by slug so multiple internal channels can be
// presented as one provider to end users.
type Provider struct {
	Id                   int    `json:"id" gorm:"primaryKey"`
	Slug                 string `json:"slug" gorm:"size:64;not null;uniqueIndex"`
	DisplayName          string `json:"display_name" gorm:"size:128;not null"`
	Icon                 string `json:"icon,omitempty" gorm:"size:128"`
	WebsiteURL           string `json:"website_url,omitempty" gorm:"size:512"`
	StatusPageURL        string `json:"status_page_url,omitempty" gorm:"size:512"`
	Headquarters         string `json:"headquarters,omitempty" gorm:"size:128"`
	ByokSupported        bool   `json:"byok_supported"`
	PromptTrainingPolicy string `json:"prompt_training_policy,omitempty" gorm:"type:text"`
	RetentionPolicy      string `json:"retention_policy,omitempty" gorm:"type:text"`
	ModerationPolicy     string `json:"moderation_policy,omitempty" gorm:"type:text"`
	MetadataSourceURL    string `json:"metadata_source_url,omitempty" gorm:"size:512"`
	MetadataVerifiedAt   int64  `json:"metadata_verified_at,omitempty" gorm:"bigint"`
	Status               int    `json:"status"`
	CreatedTime          int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime          int64  `json:"updated_time" gorm:"bigint"`
}

func (Provider) TableName() string {
	return "providers"
}

func normalizeProviderRegistrySlug(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// EnsureProviderBySlug creates a minimal registry entry when a channel uses a
// provider that has not been enriched with user-facing metadata yet.
func EnsureProviderBySlug(slug string) error {
	slug = normalizeProviderRegistrySlug(slug)
	if slug == "" {
		return errors.New("provider slug is empty")
	}

	var provider Provider
	err := DB.Where("slug = ?", slug).First(&provider).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	now := common.GetTimestamp()
	return DB.Create(&Provider{
		Slug:        slug,
		DisplayName: slug,
		Status:      1,
		CreatedTime: now,
		UpdatedTime: now,
	}).Error
}

// SeedProvidersFromChannels creates registry rows for all existing channel
// slugs. Existing display names and metadata are preserved.
func SeedProvidersFromChannels() error {
	var slugs []string
	if err := DB.Model(&Channel{}).
		Where("provider_slug IS NOT NULL AND provider_slug <> ?", "").
		Distinct().
		Pluck("provider_slug", &slugs).Error; err != nil {
		return err
	}

	for _, slug := range slugs {
		if err := EnsureProviderBySlug(slug); err != nil {
			return err
		}
	}
	return nil
}

func GetProviderDisplayNames() map[string]string {
	providers := GetProviderMetadataMap()
	result := make(map[string]string, len(providers))
	for slug, provider := range providers {
		if provider.Status != 1 || strings.TrimSpace(provider.DisplayName) == "" {
			continue
		}
		result[slug] = provider.DisplayName
	}
	return result
}

func GetProviderMetadataMap() map[string]Provider {
	var providers []Provider
	if err := DB.Find(&providers).Error; err != nil {
		return map[string]Provider{}
	}

	result := make(map[string]Provider, len(providers))
	for _, provider := range providers {
		slug := normalizeProviderRegistrySlug(provider.Slug)
		if slug == "" {
			continue
		}
		result[slug] = provider
	}
	return result
}

func GetAllProviders(offset, limit int, keyword string) ([]Provider, int64, error) {
	query := DB.Model(&Provider{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("slug LIKE ? OR display_name LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var providers []Provider
	if err := query.Order("id ASC").Offset(offset).Limit(limit).Find(&providers).Error; err != nil {
		return nil, 0, err
	}
	return providers, total, nil
}

func GetProviderByID(id int) (*Provider, error) {
	var provider Provider
	if err := DB.First(&provider, id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func (provider *Provider) Insert() error {
	provider.Slug = normalizeProviderRegistrySlug(provider.Slug)
	provider.DisplayName = strings.TrimSpace(provider.DisplayName)
	if provider.Slug == "" || provider.DisplayName == "" {
		return errors.New("provider slug and display name are required")
	}
	now := common.GetTimestamp()
	provider.Status = 1
	provider.CreatedTime = now
	provider.UpdatedTime = now
	return DB.Create(provider).Error
}

func (provider *Provider) Update() error {
	provider.Slug = normalizeProviderRegistrySlug(provider.Slug)
	provider.DisplayName = strings.TrimSpace(provider.DisplayName)
	if provider.Id <= 0 || provider.Slug == "" || provider.DisplayName == "" {
		return errors.New("provider id, slug and display name are required")
	}
	var existing Provider
	if err := DB.Select("slug").First(&existing, provider.Id).Error; err != nil {
		return err
	}
	if normalizeProviderRegistrySlug(existing.Slug) != provider.Slug {
		return errors.New("provider slug cannot be changed")
	}
	provider.UpdatedTime = common.GetTimestamp()
	return DB.Model(&Provider{}).Where("id = ?", provider.Id).Updates(map[string]any{
		"slug":                   provider.Slug,
		"display_name":           provider.DisplayName,
		"icon":                   provider.Icon,
		"website_url":            provider.WebsiteURL,
		"status_page_url":        provider.StatusPageURL,
		"headquarters":           provider.Headquarters,
		"byok_supported":         provider.ByokSupported,
		"prompt_training_policy": provider.PromptTrainingPolicy,
		"retention_policy":       provider.RetentionPolicy,
		"moderation_policy":      provider.ModerationPolicy,
		"metadata_source_url":    provider.MetadataSourceURL,
		"metadata_verified_at":   provider.MetadataVerifiedAt,
		"status":                 provider.Status,
		"updated_time":           provider.UpdatedTime,
	}).Error
}

func (provider *Provider) Delete() error {
	if provider == nil || provider.Id <= 0 {
		return errors.New("provider id is required")
	}
	var existing Provider
	if err := DB.First(&existing, provider.Id).Error; err != nil {
		return err
	}
	var channelCount int64
	if err := DB.Model(&Channel{}).
		Where("LOWER(provider_slug) = ?", normalizeProviderRegistrySlug(existing.Slug)).
		Count(&channelCount).Error; err != nil {
		return err
	}
	if channelCount > 0 {
		return errors.New("provider is referenced by channels")
	}
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := tx.Where("provider_slug = ?", existing.Slug).Delete(&ModelProviderPrice{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(&Provider{}, provider.Id).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
