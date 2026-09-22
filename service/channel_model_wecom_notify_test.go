package service

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupChannelModelWeComBotTest(t *testing.T) *operation_setting.MonitorSetting {
	t.Helper()
	setting := operation_setting.GetMonitorSetting()
	previousSetting := *setting
	previousClient := channelModelWeComBotHTTPClient
	previousRedisEnabled := common.RedisEnabled
	channelModelWeComBotLimitStore.Lock()
	previousLimitStore := channelModelWeComBotLimitStore.values
	channelModelWeComBotLimitStore.values = make(map[string][]time.Time)
	channelModelWeComBotLimitStore.Unlock()
	common.RedisEnabled = false
	setting.ChannelModelWeComBotEnabled = true
	setting.ChannelModelWeComBotURL = "https://alert.example/api/alarm/receive/"
	setting.ChannelModelWeComBotWebhookKey = "webhook-secret"
	setting.ChannelModelWeComBotContact = "first@example.com|@all"
	t.Cleanup(func() {
		*operation_setting.GetMonitorSetting() = previousSetting
		channelModelWeComBotHTTPClient = previousClient
		common.RedisEnabled = previousRedisEnabled
		channelModelWeComBotLimitStore.Lock()
		channelModelWeComBotLimitStore.values = previousLimitStore
		channelModelWeComBotLimitStore.Unlock()
	})
	return setting
}

func TestNotifyChannelModelWeComBotSendsExpectedTextPayload(t *testing.T) {
	setupChannelModelWeComBotTest(t)
	var received channelModelWeComBotPayload
	channelModelWeComBotHTTPClient = func() *http.Client {
		return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, "application/json", req.Header.Get("Content-Type"))
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.NoError(t, common.Unmarshal(body, &received))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok"}`)),
				Header:     make(http.Header),
			}, nil
		})}
	}

	require.NoError(t, NotifyChannelModelWeComBot("channel model disabled"))
	assert.Equal(t, "bot", received.AlarmType)
	assert.Equal(t, "channel model disabled", received.Content)
	assert.Equal(t, "first@example.com|@all", received.Contact)
	assert.Equal(t, "text", received.MsgType)
	assert.Equal(t, "webhook-secret", received.WebhookKey)
}

func TestChannelModelWeComNotificationsIncludeSystemName(t *testing.T) {
	setupChannelModelWeComBotTest(t)
	previousSystemName := common.SystemName
	common.SystemName = "测试站点"
	t.Cleanup(func() { common.SystemName = previousSystemName })

	contents := make([]string, 0, 2)
	channelModelWeComBotHTTPClient = func() *http.Client {
		return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload channelModelWeComBotPayload
			require.NoError(t, common.DecodeJson(req.Body, &payload))
			contents = append(contents, payload.Content)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok"}`)),
				Header:     make(http.Header),
			}, nil
		})}
	}

	notifyChannelModelDisabledWeCom(7, "gpt-5.6-luna", "local", http.StatusBadGateway, "upstream failed")
	notifyChannelModelRecoveredWeCom(7, "gpt-5.6-luna", "local", 2, "")

	require.Len(t, contents, 2)
	assert.Contains(t, contents[0], "[渠道模型熔断]\n站点：测试站点\n")
	assert.Contains(t, contents[1], "[渠道模型恢复]\n站点：测试站点\n")
}

func TestNotifyChannelModelWeComBotLimitsEachKeyToTwentyPerRollingMinute(t *testing.T) {
	setupChannelModelWeComBotTest(t)
	sent := 0
	channelModelWeComBotHTTPClient = func() *http.Client {
		return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			sent++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok"}`)),
				Header:     make(http.Header),
			}, nil
		})}
	}

	for range channelModelWeComBotLimitPerMinute {
		require.NoError(t, NotifyChannelModelWeComBot("event"))
	}
	err := NotifyChannelModelWeComBot("event")
	assert.ErrorContains(t, err, "rate limit exceeded")
	assert.Equal(t, channelModelWeComBotLimitPerMinute, sent)
}

func TestNotifyChannelModelWeComBotRejectsNonZeroErrCode(t *testing.T) {
	setupChannelModelWeComBotTest(t)
	channelModelWeComBotHTTPClient = func() *http.Client {
		return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":40001,"errmsg":"invalid key"}`)),
				Header:     make(http.Header),
			}, nil
		})}
	}

	err := NotifyChannelModelWeComBot("event")
	assert.ErrorContains(t, err, "errcode 40001")
	assert.NotContains(t, err.Error(), "webhook-secret")
}
