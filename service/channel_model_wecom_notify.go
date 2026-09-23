package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const channelModelWeComBotLimitPerMinute = 20

var channelModelWeComBotLimitStore = struct {
	sync.Mutex
	values map[string][]time.Time
}{values: make(map[string][]time.Time)}

var channelModelWeComBotHTTPClient = func() *http.Client {
	// This endpoint is configured only by the root administrator and may be an
	// internal AlertPlus service. Use the operator-managed outbound client so a
	// private DNS result is allowed without weakening SSRF protection for URLs
	// controlled by ordinary users.
	return GetHttpClient()
}

type channelModelWeComBotPayload struct {
	AlarmType  string `json:"alarm_type"`
	Content    string `json:"content"`
	Contact    string `json:"contact"`
	MsgType    string `json:"msg_type"`
	WebhookKey string `json:"webhook_key"`
}

type channelModelWeComBotResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func NotifyChannelModelWeComBot(content string) error {
	setting := operation_setting.GetMonitorSetting()
	if !setting.ChannelModelWeComBotEnabled {
		return nil
	}
	if setting.ChannelModelWeComBotURL == "" || setting.ChannelModelWeComBotWebhookKey == "" {
		return errors.New("channel model WeCom bot notification is enabled but URL or webhook key is empty")
	}
	allowed, err := acquireChannelModelWeComBotLimit(setting.ChannelModelWeComBotWebhookKey)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New("channel model WeCom bot notification rate limit exceeded")
	}

	payloadBytes, err := common.Marshal(channelModelWeComBotPayload{
		AlarmType:  "bot",
		Content:    content,
		Contact:    setting.ChannelModelWeComBotContact,
		MsgType:    "text",
		WebhookKey: setting.ChannelModelWeComBotWebhookKey,
	})
	if err != nil {
		return fmt.Errorf("marshal channel model WeCom bot payload: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.ChannelModelWeComBotURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("create channel model WeCom bot request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := channelModelWeComBotHTTPClient()
	if client == nil {
		return errors.New("HTTP client is not initialized")
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send channel model WeCom bot request: %w", err)
	}
	defer CloseResponseBodyGracefully(resp)
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("read channel model WeCom bot response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("channel model WeCom bot returned HTTP %d", resp.StatusCode)
	}
	var result channelModelWeComBotResponse
	if err := common.Unmarshal(responseBody, &result); err != nil {
		return fmt.Errorf("decode channel model WeCom bot response: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("channel model WeCom bot returned errcode %d: %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

func acquireChannelModelWeComBotLimit(webhookKey string) (bool, error) {
	now := time.Now()
	keyHash := common.Sha1([]byte(webhookKey))
	if common.RedisEnabled && common.RDB != nil {
		redisKey := fmt.Sprintf("channel_model_wecom_notify:v1:%s", keyHash)
		allowed, err := common.RDB.Eval(
			context.Background(),
			`redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
local count = redis.call('ZCARD', KEYS[1])
if count >= tonumber(ARGV[4]) then return 0 end
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[3])
redis.call('PEXPIRE', KEYS[1], 120000)
return 1`,
			[]string{redisKey},
			now.Add(-time.Minute).UnixMilli(),
			now.UnixMilli(),
			fmt.Sprintf("%d-%s", now.UnixNano(), common.GetRandomString(8)),
			channelModelWeComBotLimitPerMinute,
		).Int64()
		if err != nil {
			return false, fmt.Errorf("rate limit channel model WeCom bot notification: %w", err)
		}
		return allowed == 1, nil
	}

	channelModelWeComBotLimitStore.Lock()
	defer channelModelWeComBotLimitStore.Unlock()
	cutoff := now.Add(-time.Minute)
	for storedKey, storedTimestamps := range channelModelWeComBotLimitStore.values {
		if len(storedTimestamps) == 0 || !storedTimestamps[len(storedTimestamps)-1].After(cutoff) {
			delete(channelModelWeComBotLimitStore.values, storedKey)
		}
	}
	timestamps := channelModelWeComBotLimitStore.values[keyHash]
	firstRetained := 0
	for firstRetained < len(timestamps) && !timestamps[firstRetained].After(cutoff) {
		firstRetained++
	}
	if firstRetained > 0 {
		timestamps = append([]time.Time(nil), timestamps[firstRetained:]...)
	}
	if len(timestamps) >= channelModelWeComBotLimitPerMinute {
		channelModelWeComBotLimitStore.values[keyHash] = timestamps
		return false, nil
	}
	channelModelWeComBotLimitStore.values[keyHash] = append(timestamps, now)
	return true, nil
}
