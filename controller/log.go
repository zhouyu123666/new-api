package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func parseLogSpecialFilters(c *gin.Context, allowRetry bool) (streamError bool, retry bool, ok bool) {
	for _, query := range []struct {
		name  string
		value *bool
	}{
		{name: "stream_error", value: &streamError},
		{name: "retry", value: &retry},
	} {
		raw := c.Query(query.name)
		if raw == "" {
			continue
		}
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "invalid boolean query parameter: " + query.name,
			})
			return false, false, false
		}
		*query.value = parsed
	}

	if retry && !allowRetry {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "retry filter is only available for administrator log views",
		})
		return false, false, false
	}
	return streamError, retry, true
}

func validateLogSpecialFilterTimeRange(c *gin.Context, streamError bool, retry bool, startTimestamp int64, endTimestamp int64) bool {
	if !streamError && !retry {
		return true
	}
	if startTimestamp > 0 && endTimestamp >= startTimestamp {
		return true
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"success": false,
		"message": "start_timestamp and end_timestamp are required for stream_error and retry filters",
	})
	return false
}

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	streamError, retry, ok := parseLogSpecialFilters(c, true)
	if !ok || !validateLogSpecialFilterTimeRange(c, streamError, retry, startTimestamp, endTimestamp) {
		return
	}
	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group, requestId, upstreamRequestId, streamError, retry)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	streamError, retry, ok := parseLogSpecialFilters(c, false)
	if !ok || !validateLogSpecialFilterTimeRange(c, streamError, retry, startTimestamp, endTimestamp) {
		return
	}
	logs, total, err := model.GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group, requestId, upstreamRequestId, streamError, retry)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

// Deprecated: SearchAllLogs 已废弃，前端未使用该接口。
func SearchAllLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

// Deprecated: SearchUserLogs 已废弃，前端未使用该接口。
func SearchUserLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}
	logs, err := model.GetLogByTokenId(tokenId)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	streamError, retry, ok := parseLogSpecialFilters(c, true)
	if !ok || !validateLogSpecialFilterTimeRange(c, streamError, retry, startTimestamp, endTimestamp) {
		return
	}
	stat, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group, streamError, retry)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota":       stat.Quota,
			"rpm":         stat.Rpm,
			"tpm":         stat.Tpm,
			"total_count": stat.Total,
			"fast_count":  stat.Fast,
			"fast_ratio":  stat.FastRatio,
		},
	})
	return
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	streamError, retry, ok := parseLogSpecialFilters(c, false)
	if !ok || !validateLogSpecialFilterTimeRange(c, streamError, retry, startTimestamp, endTimestamp) {
		return
	}
	quotaNum, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group, streamError, retry)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota":       quotaNum.Quota,
			"rpm":         quotaNum.Rpm,
			"tpm":         quotaNum.Tpm,
			"total_count": quotaNum.Total,
			"fast_count":  quotaNum.Fast,
			"fast_ratio":  quotaNum.FastRatio,
			//"token": tokenNum,
		},
	})
	return
}
