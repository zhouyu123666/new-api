package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOptionsHidesChannelModelWeComWebhookKey(t *testing.T) {
	previousMap := common.OptionMap
	common.OptionMap = map[string]string{
		"monitor_setting.channel_model_wecom_bot_webhook_key": "top-secret",
		"monitor_setting.channel_model_wecom_bot_url":         operation_setting.DefaultChannelModelWeComBotURL,
	}
	t.Cleanup(func() { common.OptionMap = previousMap })

	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	GetOptions(ctx)

	require.Equal(t, http.StatusOK, response.Code)
	assert.NotContains(t, response.Body.String(), "top-secret")
	var payload struct {
		Data []model.Option `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	foundConfigured := false
	for _, option := range payload.Data {
		if option.Key == "monitor_setting.channel_model_wecom_bot_webhook_key_configured" {
			foundConfigured = true
			assert.Equal(t, "true", option.Value)
		}
		assert.NotEqual(t, "monitor_setting.channel_model_wecom_bot_webhook_key", option.Key)
	}
	assert.True(t, foundConfigured)
}

func TestUpdateOptionRejectsEnablingChannelModelWeComWithoutWebhookKey(t *testing.T) {
	setting := operation_setting.GetMonitorSetting()
	previousSetting := *setting
	setting.ChannelModelWeComBotURL = operation_setting.DefaultChannelModelWeComBotURL
	setting.ChannelModelWeComBotWebhookKey = ""
	t.Cleanup(func() { *operation_setting.GetMonitorSetting() = previousSetting })

	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"monitor_setting.channel_model_wecom_bot_enabled","value":true}`),
	)
	UpdateOption(ctx)

	require.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
}
