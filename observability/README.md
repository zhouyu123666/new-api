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

`NEW_API_OTEL_CAPTURE_CONTENT=full` records serialized input and streamed output
on the trace, subject to `NEW_API_OTEL_CAPTURE_MAX_BYTES` per direction. Content
capture is disabled when the setting is absent. Do not enable it for requests that
may contain credentials, personal data, or other sensitive material without an
appropriate access-control and retention policy.

For completed text/audio/realtime billing paths, `langfuse.observation.cost_details`
is the gateway charge in USD (`settled quota / QuotaPerUnit`), not an upstream
provider invoice. Ratio-based text requests expose an input/output split when it
is mathematically stable; fixed-price and tiered-expression requests expose a
total only because a trustworthy split is not available. The same total is also
available as `new_api.billing.gateway_cost_usd`, with
`new_api.billing.cost_semantics=gateway_charge_usd`. The internal quota and
consume log remain authoritative for accounting.

`langfuse.observation.completion_start_time` is emitted for streamed requests
that have a recorded first response. It follows the existing `RelayInfo`
first-response boundary used by performance metrics; some providers may report
the first protocol response frame rather than a semantic generated token.

The trace is a side channel. Export errors are handled by the OTel batch exporter
and do not change the model response or billing result. Existing consume logs and
performance metrics remain the source of truth for billing and aggregate dashboards.
