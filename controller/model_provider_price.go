package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func ListModelProviderPrice(c *gin.Context) {
	modelId, err := strconv.Atoi(c.Query("model_id"))
	if err != nil || modelId <= 0 {
		common.ApiErrorMsg(c, "model_id is required")
		return
	}
	prices, err := model.ListModelProviderPrice(modelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, prices)
}

func ListProviderModels(c *gin.Context) {
	providerSlug := c.Param("provider_slug")
	if providerSlug == "" {
		providerSlug = c.Query("provider_slug")
	}
	items, err := model.ListProviderModels(providerSlug)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	pageInfo.SetTotal(len(items))
	start := pageInfo.GetStartIdx()
	if start > len(items) {
		start = len(items)
	}
	end := start + pageInfo.GetPageSize()
	if end > len(items) {
		end = len(items)
	}
	pageInfo.SetItems(items[start:end])
	common.ApiSuccess(c, pageInfo)
}

func CreateModelProviderPrice(c *gin.Context) {
	var price model.ModelProviderPrice
	if err := c.ShouldBindJSON(&price); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := price.Create(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidatePricingCache()
	common.ApiSuccess(c, &price)
}

func UpdateModelProviderPrice(c *gin.Context) {
	var price model.ModelProviderPrice
	if err := c.ShouldBindJSON(&price); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := price.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidatePricingCache()
	common.ApiSuccess(c, &price)
}

func DeleteModelProviderPrice(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteModelProviderPrice(id); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidatePricingCache()
	common.ApiSuccess(c, nil)
}
