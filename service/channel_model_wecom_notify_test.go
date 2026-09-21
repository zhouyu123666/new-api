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
