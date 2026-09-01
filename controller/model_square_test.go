package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelSquareControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Model{},
		&model.Vendor{},
		&model.Provider{},
		&model.ModelProviderPrice{},
		&model.Channel{},
		&model.Ability{},
	))
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	return db
}

func TestGetModelSquareSupportsPagination(t *testing.T) {
	db := setupModelSquareControllerTest(t)
	seedModelSquareProvider(t, db)
	for index := 1; index <= 3; index++ {
		require.NoError(t, db.Create(&model.Model{
			Id:        index,
			ModelName: fmt.Sprintf("catalog-model-%d", index),
			Status:    1,
			NameRule:  model.NameRuleExact,
		}).Error)
		seedModelSquarePrice(t, db, index)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/model-square?offset=1&limit=1", nil)

	GetModelSquare(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data struct {
			Items  []modelSquareItem `json:"items"`
			Total  int64              `json:"total"`
			Offset int                `json:"offset"`
			Limit  int                `json:"limit"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, 2, response.Data.Items[0].ID)
	assert.Equal(t, "catalog-model-2", response.Data.Items[0].ModelName)
	assert.Equal(t, int64(3), response.Data.Total)
	assert.Equal(t, 1, response.Data.Offset)
	assert.Equal(t, 1, response.Data.Limit)
}

func TestGetModelSquareUsesDefaultPageSize(t *testing.T) {
	db := setupModelSquareControllerTest(t)
	seedModelSquareProvider(t, db)
	for index := 1; index <= 25; index++ {
		require.NoError(t, db.Create(&model.Model{
			Id:        index,
			ModelName: fmt.Sprintf("default-page-model-%d", index),
			Status:    1,
			NameRule:  model.NameRuleExact,
		}).Error)
		seedModelSquarePrice(t, db, index)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/model-square", nil)

	GetModelSquare(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data struct {
			Items  []modelSquareItem `json:"items"`
			Total  int64         `json:"total"`
			Offset int           `json:"offset"`
			Limit  int           `json:"limit"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Len(t, response.Data.Items, 24)
	assert.Equal(t, int64(25), response.Data.Total)
	assert.Equal(t, 0, response.Data.Offset)
	assert.Equal(t, 24, response.Data.Limit)
}

func TestGetModelSquareExcludesModelsWithoutProviderConfiguration(t *testing.T) {
	db := setupModelSquareControllerTest(t)
	require.NoError(t, db.Create(&model.Model{
		Id:        1,
		ModelName: "channel-only-model",
		Status:    1,
		NameRule:  model.NameRuleExact,
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/model-square", nil)

	GetModelSquare(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data struct {
			Items []modelSquareItem `json:"items"`
			Total int64              `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Empty(t, response.Data.Items)
	assert.Zero(t, response.Data.Total)
}

func TestGetModelSquareDetailExcludesModelsWithoutProviderConfiguration(t *testing.T) {
	db := setupModelSquareControllerTest(t)
	require.NoError(t, db.Create(&model.Model{
		Id:        1,
		ModelName: "channel-only-detail-model",
		Status:    1,
		NameRule:  model.NameRuleExact,
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: "1"}}
	context.Request = httptest.NewRequest(http.MethodGet, "/api/model-square/1", nil)

	GetModelSquareDetail(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	assert.Equal(t, "model not found", response.Message)
}

func seedModelSquareProvider(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&model.Provider{
		Id:          1,
		Slug:        "test-provider",
		DisplayName: "Test Provider",
		Status:      1,
	}).Error)
}

func seedModelSquarePrice(t *testing.T, db *gorm.DB, modelID int) {
	t.Helper()
	require.NoError(t, db.Create(&model.ModelProviderPrice{
		ModelId:      modelID,
		ProviderSlug: "test-provider",
		InputPrice:   1,
		OutputPrice:  2,
	}).Error)
}

func TestModelSquareUsesOnlyExplicitProviderPrices(t *testing.T) {
	item := modelSquareItemFromModel(
		&model.Model{Id: 1, ModelName: "gpt-test"},
		map[string]model.Pricing{
			"gpt-test": {
				Providers: []model.ProviderSummary{
					{Slug: "deepseek", Name: "DeepSeek", Available: true},
					{Slug: "siliconflow", Name: "SiliconFlow", Available: true},
				},
			},
		},
		map[int]model.Vendor{},
		map[string]model.ModelProviderPrice{
			"siliconflow": {
				ModelId:      1,
				ProviderSlug: "siliconflow",
				InputPrice:   1,
				OutputPrice:  2,
			},
		},
		map[string]model.Provider{
			"deepseek": {
				Slug:        "deepseek",
				DisplayName: "DeepSeek",
				Status:      1,
			},
			"siliconflow": {
				Slug:        "siliconflow",
				DisplayName: "SiliconFlow",
				Status:      1,
			},
		},
	)

	require.Len(t, item.Providers, 1)
	assert.Equal(t, "siliconflow", item.Providers[0].Slug)
	assert.Equal(t, "SiliconFlow", item.Providers[0].Name)
	assert.True(t, item.Providers[0].Available)
	require.NotNil(t, item.Providers[0].Pricing)
	assert.Equal(t, float64(1), item.Providers[0].Pricing.InputPrice)
}

func TestModelSquareProviderNameFallsBackWhenMetadataNameIsEmpty(t *testing.T) {
	item := modelSquareItemFromModel(
		&model.Model{Id: 1, ModelName: "fallback-model"},
		map[string]model.Pricing{
			"fallback-model": {
				Providers: []model.ProviderSummary{
					{Slug: "provider-slug", Name: "Provider label", Available: true},
				},
			},
		},
		map[int]model.Vendor{},
		map[string]model.ModelProviderPrice{
			"provider-slug": {
				ModelId:      1,
				ProviderSlug: "provider-slug",
				InputPrice:   1,
				OutputPrice:  2,
			},
		},
		map[string]model.Provider{
			"provider-slug": {Slug: "provider-slug", DisplayName: "", Status: 1},
		},
	)

	require.Len(t, item.Providers, 1)
	assert.Equal(t, "Provider label", item.Providers[0].Name)
}
