package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type modelSquareProvider struct {
	Slug          string                       `json:"slug"`
	Name          string                       `json:"name"`
	Icon          string                       `json:"icon,omitempty"`
	WebsiteURL    string                       `json:"website_url,omitempty"`
	StatusPageURL string                       `json:"status_page_url,omitempty"`
	Available     bool                         `json:"available"`
	Pricing       *modelSquareProviderPricing  `json:"pricing,omitempty"`
	Metadata      *modelSquareProviderMetadata `json:"metadata,omitempty"`
}

type modelSquareProviderPricing struct {
	InputPrice      float64  `json:"input_price"`
	OutputPrice     float64  `json:"output_price"`
	CacheReadPrice  *float64 `json:"cache_read_price,omitempty"`
	CacheWritePrice *float64 `json:"cache_write_price,omitempty"`
	Source          string   `json:"source"`
}

type modelSquareProviderMetadata struct {
	ModelName           string   `json:"model_name,omitempty"`
	ContextLength       int64    `json:"context_length,omitempty"`
	MaxOutputTokens     int64    `json:"max_output_tokens,omitempty"`
	Region              string   `json:"region,omitempty"`
	Precision           string   `json:"precision,omitempty"`
	Quantization        string   `json:"quantization,omitempty"`
	SupportedParameters []string `json:"supported_parameters,omitempty"`
	StreamCancellation  *bool    `json:"stream_cancellation,omitempty"`
	Free                *bool    `json:"free,omitempty"`
	Batch               *bool    `json:"batch,omitempty"`
	SourceURL           string   `json:"source_url,omitempty"`
	EffectiveAt         int64    `json:"effective_at,omitempty"`
}

func providerMetadataFromPrice(price model.ModelProviderPrice) *modelSquareProviderMetadata {
	if price.ModelName == "" && price.ContextLength == 0 &&
		price.MaxOutputTokens == 0 && price.Region == "" &&
		price.Precision == "" && price.Quantization == "" &&
		price.SupportedParameters == "" && price.StreamCancellation == nil &&
		price.Free == nil && price.Batch == nil && price.SourceURL == "" &&
		price.EffectiveAt == 0 {
		return nil
	}
	metadata := &modelSquareProviderMetadata{
		ModelName:          price.ModelName,
		ContextLength:      price.ContextLength,
		MaxOutputTokens:    price.MaxOutputTokens,
		Region:             price.Region,
		Precision:          price.Precision,
		Quantization:       price.Quantization,
		StreamCancellation: price.StreamCancellation,
		Free:               price.Free,
		Batch:              price.Batch,
		SourceURL:          price.SourceURL,
		EffectiveAt:        price.EffectiveAt,
	}
	if price.SupportedParameters != "" {
		var supported []string
		if err := common.Unmarshal([]byte(price.SupportedParameters), &supported); err == nil {
			metadata.SupportedParameters = supported
		}
	}
	return metadata
}

type modelSquareItem struct {
	ID                     int                            `json:"id"`
	ModelName              string                         `json:"model_name"`
	CatalogSlug            string                         `json:"catalog_slug"`
	Description            string                         `json:"description,omitempty"`
	Icon                   string                         `json:"icon,omitempty"`
	Tags                   string                         `json:"tags,omitempty"`
	VendorID               int                            `json:"vendor_id,omitempty"`
	VendorName             string                         `json:"vendor_name,omitempty"`
	VendorIcon             string                         `json:"vendor_icon,omitempty"`
	VendorDescription      string                         `json:"vendor_description,omitempty"`
	ContextLength          int64                          `json:"context_length,omitempty"`
	ParameterCount         string                         `json:"parameter_count,omitempty"`
	ReleaseDate            string                         `json:"release_date,omitempty"`
	QuotaType              int                            `json:"quota_type"`
	ModelRatio             float64                        `json:"model_ratio"`
	ModelPrice             float64                        `json:"model_price"`
	CompletionRatio        float64                        `json:"completion_ratio"`
	CacheRatio             *float64                       `json:"cache_ratio,omitempty"`
	CreateCacheRatio       *float64                       `json:"create_cache_ratio,omitempty"`
	EnableGroup            []string                       `json:"enable_groups"`
	SupportedEndpointTypes []string                       `json:"supported_endpoint_types"`
	EndpointMap            map[string]common.EndpointInfo `json:"endpoint_map,omitempty"`
	BillingMode            string                         `json:"billing_mode,omitempty"`
	BillingExpr            string                         `json:"billing_expr,omitempty"`
	Providers              []modelSquareProvider          `json:"providers,omitempty"`
}

func modelSquareItemFromModel(
	meta *model.Model,
	pricingByName map[string]model.Pricing,
	vendors map[int]model.Vendor,
	providerPrices map[string]model.ModelProviderPrice,
	providerMetadata map[string]model.Provider,
) modelSquareItem {
	catalogSlug := meta.ModelName
	if vendor, ok := vendors[meta.VendorID]; ok && vendor.Name != "" {
		ownerSlug := strings.ToLower(strings.TrimSpace(vendor.Name))
		ownerSlug = strings.NewReplacer(" ", "-", "/", "-", "_", "-").Replace(ownerSlug)
		catalogSlug = ownerSlug + "/" + meta.ModelName
	}
	item := modelSquareItem{
		ID:                     meta.Id,
		ModelName:              meta.ModelName,
		CatalogSlug:            catalogSlug,
		Description:            meta.Description,
		Icon:                   meta.Icon,
		Tags:                   meta.Tags,
		VendorID:               meta.VendorID,
		ContextLength:          meta.ContextLength,
		ParameterCount:         meta.ParameterCount,
		ReleaseDate:            meta.ReleaseDate,
		EnableGroup:            []string{},
		SupportedEndpointTypes: []string{},
		Providers:              []modelSquareProvider{},
	}
	if vendor, ok := vendors[meta.VendorID]; ok {
		item.VendorName = vendor.Name
		item.VendorIcon = vendor.Icon
		item.VendorDescription = vendor.Description
	}
	if pricing, ok := pricingByName[meta.ModelName]; ok {
		item.QuotaType = pricing.QuotaType
		item.ModelRatio = pricing.ModelRatio
		item.ModelPrice = pricing.ModelPrice
		item.CompletionRatio = pricing.CompletionRatio
		item.CacheRatio = pricing.CacheRatio
		item.CreateCacheRatio = pricing.CreateCacheRatio
		item.EnableGroup = append([]string{}, pricing.EnableGroup...)
		item.SupportedEndpointTypes = make([]string, 0, len(pricing.SupportedEndpointTypes))
		for _, endpoint := range pricing.SupportedEndpointTypes {
			item.SupportedEndpointTypes = append(item.SupportedEndpointTypes, string(endpoint))
		}
		item.BillingMode = pricing.BillingMode
		item.BillingExpr = pricing.BillingExpr
		for _, provider := range pricing.Providers {
			providerItem := modelSquareProvider{
				Slug:      provider.Slug,
				Name:      provider.Name,
				Available: provider.Available,
			}
			if metadata, ok := providerMetadata[provider.Slug]; ok {
				if strings.TrimSpace(metadata.DisplayName) != "" {
					providerItem.Name = metadata.DisplayName
				}
				providerItem.Icon = metadata.Icon
				providerItem.WebsiteURL = metadata.WebsiteURL
				providerItem.StatusPageURL = metadata.StatusPageURL
			}
			if providerPrice, ok := providerPrices[provider.Slug]; ok {
				providerItem.Pricing = &modelSquareProviderPricing{
					InputPrice:      providerPrice.InputPrice,
					OutputPrice:     providerPrice.OutputPrice,
					CacheReadPrice:  providerPrice.CacheReadPrice,
					CacheWritePrice: providerPrice.CacheWritePrice,
					Source:          "model-provider",
				}
				providerItem.Metadata = providerMetadataFromPrice(providerPrice)
			}
			item.Providers = append(item.Providers, providerItem)
		}
	}
	return item
}

func buildModelSquareItems(metas []model.Model) ([]modelSquareItem, error) {
	pricingByName := make(map[string]model.Pricing)
	for _, pricing := range model.GetPricing() {
		pricingByName[pricing.ModelName] = pricing
	}
	var vendorRows []model.Vendor
	if err := model.DB.Find(&vendorRows).Error; err != nil {
		return nil, err
	}
	vendors := make(map[int]model.Vendor, len(vendorRows))
	for _, vendor := range vendorRows {
		vendors[vendor.Id] = vendor
	}
	modelIds := make([]int, 0, len(metas))
	for i := range metas {
		modelIds = append(modelIds, metas[i].Id)
	}
	providerPriceMap, err := model.GetModelProviderPriceMap(modelIds)
	if err != nil {
		return nil, err
	}
	providerMetadata := model.GetProviderMetadataMap()
	items := make([]modelSquareItem, 0, len(metas))
	for i := range metas {
		items = append(items, modelSquareItemFromModel(
			&metas[i],
			pricingByName,
			vendors,
			providerPriceMap[metas[i].Id],
			providerMetadata,
		))
	}
	return items, nil
}

func getModelSquareItems(offset, limit int) ([]modelSquareItem, int64, error) {
	var metas []model.Model
	query := model.DB.Model(&model.Model{}).Where("status = ? AND name_rule = ?", 1, model.NameRuleExact)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = query.Order("id ASC")
	if offset > 0 {
		query = query.Offset(offset)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&metas).Error; err != nil {
		return nil, 0, err
	}
	items, err := buildModelSquareItems(metas)
	return items, total, err
}

func getModelSquareItemByID(id int) (*modelSquareItem, error) {
	var meta model.Model
	if err := model.DB.Where("id = ? AND status = ? AND name_rule = ?", id, 1, model.NameRuleExact).First(&meta).Error; err != nil {
		return nil, err
	}
	items, err := buildModelSquareItems([]model.Model{meta})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("model not found")
	}
	items[0].EndpointMap = getModelSquareEndpointMap(items[0].SupportedEndpointTypes)
	return &items[0], nil
}

func getModelSquareEndpointMap(endpointTypes []string) map[string]common.EndpointInfo {
	if len(endpointTypes) == 0 {
		return nil
	}
	configured := model.GetSupportedEndpointMap()
	result := make(map[string]common.EndpointInfo, len(endpointTypes))
	for _, endpointType := range endpointTypes {
		if info, ok := configured[endpointType]; ok {
			result[endpointType] = info
			continue
		}
		if info, ok := common.GetDefaultEndpointInfo(constant.EndpointType(endpointType)); ok {
			result[endpointType] = info
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func parseModelSquarePagination(c *gin.Context) (int, int, error) {
	offset := 0
	limit := 24
	if rawOffset := c.Query("offset"); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			return 0, 0, errors.New("offset must be a non-negative integer")
		}
		offset = parsed
	}
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return 0, 0, errors.New("limit must be a positive integer")
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}
	return offset, limit, nil
}

func GetModelSquare(c *gin.Context) {
	offset, limit, err := parseModelSquarePagination(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, total, err := getModelSquareItems(offset, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"items":  items,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	}})
}

func GetModelSquareDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := getModelSquareItemByID(id)
	if err != nil {
		common.ApiErrorMsg(c, "model not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

func GetModelSquareProviderDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	providerSlug := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	item, err := getModelSquareItemByID(id)
	if err != nil {
		common.ApiErrorMsg(c, "model not found")
		return
	}
	for _, provider := range item.Providers {
		if provider.Slug == providerSlug {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"model":     item,
					"provider":  provider,
					"groups":    item.EnableGroup,
					"endpoints": item.SupportedEndpointTypes,
				},
			})
			return
		}
	}
	common.ApiErrorMsg(c, "provider not found")
}
