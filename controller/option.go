package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

var completionRatioMetaOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

func isPaymentComplianceOptionKey(key string) bool {
	return strings.HasPrefix(key, "payment_setting.compliance_")
}

func isPositiveOptionValue(value string) bool {
	intValue, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		return intValue > 0
	}
	floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && floatValue > 0
}

func collectModelNamesFromOptionValue(raw string, modelNames map[string]struct{}) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
		return
	}

	for modelName := range parsed {
		modelNames[modelName] = struct{}{}
	}
}

func buildCompletionRatioMetaValue(optionValues map[string]string) string {
	modelNames := make(map[string]struct{})
	for _, key := range completionRatioMetaOptionKeys {
		collectModelNamesFromOptionValue(optionValues[key], modelNames)
	}

	meta := make(map[string]ratio_setting.CompletionRatioInfo, len(modelNames))
	for modelName := range modelNames {
		meta[modelName] = ratio_setting.GetCompletionRatioInfo(modelName)
	}

	jsonBytes, err := common.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetOptions(c *gin.Context) {
	var options []*model.Option
	optionValues := make(map[string]string)
	channelModelWeComBotWebhookKeyConfigured := false
	common.OptionMapRWMutex.Lock()
	for k, v := range common.OptionMap {
		if k == "theme.frontend" {
			continue
		}
		value := common.Interface2String(v)
		if k == "monitor_setting.channel_model_wecom_bot_webhook_key" {
			channelModelWeComBotWebhookKeyConfigured = strings.TrimSpace(value) != ""
			continue
		}
		isSensitiveKey := strings.HasSuffix(k, "Token") ||
			strings.HasSuffix(k, "Secret") ||
			strings.HasSuffix(k, "Key") ||
			strings.HasSuffix(k, "secret") ||
			strings.HasSuffix(k, "api_key")
		if isSensitiveKey {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: value,
		})
		for _, optionKey := range completionRatioMetaOptionKeys {
			if optionKey == k {
				optionValues[k] = value
				break
			}
		}
	}
	common.OptionMapRWMutex.Unlock()
	options = append(options, &model.Option{
		Key:   "monitor_setting.channel_model_wecom_bot_webhook_key_configured",
		Value: strconv.FormatBool(channelModelWeComBotWebhookKeyConfigured),
	})
	options = append(options, &model.Option{
		Key:   "CompletionRatioMeta",
		Value: buildCompletionRatioMetaValue(optionValues),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}

type OptionUpdateRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func UpdateOption(c *gin.Context) {
	var option OptionUpdateRequest
	err := common.DecodeJson(c.Request.Body, &option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	switch option.Value.(type) {
	case bool:
		option.Value = common.Interface2String(option.Value.(bool))
	case float64:
		option.Value = common.Interface2String(option.Value.(float64))
	case int:
		option.Value = common.Interface2String(option.Value.(int))
	default:
		option.Value = fmt.Sprintf("%v", option.Value)
	}
	switch option.Key {
	case "QuotaForInviter", "QuotaForInvitee":
		if isPositiveOptionValue(option.Value.(string)) && !operation_setting.IsPaymentComplianceConfirmed() {
			common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
			return
		}
	default:
		if isPaymentComplianceOptionKey(option.Key) {
			common.ApiErrorMsg(c, "合规确认字段不允许通过通用设置接口修改")
			return
		}
	}
	switch option.Key {
	case "GitHubOAuthEnabled":
		if option.Value == "true" && common.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！",
			})
			return
		}
	case "discord.enabled":
		if option.Value == "true" && system_setting.GetDiscordSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！",
			})
			return
		}
	case "oidc.enabled":
		if option.Value == "true" && system_setting.GetOIDCSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！",
			})
			return
		}
	case "LinuxDOOAuthEnabled":
		if option.Value == "true" && common.LinuxDOClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！",
			})
			return
		}
	case "EmailDomainRestrictionEnabled":
		if option.Value == "true" && len(common.EmailDomainWhitelist) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用邮箱域名限制，请先填入限制的邮箱域名！",
			})
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && common.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用微信登录，请先填入微信登录相关配置信息！",
			})
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && common.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！",
			})

			return
		}
	case "TelegramOAuthEnabled":
		if option.Value == "true" && common.TelegramBotToken == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Telegram OAuth，请先填入 Telegram Bot Token！",
			})
			return
		}
	case "theme.frontend":
		if option.Value != "default" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Classic 前端已移除，主题只能设置为 default",
			})
			return
		}
	case "GroupRatio":
		err = ratio_setting.CheckGroupRatio(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "gemini.safety_settings":
		err = model_setting.ValidateGeminiSafetySettings(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "claude.default_max_tokens":
		err = model_setting.ValidateClaudeDefaultMaxTokens(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case operation_setting.ToolPriceOptionKey:
		err = operation_setting.ValidateToolPricesJSON(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "ImageRatio":
		err = ratio_setting.UpdateImageRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "图片倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioRatio":
		err = ratio_setting.UpdateAudioRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioCompletionRatio":
		err = ratio_setting.UpdateAudioCompletionRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频补全倍率设置失败: " + err.Error(),
			})
			return
		}
	case "CreateCacheRatio":
		err = ratio_setting.UpdateCreateCacheRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "缓存创建倍率设置失败: " + err.Error(),
			})
			return
		}
	case "ModelRequestRateLimitGroup":
		err = setting.CheckModelRequestRateLimitGroup(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticDisableStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "monitor_setting.channel_error_status_codes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "monitor_setting.channel_model_circuit_breaker_enabled", "monitor_setting.channel_model_recovery_enabled":
		if _, parseErr := strconv.ParseBool(option.Value.(string)); parseErr != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "渠道模型熔断开关必须是布尔值",
			})
			return
		}
	case "monitor_setting.channel_model_wecom_bot_enabled":
		enabled, parseErr := strconv.ParseBool(option.Value.(string))
		if parseErr != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "企业微信机器人通知开关必须是布尔值"})
			return
		}
		setting := operation_setting.GetMonitorSetting()
		if enabled && (strings.TrimSpace(setting.ChannelModelWeComBotURL) == "" || strings.TrimSpace(setting.ChannelModelWeComBotWebhookKey) == "") {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "启用企业微信机器人通知前必须配置 URL 和 Webhook Key"})
			return
		}
	case "monitor_setting.channel_model_wecom_bot_url":
		value := strings.TrimSpace(option.Value.(string))
		parsedURL, parseErr := url.ParseRequestURI(value)
		if parseErr != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || len(value) > 2048 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "企业微信机器人 URL 必须是有效的 HTTP 或 HTTPS 地址"})
			return
		}
		option.Value = value
	case "monitor_setting.channel_model_wecom_bot_webhook_key":
		value := strings.TrimSpace(option.Value.(string))
		if value == "" || len(value) > 1024 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Webhook Key 不能为空且长度不能超过 1024"})
			return
		}
		option.Value = value
	case "monitor_setting.channel_model_wecom_bot_contact":
		value := strings.TrimSpace(option.Value.(string))
		if len(value) > 2048 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "提醒成员长度不能超过 2048"})
			return
		}
		option.Value = value
	case "monitor_setting.channel_model_excluded_channel_ids":
		_, normalized, parseErr := operation_setting.ParseChannelModelExcludedChannelIDs(option.Value.(string))
		if parseErr != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": parseErr.Error(),
			})
			return
		}
		option.Value = normalized
	case "monitor_setting.channel_error_window_minutes":
		value, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || value < 1 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "渠道失败统计窗口必须是大于 0 的整数",
			})
			return
		}
	case "monitor_setting.channel_error_threshold", "monitor_setting.channel_error_consecutive_threshold":
		value, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || value < 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "渠道失败阈值必须是非负整数",
			})
			return
		}
	case "monitor_setting.channel_model_recovery_success_threshold":
		value, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || value < 1 || value > operation_setting.MaxChannelModelRecoverySuccessThreshold {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("恢复连续成功阈值必须是 1 到 %d 的整数", operation_setting.MaxChannelModelRecoverySuccessThreshold),
			})
			return
		}
	case "monitor_setting.channel_model_recovery_delay_minutes":
		value, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || value < 0 || value > operation_setting.MaxChannelModelRecoveryDelayMinutes {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("恢复探测等待时间必须是 0 到 %d 的整数", operation_setting.MaxChannelModelRecoveryDelayMinutes),
			})
			return
		}
	case "monitor_setting.channel_model_recovery_interval_minutes":
		value, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || value < 1 || value > operation_setting.MaxChannelModelRecoveryIntervalMinutes {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("恢复探测间隔必须是 1 到 %d 的整数", operation_setting.MaxChannelModelRecoveryIntervalMinutes),
			})
			return
		}
	case "monitor_setting.channel_model_recovery_concurrency":
		value, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || value < 1 || value > operation_setting.MaxChannelModelRecoveryConcurrency {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("恢复探测并发数必须是 1 到 %d 的整数", operation_setting.MaxChannelModelRecoveryConcurrency),
			})
			return
		}
	case "monitor_setting.channel_model_event_retention_days":
		value, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || value < 1 || value > operation_setting.MaxChannelModelEventRetentionDays {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("渠道模型事件日志保留天数必须是 1 到 %d 的整数", operation_setting.MaxChannelModelEventRetentionDays),
			})
			return
		}
	case "AutomaticRetryStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.api_info":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "ApiInfo")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.announcements":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "Announcements")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.faq":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "FAQ")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.uptime_kuma_groups":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "UptimeKumaGroups")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	isCircuitBreakerUpdate := option.Key == "monitor_setting.channel_model_circuit_breaker_enabled"
	enableCircuitBreaker := isCircuitBreakerUpdate && option.Value == "true"
	if enableCircuitBreaker && !operation_setting.IsChannelModelCircuitBreakerEnabled() {
		if err := service.ResetChannelModelCircuitBreakerState("circuit breaker enabled from clean state"); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	err = model.UpdateOption(option.Key, option.Value.(string))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if isCircuitBreakerUpdate && !enableCircuitBreaker {
		if err := service.ResetChannelModelCircuitBreakerState("circuit breaker disabled"); err != nil {
			common.SysError("failed to reset channel-model circuit breaker state: " + err.Error())
		}
	}
	if option.Key == "monitor_setting.channel_model_excluded_channel_ids" {
		channelIDs, _, parseErr := operation_setting.ParseChannelModelExcludedChannelIDs(option.Value.(string))
		if parseErr == nil {
			if err := service.ResetExcludedChannelModelCircuitBreakerState(channelIDs, "channel excluded from circuit breaker"); err != nil {
				common.SysError("failed to reset excluded channel-model state: " + err.Error())
			}
		}
	}
	// 出于安全考虑只记录被修改的配置项名称，不记录配置值（可能含密钥等敏感信息）。
	recordManageAudit(c, "option.update", map[string]interface{}{
		"key": option.Key,
	})
	response := gin.H{
		"success": true,
		"message": "",
	}
	// Return the canonical value only for the GPT policy settings that need
	// immediate frontend reconciliation. Do not echo arbitrary option values,
	// since some settings contain secrets.
	if strings.HasPrefix(option.Key, "global.gpt_request_policy.") {
		common.OptionMapRWMutex.RLock()
		response["data"] = gin.H{
			"key":   option.Key,
			"value": common.OptionMap[option.Key],
		}
		common.OptionMapRWMutex.RUnlock()
	}
	c.JSON(http.StatusOK, response)
}
