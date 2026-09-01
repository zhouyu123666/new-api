package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestListProviderModelsReturnsOnlyExplicitlyConfiguredModels(t *testing.T) {
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
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	require.NoError(t, db.Create(&model.Model{
		Id: 1, ModelName: "configured-model", VendorID: 1, Status: 1,
		NameRule: model.NameRuleExact,
	}).Error)
	require.NoError(t, db.Create(&model.Model{
		Id: 2, ModelName: "available-model", VendorID: 1, Status: 1,
		NameRule: model.NameRuleExact,
	}).Error)
	require.NoError(t, db.Create(&model.Vendor{Id: 1, Name: "DeepSeek"}).Error)
	require.NoError(t, db.Create(&model.Provider{
		Slug: "siliconflow", DisplayName: "SiliconFlow", Status: 1,
	}).Error)
	require.NoError(t, db.Create(&model.Provider{
		Slug: "openai", DisplayName: "OpenAI", Status: 1,
	}).Error)
	require.NoError(t, db.Create(&model.ModelProviderPrice{
		ModelId: 1, ProviderSlug: "siliconflow", InputPrice: 1, OutputPrice: 2,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id: 1, Name: "internal", ProviderSlug: "siliconflow",
		Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Model: "available-model", ChannelId: 1, Enabled: true,
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/model-provider-prices/providers/siliconflow/models", nil)
	context.Params = gin.Params{{Key: "provider_slug", Value: "siliconflow"}}

	ListProviderModels(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data struct {
			Items []model.ProviderModelRelation `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, "configured-model", response.Data.Items[0].ModelName)
	assert.True(t, response.Data.Items[0].Configured)
	assert.False(t, response.Data.Items[0].Available)
	assert.Equal(t, "DeepSeek", response.Data.Items[0].VendorName)

	legacyModel := &model.Model{
		Id: 3, ModelName: "legacy-openai-model", VendorID: 1, Status: 1,
		NameRule: model.NameRuleExact,
	}
	require.NoError(t, db.Create(legacyModel).Error)
	require.NoError(t, db.Create(&model.ModelProviderPrice{
		ModelId: 3, ProviderSlug: "openai", InputPrice: 1, OutputPrice: 2,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id: 2, Type: constant.ChannelTypeOpenAI, Name: "legacy-openai",
		Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "default", Model: "legacy-openai-model", ChannelId: 2, Enabled: true,
	}).Error)
	legacyItems, err := model.ListProviderModels("openai")
	require.NoError(t, err)
	require.Len(t, legacyItems, 1)
	assert.True(t, legacyItems[0].Available)
}
