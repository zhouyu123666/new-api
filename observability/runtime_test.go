package observability

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	require.Equal(t, int64(42), attributeValue(spans[0].Attributes, "new_api.billing.quota").AsInt64())
	require.Contains(t, attributeValue(spans[0].Attributes, "gen_ai.input.messages").AsString(), "hello")
	require.JSONEq(t, `{"total":0.000084}`, attributeValue(spans[0].Attributes, "langfuse.observation.cost_details").AsString())
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
	require.InDelta(t, 0.003, attributeValue(generation.Attributes, "new_api.billing.gateway_cost_usd").AsFloat64(), 1e-12)
	require.Equal(t, "", attributeValue(attempt.Attributes, "langfuse.observation.cost_details").AsString())
	require.Equal(t, firstResponse.Format(time.RFC3339Nano), attributeValue(generation.Attributes, "langfuse.observation.completion_start_time").AsString())
	require.Equal(t, int64(175), attributeValue(generation.Attributes, "new_api.ttft_ms").AsInt64())
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
