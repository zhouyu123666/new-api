package observability

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNewFromEnvDisabled(t *testing.T) {
	t.Setenv("NEW_API_OTEL_ENABLED", "false")
	runtime, err := NewFromEnv()
	require.NoError(t, err)
	require.False(t, runtime.Enabled())
}

func TestNewFromEnvAcceptsLangfuseBaseURL(t *testing.T) {
	t.Setenv("NEW_API_OTEL_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("LANGFUSE_OTEL_ENDPOINT", "")
	t.Setenv("LANGFUSE_HOST", "")
	t.Setenv("LANGFUSE_BASE_URL", "http://langfuse:3000")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-test")
	t.Setenv("LANGFUSE_SECRET_KEY", "sk-test")
	runtime, err := NewFromEnv()
	require.NoError(t, err)
	require.NotNil(t, runtime)
	require.Equal(t, "http://langfuse:3000", runtime.langfuseHost)
	require.NoError(t, runtime.Shutdown(context.Background()))
}

func TestNewFromEnvExportsToLangfuseEndpoint(t *testing.T) {
	received := make(chan struct{}, 1)
	var requestPath string
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("NEW_API_OTEL_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("LANGFUSE_OTEL_ENDPOINT", "")
	t.Setenv("LANGFUSE_HOST", server.URL)
	t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-test")
	t.Setenv("LANGFUSE_SECRET_KEY", "sk-test")
	runtime, err := NewFromEnv()
	require.NoError(t, err)

	info := &common.RelayInfo{RequestId: "req-export", OriginModelName: "gpt-test", RelayFormat: "openai"}
	ctx, span := runtime.StartLLMRequest(context.Background(), info, &dto.GeneralOpenAIRequest{Model: "gpt-test"})
	runtime.FinishLLM(ctx, span, nil, info)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, runtime.Shutdown(shutdownCtx))
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("OTLP exporter did not send a trace")
	}
	require.Equal(t, "/api/public/otel/v1/traces", requestPath)
	require.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("pk-test:sk-test")), authorization)
}

func TestResolveExporterConfigUsesLangfuseCredentials(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-test")
	t.Setenv("LANGFUSE_SECRET_KEY", "sk-test")
	endpoint, headers, err := resolveExporterConfig("http://langfuse:3000")
	require.NoError(t, err)
	require.Equal(t, "http://langfuse:3000/api/public/otel", endpoint)
	require.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("pk-test:sk-test")), headers["Authorization"])
	require.Equal(t, "4", headers["x-langfuse-ingestion-version"])
}

func TestStartLLMRequestCapturesInputAndUsage(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:         true,
		captureContent:  true,
		captureMaxBytes: 1024,
		tracerProvider:  provider,
		tracer:          provider.Tracer("test"),
		propagator:      propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	request := &dto.GeneralOpenAIRequest{
		Model:    "gpt-test",
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	}
	info := &common.RelayInfo{RequestId: "req-test", OriginModelName: "gpt-test", RelayFormat: "openai"}
	ctx, span := runtime.StartLLMRequest(context.Background(), info, request)
	runtime.RecordUsage(ctx, 3, 5, 8, 42, 0.01, true)
	runtime.FinishLLM(ctx, span, nil, info)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, int64(3), attributeValue(spans[0].Attributes, "gen_ai.usage.input_tokens").AsInt64())
	require.Equal(t, int64(5), attributeValue(spans[0].Attributes, "gen_ai.usage.output_tokens").AsInt64())
	require.Equal(t, int64(8), attributeValue(spans[0].Attributes, "gen_ai.usage.total_tokens").AsInt64())
	require.Equal(t, int64(42), attributeValue(spans[0].Attributes, "new_api.billing.quota").AsInt64())
	require.Contains(t, attributeValue(spans[0].Attributes, "gen_ai.input.messages").AsString(), "hello")
	require.Equal(t, "", attributeValue(spans[0].Attributes, "langfuse.observation.input").AsString())
	require.Equal(t, "", attributeValue(spans[0].Attributes, "langfuse.observation.output").AsString())
	require.Equal(t, "", attributeValue(spans[0].Attributes, "langfuse.observation.model.name").AsString())
	require.Equal(t, "", attributeValue(spans[0].Attributes, "langfuse.observation.usage_details").AsString())
	require.Equal(t, "", attributeValue(spans[0].Attributes, "new_api.billing.gateway_cost_usd").AsString())
	require.JSONEq(t, `{"total":0.000084}`, attributeValue(spans[0].Attributes, "langfuse.observation.cost_details").AsString())
}

func TestStartLLMRequestPropagatesSessionID(t *testing.T) {
	tests := []struct {
		name     string
		request  dto.Request
		expected string
	}{
		{
			name: "codex",
			request: &dto.OpenAIResponsesRequest{
				Model:          "gpt-test",
				ClientMetadata: json.RawMessage(`{"session_id":"codex-session-123"}`),
			},
			expected: "codex-session-123",
		},
		{
			name: "claude code",
			request: &dto.ClaudeRequest{
				Model:    "claude-test",
				Metadata: json.RawMessage(`{"user_id":{"device_id":"device-123","account_uuid":"account-123","session_id":"claude-code-session-123"}}`),
			},
			expected: "claude-code-session-123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
			runtime := &Runtime{
				enabled:         true,
				captureMaxBytes: 1024,
				tracerProvider:  provider,
				tracer:          provider.Tracer("test"),
				propagator:      propagationTraceContextForTest(),
			}
			t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

			parentCtx, parentSpan := runtime.tracer.Start(context.Background(), "http")
			llmCtx, llmSpan := runtime.StartLLMRequest(parentCtx, &common.RelayInfo{
				RequestId:       "req-session",
				OriginModelName: "test-model",
				RelayFormat:     "openai",
			}, tt.request)
			attemptCtx, attemptSpan := runtime.StartAttempt(llmCtx, nil, 7, 8, "test-channel")
			providerCtx, providerSpan := runtime.StartProviderRequest(attemptCtx, nil, nil)
			_, streamSpan := runtime.StartStream(providerCtx, nil)

			runtime.FinishSpan(streamSpan, nil)
			runtime.FinishSpan(providerSpan, nil)
			runtime.FinishSpan(attemptSpan, nil)
			runtime.FinishLLM(llmCtx, llmSpan, nil, nil)
			parentSpan.End()

			spans := exporter.GetSpans()
			require.Len(t, spans, 5)
			for _, span := range spans {
				require.Equal(t, tt.expected, attributeValue(span.Attributes, "session.id").AsString(), span.Name)
			}
		})
	}
}

func TestExtractLangfuseSessionIDRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		req  dto.Request
	}{
		{name: "nil request"},
		{name: "other client", req: &dto.GeneralOpenAIRequest{}},
		{name: "missing", req: &dto.OpenAIResponsesRequest{ClientMetadata: json.RawMessage(`{}`)}},
		{name: "malformed", req: &dto.OpenAIResponsesRequest{ClientMetadata: json.RawMessage(`{`)}},
		{name: "non string", req: &dto.OpenAIResponsesRequest{ClientMetadata: json.RawMessage(`{"session_id":42}`)}},
		{name: "empty", req: &dto.OpenAIResponsesRequest{ClientMetadata: json.RawMessage(`{"session_id":"   "}`)}},
		{name: "non ascii", req: &dto.OpenAIResponsesRequest{ClientMetadata: json.RawMessage(`{"session_id":"会话"}`)}},
		{name: "too long", req: &dto.OpenAIResponsesRequest{ClientMetadata: json.RawMessage(`{"session_id":"` + strings.Repeat("a", maxLangfuseSessionIDLen) + `"}`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Empty(t, extractLangfuseSessionID(tt.req))
		})
	}
	require.Equal(t, strings.Repeat("a", maxLangfuseSessionIDLen-1), extractLangfuseSessionID(&dto.OpenAIResponsesRequest{
		ClientMetadata: json.RawMessage(`{"session_id":"` + strings.Repeat("a", maxLangfuseSessionIDLen-1) + `"}`),
	}))
}

func TestExtractLangfuseSessionIDSupportsClaudeCode(t *testing.T) {
	require.Equal(t, "claude-code-session-123", extractLangfuseSessionID(&dto.ClaudeRequest{
		Metadata: json.RawMessage(`{"user_id":{"device_id":"device-123","account_uuid":"account-123","session_id":"claude-code-session-123"}}`),
	}))
	require.Equal(t, "claude-code-string-session-123", extractLangfuseSessionID(&dto.ClaudeRequest{
		Metadata: json.RawMessage(`{"user_id":"{\"device_id\":\"device-123\",\"account_uuid\":\"account-123\",\"session_id\":\"claude-code-string-session-123\"}"}`),
	}))
	require.Empty(t, extractLangfuseSessionID(&dto.ClaudeRequest{
		Metadata: json.RawMessage(`{"user_id":{"session_id":"会话"}}`),
	}))
}

func TestRecordStreamChunkCapturesOneFinalJSONEvent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:         true,
		captureContent:  true,
		captureMaxBytes: 1024,
		tracerProvider:  provider,
		tracer:          provider.Tracer("test"),
		propagator:      propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	info := &common.RelayInfo{RequestId: "req-stream-output", OriginModelName: "gpt-test", RelayFormat: "openai", IsStream: true}
	ctx, span := runtime.StartLLMRequest(context.Background(), info, nil)
	events := []string{
		`{"type":"response.created","response":{"id":"resp_1","model":"gpt-test"}}`,
		`{"type":"response.in_progress","response":{"id":"resp_1","model":"gpt-test"}}`,
		`{"type":"response.output_text.delta","delta":"hello"}`,
		`{"type":"response.output_text.done","text":"hello"}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}}`,
	}
	for _, event := range events {
		runtime.RecordStreamChunk(ctx, event)
	}
	runtime.RecordStreamChunk(ctx, `{"type":"response.output_text.delta","delta":"late"}`)
	runtime.FinishLLM(ctx, span, nil, info)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	output := attributeValue(spans[0].Attributes, "gen_ai.output.messages").AsString()
	require.JSONEq(t, events[len(events)-1], output)
	require.NotContains(t, output, "response.created")
	require.NotContains(t, output, "response.output_text.delta")
}

func TestRecordStreamChunkFallsBackToDeltaWhenCompletedOutputIsEmpty(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:         true,
		captureContent:  true,
		captureMaxBytes: 1024,
		tracerProvider:  provider,
		tracer:          provider.Tracer("test"),
		propagator:      propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	info := &common.RelayInfo{RequestId: "req-empty-completed-output", OriginModelName: "gpt-test", RelayFormat: "openai", IsStream: true}
	ctx, span := runtime.StartLLMRequest(context.Background(), info, nil)
	runtime.RecordStreamChunk(ctx, `{"type":"response.created","response":{"id":"resp_1"}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"response.output_text.delta","delta":"hello "}`)
	runtime.RecordStreamChunk(ctx, `{"type":"response.output_text.delta","delta":"from delta"}`)
	runtime.RecordStreamChunk(ctx, `{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[]}}`)
	runtime.FinishLLM(ctx, span, nil, info)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	output := attributeValue(spans[0].Attributes, "gen_ai.output.messages").AsString()
	var got map[string]any
	require.NoError(t, rootcommon.UnmarshalJsonStr(output, &got))
	require.Equal(t, "response.completed", got["type"])
	require.JSONEq(t, responsesAssistantOutputJSON("hello from delta"), mustJSON(t, got["response"].(map[string]any)["output"]))
}

func TestRecordStreamChunkKeepsPartialResponsesOutput(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:         true,
		captureContent:  true,
		captureMaxBytes: 1024,
		tracerProvider:  provider,
		tracer:          provider.Tracer("test"),
		propagator:      propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	info := &common.RelayInfo{RequestId: "req-incomplete-stream", OriginModelName: "gpt-test", RelayFormat: "openai", IsStream: true}
	ctx, span := runtime.StartLLMRequest(context.Background(), info, nil)
	runtime.RecordStreamChunk(ctx, `{"type":"response.output_text.delta","delta":"partial"}`)
	runtime.FinishLLM(ctx, span, nil, info)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.JSONEq(t,
		responsesAssistantOutputJSON("partial"),
		attributeValue(spans[0].Attributes, "gen_ai.output.messages").AsString(),
	)
}

func TestRecordStreamChunkAggregatesChatCompletionsDeltas(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:         true,
		captureContent:  true,
		captureMaxBytes: 1024,
		tracerProvider:  provider,
		tracer:          provider.Tracer("test"),
		propagator:      propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	info := &common.RelayInfo{RequestId: "req-chat-stream", OriginModelName: "gpt-test", RelayFormat: "openai", IsStream: true}
	ctx, span := runtime.StartLLMRequest(context.Background(), info, nil)
	runtime.RecordStreamChunk(ctx, `{"id":"c1","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}`)
	runtime.RecordStreamChunk(ctx, `{"id":"c1","choices":[{"index":0,"delta":{"content":"hello "}}]}`)
	runtime.RecordStreamChunk(ctx, `{"id":"c1","choices":[{"index":0,"delta":{"content":"world"}}]}`)
	runtime.RecordStreamChunk(ctx, `{"id":"c1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
	runtime.RecordStreamChunk(ctx, `{"id":"c1","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)
	runtime.FinishLLM(ctx, span, nil, info)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.JSONEq(t,
		`[{"role":"assistant","content":"hello world"}]`,
		attributeValue(spans[0].Attributes, "gen_ai.output.messages").AsString(),
	)
}

func TestRecordStreamChunkAggregatesLegacyCompletionsText(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:         true,
		captureContent:  true,
		captureMaxBytes: 1024,
		tracerProvider:  provider,
		tracer:          provider.Tracer("test"),
		propagator:      propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	info := &common.RelayInfo{RequestId: "req-completions-stream", OriginModelName: "gpt-test", RelayFormat: "openai", IsStream: true}
	ctx, span := runtime.StartLLMRequest(context.Background(), info, nil)
	runtime.RecordStreamChunk(ctx, `{"choices":[{"text":"once ","finish_reason":""}]}`)
	runtime.RecordStreamChunk(ctx, `{"choices":[{"text":"upon","finish_reason":"stop"}]}`)
	runtime.FinishLLM(ctx, span, nil, info)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.JSONEq(t,
		`[{"role":"assistant","content":"once upon"}]`,
		attributeValue(spans[0].Attributes, "gen_ai.output.messages").AsString(),
	)
}

func TestRecordStreamChunkRebuildsClaudeTextMessage(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:         true,
		captureContent:  true,
		captureMaxBytes: 1024,
		tracerProvider:  provider,
		tracer:          provider.Tracer("test"),
		propagator:      propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	info := &common.RelayInfo{RequestId: "req-claude-stream", OriginModelName: "claude-test", RelayFormat: "claude", IsStream: true, ChannelMeta: &common.ChannelMeta{ApiType: constant.APITypeAnthropic}}
	ctx, span := runtime.StartLLMRequest(context.Background(), info, nil)
	runtime.RecordStreamChunk(ctx, `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-test","content":[]}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello "}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_stop","index":0}`)
	runtime.RecordStreamChunk(ctx, `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"message_stop"}`)
	runtime.FinishLLM(ctx, span, nil, info)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.JSONEq(t,
		`[{"role":"assistant","content":"hello world"}]`,
		attributeValue(spans[0].Attributes, "gen_ai.output.messages").AsString(),
	)
}

// TestRecordStreamChunkRebuildsClaudeThinkingAndToolUse also covers the
// fallback dispatch pass: the relay info claims the default OpenAI API type,
// so the Claude aggregator is only reached because a frame no preferred
// aggregator recognized is still offered to the rest.
func TestRecordStreamChunkRebuildsClaudeThinkingAndToolUse(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:         true,
		captureContent:  true,
		captureMaxBytes: 1024,
		tracerProvider:  provider,
		tracer:          provider.Tracer("test"),
		propagator:      propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	info := &common.RelayInfo{RequestId: "req-claude-tool", OriginModelName: "claude-test", RelayFormat: "claude", IsStream: true}
	ctx, span := runtime.StartLLMRequest(context.Background(), info, nil)
	runtime.RecordStreamChunk(ctx, `{"type":"message_start","message":{"id":"msg_2","type":"message","role":"assistant","model":"claude-test","content":[]}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"let me think"}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig"}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_stop","index":0}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"SF\"}"}}`)
	runtime.RecordStreamChunk(ctx, `{"type":"content_block_stop","index":1}`)
	runtime.RecordStreamChunk(ctx, `{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`)
	runtime.FinishLLM(ctx, span, nil, info)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.JSONEq(t,
		`[{"role":"assistant","content":"","reasoning_content":"let me think\n","tool_calls":[{"id":"toolu_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"SF\"}"}}]}]`,
		attributeValue(spans[0].Attributes, "gen_ai.output.messages").AsString(),
	)
}

// responsesAssistantOutputJSON is the marshaled form of the assistant message
// the shared relayconvert accumulator rebuilds. dto.ResponsesOutput has no
// omitempty on its scalar fields, so the rebuilt item carries empty id/status/
// quality/size keys that the upstream terminal frame would not have had.
func responsesAssistantOutputJSON(text string) string {
	return fmt.Sprintf(`[{"type":"message","id":"","status":"","role":"assistant","quality":"","size":"","content":[{"type":"output_text","text":%q,"annotations":null}]}]`, text)
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := rootcommon.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

func TestNestedAttemptUsageAndTTFTStayOnGenerationSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:         true,
		captureMaxBytes: 1024,
		tracerProvider:  provider,
		tracer:          provider.Tracer("test"),
		propagator:      propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	start := time.Now().Add(-500 * time.Millisecond).UTC()
	firstResponse := start.Add(175 * time.Millisecond)
	info := &common.RelayInfo{
		RequestId:         "req-nested",
		OriginModelName:   "gpt-test",
		RelayFormat:       "openai",
		IsStream:          true,
		StartTime:         start,
		FirstResponseTime: firstResponse,
	}
	llmCtx, llmSpan := runtime.StartLLMRequest(context.Background(), info, nil)
	attemptCtx, attemptSpan := runtime.StartAttempt(llmCtx, info, 7, 8, "test-channel")
	inputCost := 0.001
	outputCost := 0.002
	runtime.RecordUsageWithCosts(attemptCtx, 10, 20, 30, 15000, 0, false, &BillingCostDetails{
		InputUSD:  &inputCost,
		OutputUSD: &outputCost,
		TotalUSD:  0.003,
	})
	runtime.FinishSpan(attemptSpan, nil)
	runtime.FinishLLM(llmCtx, llmSpan, nil, info)

	spans := exporter.GetSpans()
	require.Len(t, spans, 2)
	var generation, attempt tracetest.SpanStub
	for _, span := range spans {
		switch span.Name {
		case llmRequestSpanName:
			generation = span
		case relayAttemptSpanName:
			attempt = span
		}
	}
	require.Equal(t, llmRequestSpanName, generation.Name)
	require.Equal(t, relayAttemptSpanName, attempt.Name)
	require.WithinDuration(t, start, generation.StartTime, time.Nanosecond)
	require.Equal(t, int64(10), attributeValue(generation.Attributes, "gen_ai.usage.input_tokens").AsInt64())
	require.JSONEq(t, `{"input":0.001,"output":0.002,"total":0.003}`, attributeValue(generation.Attributes, "langfuse.observation.cost_details").AsString())
	require.Equal(t, "gateway_charge_usd", attributeValue(generation.Attributes, "new_api.billing.cost_semantics").AsString())
	require.Equal(t, "", attributeValue(generation.Attributes, "new_api.billing.gateway_cost_usd").AsString())
	require.Equal(t, "", attributeValue(attempt.Attributes, "langfuse.observation.cost_details").AsString())
	require.Equal(t, firstResponse.Format(time.RFC3339Nano), attributeValue(generation.Attributes, "langfuse.observation.completion_start_time").AsString())
	require.Equal(t, int64(175), attributeValue(generation.Attributes, "new_api.ttft_ms").AsInt64())
}

func TestStartStreamDoesNotDuplicateStreamMarker(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:        true,
		tracerProvider: provider,
		tracer:         provider.Tracer("test"),
		propagator:     propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	_, span := runtime.StartStream(context.Background(), nil)
	runtime.FinishSpan(span, nil)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, "", attributeValue(spans[0].Attributes, "new_api.stream.is_stream").AsString())
}

func TestFinishLLMDoesNotSetTTFTWithoutResponse(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:        true,
		tracerProvider: provider,
		tracer:         provider.Tracer("test"),
		propagator:     propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	start := time.Now().Add(-100 * time.Millisecond)
	info := &common.RelayInfo{
		RequestId:         "req-no-response",
		OriginModelName:   "gpt-test",
		RelayFormat:       "openai",
		IsStream:          true,
		StartTime:         start,
		FirstResponseTime: start.Add(-time.Second),
	}
	ctx, span := runtime.StartLLMRequest(context.Background(), info, nil)
	runtime.FinishLLM(ctx, span, nil, info)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, "", attributeValue(spans[0].Attributes, "langfuse.observation.completion_start_time").AsString())
}

func propagationTraceContextForTest() propagation.TextMapPropagator {
	return propagation.TraceContext{}
}

func TestShouldTracePath(t *testing.T) {
	require.True(t, shouldTracePath("/v1/chat/completions"))
	require.True(t, shouldTracePath("/v1beta/models"))
	require.True(t, shouldTracePath("/pg/chat/completions"))
	require.False(t, shouldTracePath("/health"))
	require.False(t, shouldTracePath("/api/logs"))
}

func TestMiddlewareRecordsUsernameAsUserID(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	runtime := &Runtime{
		enabled:        true,
		tracerProvider: provider,
		tracer:         provider.Tracer("test"),
		propagator:     propagationTraceContextForTest(),
	}
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	engine := gin.New()
	engine.Use(runtime.Middleware())
	engine.GET("/v1/models", func(c *gin.Context) {
		c.Set(string(constant.ContextKeyUserName), "alice")
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, "alice", attributeValue(spans[0].Attributes, "user.id").AsString())
}

func TestParseHeaders(t *testing.T) {
	result := parseHeaders("Authorization=Basic abc==,x-test=Bearer%20token")
	require.Equal(t, "Basic abc==", result["Authorization"])
	require.Equal(t, "Bearer token", result["x-test"])
}

func TestNormalizeTraceEndpoint(t *testing.T) {
	require.Equal(t, "http://langfuse:3000/api/public/otel/v1/traces", normalizeTraceEndpoint("http://langfuse:3000/api/public/otel"))
	require.Equal(t, "http://collector:4318/v1/traces", normalizeTraceEndpoint("http://collector:4318/v1/traces"))
}

func attributeValue(attrs []attribute.KeyValue, key string) attribute.Value {
	for _, attr := range attrs {
		if string(attr.Key) == key {
			return attr.Value
		}
	}
	return attribute.Value{}
}
