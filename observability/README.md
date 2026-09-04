# OpenTelemetry and Langfuse

new-api exports relay traces directly to a Langfuse OTLP/HTTP endpoint. The
feature is disabled unless `NEW_API_OTEL_ENABLED=true`.

## Configuration

```text
NEW_API_OTEL_ENABLED=true
LANGFUSE_BASE_URL=http://langfuse:3000
LANGFUSE_PROJECT_ID=your-project-id
LANGFUSE_PUBLIC_KEY=pk-...
LANGFUSE_SECRET_KEY=sk-...
NEW_API_OTEL_CAPTURE_CONTENT=full
NEW_API_OTEL_CAPTURE_MAX_BYTES=4194304
```

The exporter posts to `${LANGFUSE_BASE_URL}/api/public/otel/v1/traces`. An
explicit `OTEL_EXPORTER_OTLP_ENDPOINT` takes precedence over the Langfuse base
URL; `LANGFUSE_HOST` is accepted as a backwards-compatible alias. The endpoint
may point to a collector or another OTLP/HTTP backend. `OTEL_EXPORTER_OTLP_HEADERS`
can add custom headers, while Langfuse credentials add the Basic Auth and
ingestion-version headers automatically.

`NEW_API_OTEL_CAPTURE_CONTENT=full` records serialized input and one valid output
JSON value on the trace, subject to `NEW_API_OTEL_CAPTURE_MAX_BYTES` per
direction. Stream frames are aggregated in memory and never exported
individually.

Each upstream protocol has its own aggregator behind the `streamAggregator`
interface in `stream_aggregator.go`, one per file. The first frame that an
aggregator recognizes decides which one owns the stream, so supporting another
provider means adding a `stream_aggregator_<protocol>.go` and one entry in the
`streamAggregators` registry, ahead of the loose OpenAI chat shape.

A registry entry lists the upstream API types known to speak its protocol, and
those aggregators are offered the first frame first. The list is only an
ordering hint: a frame that none of the preferred aggregators recognizes is
still offered to the rest, so a custom or aggregating channel proxying a
protocol its API type does not advertise keeps its output.

An OpenAI Responses stream is aggregated by the same
`relayconvert.ResponsesBufferedAccumulator` the buffered Responses-to-Chat
converter uses, so text, reasoning and tool calls are all collected. The trace
then receives the terminal frame verbatim. Only when that frame carries an empty
`output` array is the array replaced by the aggregated output, leaving every
other terminal field untouched; the rebuilt items come from `dto.ResponsesOutput`
and therefore carry its empty scalar keys. A stream that never reaches a
terminal event exports the aggregated output array on its own, because a partial
answer is worth more on the trace than none.

A Claude Messages stream is replayed through `relayconvert.StreamResponseClaude2OpenAI`
and `relayconvert.FormatClaudeResponseInfo`, the same converters the relay path
uses to serve a Claude upstream in OpenAI format, so the trace and the client
read the stream through one implementation. The per-frame deltas are folded back
into a single assistant message — text, reasoning and tool calls, with the
argument fragments of each tool call joined under the content block index that
carries them — and exported in the same
`[{"role":"assistant","content":"..."}]` shape as every other chat upstream.

Every other stream is reduced to its assistant text
(`choices[].delta.content`, or `choices[].text` for legacy completions) and
exported as a single `[{"role":"assistant","content":"..."}]` value.

Content capture is disabled when the setting is absent. Do not enable it for
requests that may contain credentials, personal data, or other sensitive material
without an appropriate access-control and retention policy.

For completed text/audio/realtime billing paths, `langfuse.observation.cost_details`
is the gateway charge in USD (`settled quota / QuotaPerUnit`), not an upstream
provider invoice. Ratio-based text requests expose an input/output split when it
is mathematically stable; fixed-price and tiered-expression requests expose a
total only because a trustworthy split is not available. The same total is also
`new_api.billing.cost_semantics=gateway_charge_usd` identifies this as a
gateway charge. The internal quota and consume log remain authoritative for
accounting.

`langfuse.observation.completion_start_time` is emitted for streamed requests
that have a recorded first response. It follows the existing `RelayInfo`
first-response boundary used by performance metrics; some providers may report
the first protocol response frame rather than a semantic generated token.

The trace is a side channel. Export errors are handled by the OTel batch exporter
and do not change the model response or billing result. Existing consume logs and
performance metrics remain the source of truth for billing and aggregate dashboards.
