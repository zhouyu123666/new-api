package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetChannelForRoutingRetriesWithinProviderBeforeFallback(t *testing.T) {
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	defer func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	}()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = false

	priority := int64(0)
	weight := uint(100)
	for _, channel := range []model.Channel{
		{
			Id:           1,
			Type:         constant.ChannelTypeOpenAI,
			Key:          "key-1",
			Status:       common.ChannelStatusEnabled,
			Name:         "internal-channel-1",
			ProviderSlug: "openai",
			Models:       "provider-retry-model",
			Group:        "default",
			Priority:     &priority,
			Weight:       &weight,
		},
		{
			Id:           2,
			Type:         constant.ChannelTypeOpenAI,
			Key:          "key-2",
			Status:       common.ChannelStatusEnabled,
			Name:         "internal-channel-2",
			ProviderSlug: "openai",
			Models:       "provider-retry-model",
			Group:        "default",
			Priority:     &priority,
			Weight:       &weight,
		},
	} {
		require.NoError(t, db.Create(&channel).Error)
		require.NoError(t, db.Create(&model.Ability{
			Group:     "default",
			Model:     "provider-retry-model",
			ChannelId: channel.Id,
			Enabled:   true,
		}).Error)
	}

	routing := &model.ProviderRouting{
		Order:             []string{"openai"},
		AllowFallbacks:    false,
		HasAllowFallbacks: true,
	}
	first, err := getChannelForRouting("default", "provider-retry-model", "/v1/chat/completions", 0, routing, nil)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := getChannelForRouting(
		"default",
		"provider-retry-model",
		"/v1/chat/completions",
		1,
		routing,
		map[int]struct{}{first.Id: {}},
	)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.NotEqual(t, first.Id, second.Id)
}
