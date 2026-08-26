package helper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 裸 [DONE]（无 data: 前缀）必须被识别为正常结束，且不能作为数据块下发。
// 回归此前的解析缺陷：无条件裁剪 5 字节会把 "[DONE]" 变成 "]"。
func TestStreamScannerHandler_BareDoneRecognized(t *testing.T) {
	c, resp, info := setupStreamTest(t, strings.NewReader("data: {\"a\":1}\n[DONE]\n"))
	info.DisablePing = true

	var got []string
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		got = append(got, data)
	})

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.Equal(t, []string{`{"a":1}`}, got)
	assert.Equal(t, 1, info.ReceivedResponseCount)
}

// handler 调用 sr.Done() 后，即使客户端随即断开，也必须保持 done 判定。
// 回归误报：协议终态（如 Responses 的 response.completed）之后客户端立刻关闭连接时，
// 主循环曾先命中 context 取消并把完整请求标记为 client_gone。
func TestStreamScannerHandler_HandlerDoneWinsOverClientDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pr, pw := io.Pipe()
	t.Cleanup(func() {
		_ = pr.Close()
		_ = pw.Close()
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)

	resp := &http.Response{Body: pr}
	info := &relaycommon.RelayInfo{DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{}}

	terminalSent := make(chan struct{})
	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
			if strings.Contains(data, "response.completed") {
				sr.Done()
				close(terminalSent)
			}
		})
		close(done)
	}()

	_, err := fmt.Fprint(pw, "data: {\"type\":\"response.completed\"}\n")
	require.NoError(t, err)

	select {
	case <-terminalSent:
	case <-time.After(2 * time.Second):
		t.Fatal("terminal event not handled")
	}

	// 客户端收到终态后断开，此时上游连接尚未关闭。
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return")
	}

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.True(t, info.StreamStatus.IsNormalEnd())
	assert.False(t, info.StreamStatus.HasErrors())
}

// cleanup 主动关闭 resp.Body 引发的 scanner 报错不应记为 scanner_error，
// 也不应覆盖已确定的结束原因。
func TestStreamScannerHandler_SelfInflictedCloseNotRecordedAsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pr, pw := io.Pipe()
	t.Cleanup(func() {
		_ = pr.Close()
		_ = pw.Close()
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)

	resp := &http.Response{Body: pr}
	info := &relaycommon.RelayInfo{DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{}}

	firstHandled := make(chan struct{})
	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
			close(firstHandled)
		})
		close(done)
	}()

	_, err := fmt.Fprint(pw, "data: {\"a\":1}\n")
	require.NoError(t, err)

	select {
	case <-firstHandled:
	case <-time.After(2 * time.Second):
		t.Fatal("first chunk not handled")
	}

	// 真实的客户端中断：client_gone 是准确结论，不应再叠加 scanner_error。
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return")
	}

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
	assert.Equal(t, 0, info.StreamStatus.TotalErrorCount())
}
