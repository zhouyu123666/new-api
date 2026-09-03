package observability

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName             = "new-api"
	defaultCaptureMaxBytes  = 4 << 20
	maxLangfuseSessionIDLen = 200
	traceContextKey         = "new_api_otel_trace"
	llmRequestSpanName      = "newapi.llm.request"
	relayAttemptSpanName    = "newapi.relay.attempt"
	providerRequestSpanName = "newapi.provider.request"
	streamSpanName          = "newapi.stream.consume"
)

type runtimeContextKey struct{}

type Runtime struct {
	enabled         bool
	captureContent  bool
	captureMaxBytes int64
	langfuseHost    string
	langfuseProject string
	tracerProvider  *sdktrace.TracerProvider
	tracer          trace.Tracer
	propagator      propagation.TextMapPropagator
	shutdownOnce    sync.Once
	shutdownErr     error
}

type traceState struct {
	mu          sync.Mutex
	span        trace.Span
	sessionID   string
	input       strings.Builder
	output      strings.Builder
	inputTrunc  bool
	outputTrunc bool
	maxBytes    int64

	// apiType is the upstream API type of the relayed request, or
	// apiTypeUnknown. It decides which stream aggregators are offered the
	// first frame first.
	apiType int

	// streamAgg rebuilds the answer of a streamed response. It is chosen from
	// the first recognized frame and owns every frame after that.
	streamAgg streamAggregator
}

// BillingCostDetails describes the gateway-side USD amount associated with a
// completed request. Input/output are optional because fixed-price and
// expression-based billing do not always have a trustworthy split, while the
// total is always derived from the authoritative settled quota.
type BillingCostDetails struct {
	InputUSD  *float64
	OutputUSD *float64
	TotalUSD  float64
}

func NewFromEnv() (*Runtime, error) {
	enabled := envBool("NEW_API_OTEL_ENABLED", false)
	runtime := &Runtime{
		enabled:         enabled,
		captureContent:  envString("NEW_API_OTEL_CAPTURE_CONTENT", "") == "full",
		captureMaxBytes: envInt64("NEW_API_OTEL_CAPTURE_MAX_BYTES", defaultCaptureMaxBytes),
		langfuseHost:    strings.TrimRight(firstNonEmptyEnv("LANGFUSE_BASE_URL", "LANGFUSE_HOST"), "/"),
		langfuseProject: os.Getenv("LANGFUSE_PROJECT_ID"),
		propagator:      propagation.TraceContext{},
	}
	if runtime.captureMaxBytes < 0 {
		runtime.captureMaxBytes = defaultCaptureMaxBytes
	}
	if !enabled {
		return runtime, nil
	}

	endpoint, headers, err := resolveExporterConfig(runtime.langfuseHost)
	if err != nil {
		return nil, err
	}
	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpointURL(normalizeTraceEndpoint(endpoint)),
		otlptracehttp.WithHeaders(headers),
		otlptracehttp.WithTimeout(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize OTLP HTTP exporter: %w", err)
	}
	resource, err := sdkresource.New(context.Background(),
		sdkresource.WithAttributes(
			attribute.String("service.name", envString("OTEL_SERVICE_NAME", serviceName)),
			attribute.String("deployment.environment", envString("OTEL_ENVIRONMENT_NAME", "production")),
		),
		sdkresource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize OTel resource: %w", err)
	}
	runtime.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
	)
	runtime.tracer = runtime.tracerProvider.Tracer(envString("OTEL_TRACER_NAME", serviceName))
	return runtime, nil
}

func (r *Runtime) Enabled() bool {
	return r != nil && r.enabled && r.tracer != nil
}

func (r *Runtime) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !r.Enabled() || !shouldTracePath(c.Request.URL.Path) {
			c.Next()
			return
		}
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		name := c.Request.Method + " " + route
		ctx := r.propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := r.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindServer))
		span.SetAttributes(
			attribute.String("http.request.method", c.Request.Method),
			attribute.String("http.route", route),
			attribute.String("url.path", c.Request.URL.Path),
			attribute.String("new_api.request_id", c.GetString(common.RequestIdKey)),
		)
		c.Request = c.Request.WithContext(ctx)
		ctx = context.WithValue(ctx, runtimeContextKey{}, r)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		span.SetAttributes(attribute.Int("http.response.status_code", c.Writer.Status()))
		if username := common.GetContextKeyString(c, constant.ContextKeyUserName); username != "" {
			span.SetAttributes(attribute.String("user.id", username))
		}
		if c.Writer.Status() >= http.StatusBadRequest {
			span.SetStatus(codes.Error, http.StatusText(c.Writer.Status()))
		}
		span.End()
	}
}

func FromContext(ctx context.Context) *Runtime {
	if ctx == nil {
		return nil
	}
	runtime, _ := ctx.Value(runtimeContextKey{}).(*Runtime)
	return runtime
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	if !r.Enabled() {
		return nil
	}
	r.shutdownOnce.Do(func() {
		r.shutdownErr = r.tracerProvider.Shutdown(ctx)
	})
	return r.shutdownErr
}

func (r *Runtime) StartLLMRequest(ctx context.Context, info *relaycommon.RelayInfo, request dto.Request) (context.Context, trace.Span) {
	if !r.Enabled() {
		return ctx, trace.SpanFromContext(ctx)
	}
	// The channel is not always resolved yet, and its API type is only a hint
	// for picking a stream aggregator.
	apiType := apiTypeUnknown
	if info != nil && info.HasChannelMeta() {
		apiType = info.ApiType
	}
	sessionID := extractLangfuseSessionID(request)
	state := &traceState{maxBytes: r.captureMaxBytes, apiType: apiType, sessionID: sessionID}
	setSessionID(trace.SpanFromContext(ctx), sessionID)
	spanAttrs := []attribute.KeyValue{
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.request.model", info.OriginModelName),
		attribute.String("langfuse.observation.type", "generation"),
		attribute.String("new_api.request_id", info.RequestId),
		attribute.String("new_api.relay_format", string(info.RelayFormat)),
		attribute.Bool("new_api.request.stream", info.IsStream),
	}
	startOptions := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindInternal)}
	// RelayInfo.StartTime is the gateway request boundary used by the existing
	// FRT/performance metrics. Align the generation span start with it so
	// Langfuse's completion_start_time - start_time is the same measurement.
	if info != nil && !info.StartTime.IsZero() {
		startOptions = append(startOptions, trace.WithTimestamp(info.StartTime))
	}
	ctx, span := r.tracer.Start(ctx, llmRequestSpanName, startOptions...)
	span.SetAttributes(spanAttrs...)
	setSessionID(span, sessionID)
	state.span = span
	ctx = context.WithValue(ctx, traceContextKey, state)
	return ctx, span
}

// RecordInput stores a filtered copy of the final upstream request body for
// Langfuse. It must be called after relay conversion and all request policies
// have been applied, immediately before the upstream request is sent.
func (r *Runtime) RecordInput(ctx context.Context, body []byte, format types.RelayFormat) {
	if !r.Enabled() || !r.captureContent || len(body) == 0 {
		return
	}
	state := stateFromContext(ctx)
	if state == nil {
		return
	}
	filtered, ok := buildLangfuseInput(body, format)
	if !ok {
		return
	}
	state.setInput(generationSpanFromContext(ctx), filtered)
}

func (r *Runtime) StartAttempt(ctx context.Context, info *relaycommon.RelayInfo, channelID int, channelType int, channelName string) (context.Context, trace.Span) {
	if !r.Enabled() {
		return ctx, trace.SpanFromContext(ctx)
	}
	ctx, span := r.tracer.Start(ctx, relayAttemptSpanName, trace.WithSpanKind(trace.SpanKindInternal))
	span.SetAttributes(
		attribute.Int("new_api.channel_id", channelID),
		attribute.Int("new_api.channel_type", channelType),
		attribute.String("new_api.channel_name", channelName),
	)
	setSessionID(span, sessionIDFromContext(ctx))
	// RelayInfo is intentionally not read here. The relay may replace or clear
	// it while selecting a channel; observability must never be able to panic
	// the request path. The request/model attributes are recorded on the parent
	// LLM span and the final upstream model is added when the attempt finishes.
	return ctx, span
}

func (r *Runtime) StartProviderRequest(ctx context.Context, info *relaycommon.RelayInfo, req *http.Request) (context.Context, trace.Span) {
	if !r.Enabled() {
		return ctx, trace.SpanFromContext(ctx)
	}
	ctx, span := r.tracer.Start(ctx, providerRequestSpanName, trace.WithSpanKind(trace.SpanKindClient))
	if req != nil && req.URL != nil {
		span.SetAttributes(
			attribute.String("server.address", req.URL.Hostname()),
			attribute.String("url.path", req.URL.Path),
			attribute.String("http.request.method", req.Method),
		)
	}
	setSessionID(span, sessionIDFromContext(ctx))
	return ctx, span
}

func (r *Runtime) StartStream(ctx context.Context, info *relaycommon.RelayInfo) (context.Context, trace.Span) {
	if !r.Enabled() {
		return ctx, trace.SpanFromContext(ctx)
	}
	ctx, span := r.tracer.Start(ctx, streamSpanName, trace.WithSpanKind(trace.SpanKindInternal))
	setSessionID(span, sessionIDFromContext(ctx))
	return ctx, span
}

func (r *Runtime) Inject(ctx context.Context, req *http.Request) {
	if !r.Enabled() || req == nil {
		return
	}
	r.propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
}

func (r *Runtime) RecordStreamChunk(ctx context.Context, data string) {
	if !r.Enabled() || !r.captureContent || data == "" {
		return
	}
	state := stateFromContext(ctx)
	if state == nil {
		return
	}
	state.recordStreamEvent(data)
}

func (r *Runtime) WrapResponseBody(ctx context.Context, body io.ReadCloser) io.ReadCloser {
	if !r.Enabled() || !r.captureContent || body == nil {
		return body
	}
	return &captureReadCloser{ReadCloser: body, state: stateFromContext(ctx)}
}

func (r *Runtime) RecordUsage(ctx context.Context, promptTokens, completionTokens, totalTokens, quota int, modelPrice float64, usePrice bool) {
	r.recordUsage(ctx, promptTokens, completionTokens, totalTokens, quota, modelPrice, usePrice, nil)
}

// RecordUsageWithCosts records usage and an optional authoritative gateway
// cost split. The cost values are serialized as JSON because Langfuse's OTLP
// ingestion treats langfuse.observation.cost_details as a JSON object string.
func (r *Runtime) RecordUsageWithCosts(ctx context.Context, promptTokens, completionTokens, totalTokens, quota int, modelPrice float64, usePrice bool, costs *BillingCostDetails) {
	r.recordUsage(ctx, promptTokens, completionTokens, totalTokens, quota, modelPrice, usePrice, costs)
}

func (r *Runtime) recordUsage(ctx context.Context, promptTokens, completionTokens, totalTokens, quota int, modelPrice float64, usePrice bool, costs *BillingCostDetails) {
	if !r.Enabled() {
		return
	}
	span := generationSpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	if costs != nil && !validBillingCostDetails(costs) {
		costs = nil
	}
	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", promptTokens),
		attribute.Int("gen_ai.usage.output_tokens", completionTokens),
		attribute.Int("gen_ai.usage.total_tokens", totalTokens),
		attribute.Int("new_api.billing.quota", quota),
		attribute.Float64("new_api.billing.model_price", modelPrice),
		attribute.Bool("new_api.billing.use_price", usePrice),
	)
	if costs != nil {
		span.SetAttributes(
			attribute.String("new_api.billing.cost_semantics", "gateway_charge_usd"),
		)
		costDetails := map[string]float64{"total": costs.TotalUSD}
		if costs.InputUSD != nil {
			costDetails["input"] = *costs.InputUSD
		}
		if costs.OutputUSD != nil {
			costDetails["output"] = *costs.OutputUSD
		}
		if cost, err := common.Marshal(costDetails); err == nil {
			span.SetAttributes(attribute.String("langfuse.observation.cost_details", string(cost)))
		}
	} else if quota > 0 && common.QuotaPerUnit > 0 && !math.IsNaN(common.QuotaPerUnit) && !math.IsInf(common.QuotaPerUnit, 0) {
		span.SetAttributes(attribute.String("new_api.billing.cost_semantics", "gateway_charge_usd"))
		// The settled quota is the only value that includes group, request,
		// tool, cache, and other multipliers consistently across billing modes.
		// Keep a total-only fallback for callers that do not have a reliable
		// input/output decomposition.
		if cost, err := common.Marshal(map[string]float64{"total": float64(quota) / common.QuotaPerUnit}); err == nil {
			span.SetAttributes(attribute.String("langfuse.observation.cost_details", string(cost)))
		}
	}
}

func validBillingCostDetails(costs *BillingCostDetails) bool {
	if costs == nil || costs.TotalUSD < 0 || math.IsNaN(costs.TotalUSD) || math.IsInf(costs.TotalUSD, 0) {
		return false
	}
	for _, value := range []*float64{costs.InputUSD, costs.OutputUSD} {
		if value != nil && (*value < 0 || math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return false
		}
	}
	return true
}

func generationSpanFromContext(ctx context.Context) trace.Span {
	if state := stateFromContext(ctx); state != nil && state.span != nil {
		return state.span
	}
	return trace.SpanFromContext(ctx)
}

func (r *Runtime) EnrichLogOther(ctx context.Context, other map[string]interface{}) {
	if !r.Enabled() || other == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return
	}
	traceID := span.SpanContext().TraceID().String()
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	if !ok || adminInfo == nil {
		adminInfo = map[string]interface{}{}
		other["admin_info"] = adminInfo
	}
	adminInfo["otel_trace_id"] = traceID
	if r.langfuseHost != "" && r.langfuseProject != "" {
		adminInfo["langfuse_trace_url"] = fmt.Sprintf("%s/project/%s/traces/%s", r.langfuseHost, r.langfuseProject, traceID)
	}
}

func (r *Runtime) FinishLLM(ctx context.Context, span trace.Span, err error, info *relaycommon.RelayInfo) {
	if !r.Enabled() || span == nil {
		return
	}
	if info != nil {
		span.SetAttributes(
			attribute.Int("new_api.stream.chunk_count", info.ReceivedResponseCount),
			attribute.Int("new_api.retry.count", info.RetryIndex),
		)
		if info.StreamStatus != nil {
			span.SetAttributes(attribute.String("new_api.stream.end_reason", string(info.StreamStatus.EndReason)))
		}
		if info.IsStream && info.HasSendResponse() {
			// Langfuse derives Time To First Token from this absolute timestamp,
			// not from a duration attribute. RelayInfo uses the same request start
			// boundary as the existing performance metrics.
			span.SetAttributes(
				attribute.String("langfuse.observation.completion_start_time", info.FirstResponseTime.UTC().Format(time.RFC3339Nano)),
				attribute.Int64("new_api.ttft_ms", info.FirstResponseTime.Sub(info.StartTime).Milliseconds()),
			)
		}
	}
	if state := stateFromContext(ctx); state != nil {
		state.mu.Lock()
		state.finalizeStreamOutput(span)
		state.finalizeCapturedOutput(span)
		if state.input.Len() > 0 {
			span.SetAttributes(
				attribute.String("gen_ai.input.messages", state.input.String()),
			)
		}
		if state.output.Len() > 0 {
			span.SetAttributes(
				attribute.String("gen_ai.output.messages", state.output.String()),
			)
		}
		state.mu.Unlock()
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

func (r *Runtime) FinishSpan(span trace.Span, err error) {
	if !r.Enabled() || span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

func TraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().TraceID().String()
}

func stateFromContext(ctx context.Context) *traceState {
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(traceContextKey).(*traceState)
	return state
}

func sessionIDFromContext(ctx context.Context) string {
	if state := stateFromContext(ctx); state != nil {
		return state.sessionID
	}
	return ""
}

func setSessionID(span trace.Span, sessionID string) {
	if span == nil || sessionID == "" {
		return
	}
	span.SetAttributes(attribute.String("session.id", sessionID))
}

func extractLangfuseSessionID(request dto.Request) string {
	var sessionID string
	switch request := request.(type) {
	case *dto.OpenAIResponsesRequest:
		// Codex includes its conversation identifier in Responses
		// client_metadata.session_id.
		if request == nil || len(request.ClientMetadata) == 0 {
			return ""
		}
		var metadata struct {
			SessionID string `json:"session_id"`
		}
		if err := common.Unmarshal([]byte(request.ClientMetadata), &metadata); err != nil {
			return ""
		}
		sessionID = metadata.SessionID
	case *dto.ClaudeRequest:
		// Claude Code nests its session identifier in metadata.user_id.session_id.
		// Depending on the client version, user_id is either an object or a
		// JSON-encoded string containing that object.
		if request == nil || len(request.Metadata) == 0 {
			return ""
		}
		var metadata struct {
			UserID json.RawMessage `json:"user_id"`
		}
		if err := common.Unmarshal([]byte(request.Metadata), &metadata); err != nil {
			return ""
		}
		var userID struct {
			SessionID string `json:"session_id"`
		}
		if err := common.Unmarshal(metadata.UserID, &userID); err == nil {
			sessionID = userID.SessionID
			break
		}
		var encodedUserID string
		if err := common.Unmarshal(metadata.UserID, &encodedUserID); err != nil {
			return ""
		}
		if err := common.Unmarshal([]byte(encodedUserID), &userID); err != nil {
			return ""
		}
		sessionID = userID.SessionID
	default:
		return ""
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || len(sessionID) >= maxLangfuseSessionIDLen {
		return ""
	}
	for i := 0; i < len(sessionID); i++ {
		if sessionID[i] > 0x7f {
			return ""
		}
	}
	return sessionID
}

type captureReadCloser struct {
	io.ReadCloser
	state *traceState
}

func (r *captureReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 && r.state != nil {
		r.state.addOutput(r.state.span, p[:n])
	}
	return n, err
}

func (s *traceState) setInput(span trace.Span, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.input.Reset()
	s.inputTrunc = false
	s.setJSONLocked(&s.input, span, data, &s.inputTrunc, true)
}

func (s *traceState) setJSONLocked(builder *strings.Builder, span trace.Span, data []byte, truncated *bool, input bool) {
	if s.maxBytes > 0 && int64(len(data)) > s.maxBytes {
		// A byte slice cut from JSON can no longer be parsed by Langfuse. Keep
		// an empty valid JSON object/array and mark the capture as truncated.
		if input {
			builder.WriteString("{}")
		} else {
			builder.WriteString("[]")
		}
		*truncated = true
		if span != nil {
			key := "new_api.capture.output_truncated"
			if input {
				key = "new_api.capture.input_truncated"
			}
			span.SetAttributes(attribute.Bool(key, true))
		}
		return
	}
	builder.Write(data)
}

func (s *traceState) addOutput(span trace.Span, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeOutputLocked(span, data)
}

func (s *traceState) writeOutputLocked(span trace.Span, data []byte) {
	if s.outputTrunc {
		return
	}
	if s.maxBytes > 0 && int64(s.output.Len()+len(data)) > s.maxBytes {
		remaining := s.maxBytes - int64(s.output.Len())
		if remaining > 0 {
			s.output.Write(data[:remaining])
		}
		s.outputTrunc = true
		span.SetAttributes(attribute.Bool("new_api.capture.output_truncated", true))
		return
	}
	s.output.Write(data)
}

// recordStreamEvent consumes stream frames without exporting them
// individually. The first recognized frame picks the aggregator for the
// upstream protocol, and FinishLLM turns what that aggregator collected into a
// single output value.
func (s *traceState) recordStreamEvent(data string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.streamAgg != nil {
		s.streamAgg.accept(data)
		return
	}
	// The upstream API type decides which protocols are tried first. A frame
	// none of them recognizes is still offered to the rest, so an upstream
	// speaking a protocol its API type does not advertise keeps its output.
	for _, preferred := range []bool{true, false} {
		for _, entry := range streamAggregators {
			if entry.preferredFor(s.apiType) != preferred {
				continue
			}
			aggregator := entry.newAggregator(s.maxBytes)
			if aggregator.accept(data) {
				s.streamAgg = aggregator
				return
			}
		}
	}
}

func (s *traceState) finalizeStreamOutput(span trace.Span) {
	if s.streamAgg == nil {
		return
	}
	out := s.streamAgg.output()
	s.output.Reset()
	if len(out.value) == 0 {
		return
	}
	if !out.selfCapped {
		s.writeOutputLocked(span, out.value)
		return
	}
	if out.truncated {
		s.outputTrunc = true
		span.SetAttributes(attribute.Bool("new_api.capture.output_truncated", true))
	}
	s.output.Write(out.value)
}

func (s *traceState) finalizeCapturedOutput(span trace.Span) {
	if s.output.Len() == 0 {
		return
	}
	normalized, ok := normalizeLangfuseOutput([]byte(s.output.String()))
	if !ok {
		if s.outputTrunc {
			s.output.Reset()
			s.output.WriteString("[]")
		}
		return
	}
	s.output.Reset()
	s.outputTrunc = false
	s.setJSONLocked(&s.output, span, normalized, &s.outputTrunc, false)
}

func resolveExporterConfig(host string) (string, map[string]string, error) {
	endpoint := strings.TrimRight(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), "/")
	if endpoint == "" {
		endpoint = strings.TrimRight(os.Getenv("LANGFUSE_OTEL_ENDPOINT"), "/")
	}
	if endpoint == "" && host != "" {
		endpoint = host + "/api/public/otel"
	}
	if endpoint == "" {
		return "", nil, errors.New("NEW_API_OTEL_ENABLED requires OTEL_EXPORTER_OTLP_ENDPOINT or LANGFUSE_BASE_URL/LANGFUSE_HOST")
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return "", nil, fmt.Errorf("OTel endpoint must include http:// or https://: %s", endpoint)
	}
	headers := parseHeaders(os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"))
	publicKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	secretKey := os.Getenv("LANGFUSE_SECRET_KEY")
	if publicKey != "" || secretKey != "" {
		if publicKey == "" || secretKey == "" {
			return "", nil, errors.New("LANGFUSE_PUBLIC_KEY and LANGFUSE_SECRET_KEY must be set together")
		}
		auth := base64.StdEncoding.EncodeToString([]byte(publicKey + ":" + secretKey))
		headers["Authorization"] = "Basic " + auth
		headers["x-langfuse-ingestion-version"] = "4"
	}
	return endpoint, headers, nil
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func normalizeTraceEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/v1/traces") {
		return endpoint
	}
	return endpoint + "/v1/traces"
}

func parseHeaders(raw string) map[string]string {
	result := map[string]string{}
	for _, item := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(item, "=")
		if ok && strings.TrimSpace(key) != "" {
			decoded, err := url.PathUnescape(strings.TrimSpace(value))
			if err != nil {
				decoded = strings.TrimSpace(value)
			}
			result[strings.TrimSpace(key)] = decoded
		}
	}
	return result
}

func shouldTracePath(path string) bool {
	return strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/v1beta/") || strings.HasPrefix(path, "/pg/")
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
