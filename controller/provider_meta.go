package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func ListProvider(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	providers, total, err := model.GetAllProviders(
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
		c.Query("keyword"),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(providers)
	common.ApiSuccess(c, pageInfo)
}

func GetProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	provider, err := model.GetProviderByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, provider)
}

func CreateProvider(c *gin.Context) {
	var provider model.Provider
	if err := c.ShouldBindJSON(&provider); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := provider.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidatePricingCache()
	common.ApiSuccess(c, &provider)
}

func UpdateProvider(c *gin.Context) {
	var provider model.Provider
	if err := c.ShouldBindJSON(&provider); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := provider.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidatePricingCache()
	common.ApiSuccess(c, &provider)
}

func DeleteProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	provider := &model.Provider{Id: id}
	if err := provider.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidatePricingCache()
	common.ApiSuccess(c, nil)
}
