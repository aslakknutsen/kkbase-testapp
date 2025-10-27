# Observability

TestService provides comprehensive observability through metrics, distributed tracing, and structured logging.

## Metrics (Prometheus)

TestService exposes Prometheus metrics on the `/metrics` endpoint (default port 9091).

### Available Metrics

**Request Metrics**

- `testservice_requests_total` - Counter
  - Labels: `service`, `method`, `status`, `protocol`
  - Total requests received by service

- `testservice_request_duration_seconds` - Histogram
  - Labels: `service`, `method`, `protocol`
  - Request duration distribution

- `testservice_active_requests` - Gauge
  - Labels: `service`, `protocol`
  - Current active requests

**Upstream Metrics**

- `testservice_upstream_calls_total` - Counter
  - Labels: `service`, `upstream`, `status`
  - Total upstream calls made

- `testservice_upstream_duration_seconds` - Histogram
  - Labels: `service`, `upstream`
  - Upstream call duration distribution

**Behavior Metrics**

- `testservice_behavior_applied_total` - Counter
  - Labels: `service`, `behavior_type`
  - Count of behaviors applied

### Accessing Metrics

```bash
# Port forward to metrics port
kubectl port-forward svc/frontend 9091:9091

# View all metrics
curl http://localhost:9091/metrics

# Filter for testservice metrics
curl http://localhost:9091/metrics | grep testservice
```

### ServiceMonitor

TestGen automatically creates ServiceMonitor CRDs for Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: frontend-monitor
spec:
  selector:
    matchLabels:
      app: frontend
  endpoints:
  - port: metrics
    path: /metrics
```

### Prometheus Queries

**Request rate:**

```promql
rate(testservice_requests_total{service="frontend"}[5m])
```

**Error rate:**

```promql
sum(rate(testservice_requests_total{status=~"5.."}[5m])) 
/ 
sum(rate(testservice_requests_total[5m])) * 100
```

**P95 latency:**

```promql
histogram_quantile(0.95, 
  rate(testservice_request_duration_seconds_bucket{service="frontend"}[5m]))
```

**Upstream call success rate:**

```promql
sum(rate(testservice_upstream_calls_total{status="200"}[5m]))
/
sum(rate(testservice_upstream_calls_total[5m])) * 100
```

## Distributed Tracing (OpenTelemetry)

TestService uses OpenTelemetry for distributed tracing with OTLP/gRPC export.

### Configuration

Set the OTEL collector endpoint via environment variable:

```yaml
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "otel-collector:4317"
```

This is automatically configured in generated manifests.

### Trace Context Propagation

TestService propagates W3C trace context across all protocol boundaries:

- **HTTP**: via `traceparent` and `tracestate` headers
- **gRPC**: via metadata fields

All upstream calls maintain the trace context, creating a complete distributed trace.

### Trace Data

Each trace includes:

- **Parent span** for each request
- **Child spans** for upstream calls
- **Span events** for behaviors applied
- **Attributes**:
  - `service.name` - Service name
  - `service.namespace` - Kubernetes namespace
  - `http.method` - HTTP method
  - `http.status_code` - Response status
  - `rpc.service` - gRPC service name
  - `rpc.method` - gRPC method name

### Extracting Trace IDs

Trace IDs are included in all responses:

```json
{
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  ...
}
```

Use these to find traces in your tracing backend:

```bash
TRACE_ID=$(curl -s http://localhost:8080/ | jq -r '.trace_id')
echo "View trace: http://localhost:16686/trace/$TRACE_ID"
```

### Jaeger Integration

See the [Jaeger Setup Guide](../guides/jaeger-setup.md) for deploying Jaeger and configuring TestService to send traces.

## Structured Logging

TestService uses Zap for structured JSON logging.

### Log Format

All logs are JSON with consistent fields:

```json
{
  "level": "info",
  "ts": "2025-10-24T12:34:56.789Z",
  "service": "frontend",
  "namespace": "default",
  "pod": "frontend-abc123",
  "node": "node-1",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "msg": "Request completed",
  "method": "GET",
  "path": "/",
  "duration": "45.123ms",
  "status": 200
}
```

### Log Levels

- `debug` - Detailed debugging information
- `info` - General informational messages
- `warn` - Warning messages
- `error` - Error messages

Configure via `LOG_LEVEL` environment variable (default: `info`).

### Key Log Events

**Request received:**
```json
{"level":"info","msg":"Request received","method":"GET","path":"/"}
```

**Request completed:**
```json
{"level":"info","msg":"Request completed","duration":"45ms","status":200}
```

**Upstream call:**
```json
{"level":"info","msg":"Calling upstream","upstream":"api","url":"http://api:8080"}
```

**Behavior applied:**
```json
{"level":"info","msg":"Behavior applied","behavior":"latency:100ms"}
```

**Error occurred:**
```json
{"level":"error","msg":"Upstream call failed","upstream":"api","error":"connection refused"}
```

### Viewing Logs

```bash
# View logs from a specific service
kubectl logs -l app=frontend

# Follow logs in real-time
kubectl logs -l app=frontend -f

# View logs with trace ID correlation
kubectl logs -l app=frontend | grep "4bf92f3577b34da6a3ce929d0e0e4736"

# Parse JSON logs with jq
kubectl logs -l app=frontend | jq -r 'select(.level=="error") | .msg'
```

### Log Correlation

Trace and span IDs are included in all log entries, allowing correlation between:
- Logs and traces
- Logs across services in the same request chain
- Metrics and logs

## Response Structure

Every TestService response includes observability metadata:

```json
{
  "service": {
    "name": "frontend",
    "version": "1.0.0",
    "namespace": "default",
    "pod": "frontend-abc123",
    "node": "node-1",
    "protocol": "http"
  },
  "start_time": "2025-10-24T12:34:56.789Z",
  "end_time": "2025-10-24T12:34:56.891Z",
  "duration": "102ms",
  "code": 200,
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "upstream_calls": [...],
  "behaviors_applied": [...]
}
```

This enables:
- **Request timing analysis** - Start, end, duration
- **Trace correlation** - Via trace_id and span_id
- **Service identification** - Service, pod, node, namespace
- **Behavior tracking** - Which behaviors were applied

## Integration Patterns

### Prometheus + Grafana

1. Deploy Prometheus with ServiceMonitor support
2. ServiceMonitors are automatically created by TestGen
3. Import Grafana dashboards to visualize metrics

### Jaeger + Elasticsearch

1. Deploy Jaeger with Elasticsearch backend
2. Configure TestService with OTEL endpoint
3. View distributed traces in Jaeger UI

### ELK Stack

1. Deploy Elasticsearch, Logstash, Kibana
2. Configure Logstash to parse JSON logs
3. Create Kibana dashboards with trace ID correlation

### Complete Observability Stack

Example stack for comprehensive observability:

```
TestService Pods
    │
    ├──> Prometheus (metrics)
    │        └──> Grafana (visualization)
    │
    ├──> OTEL Collector (traces)
    │        └──> Jaeger (trace storage & UI)
    │
    └──> Fluent Bit (logs)
             └──> Elasticsearch/Loki (log storage)
                      └──> Kibana/Grafana (log search & viz)
```

### Correlation Across Signals

Use trace IDs to correlate:

**Metrics → Traces:**
```promql
# Find high-latency requests in Prometheus
topk(5, testservice_request_duration_seconds)
# Copy trace IDs from labels, search in Jaeger
```

**Traces → Logs:**
```bash
# Get trace ID from Jaeger span
# Search logs: kubectl logs | grep "<trace-id>"
```

**Logs → Metrics:**
```bash
# Find errors in logs, get service name
# Query metrics for that service
curl 'http://prometheus/api/v1/query?query=testservice_requests_total{service="frontend",status=~"5.."}'
```

## Performance Considerations

### Metrics

- Low overhead (~10 time series per service)
- Sub-millisecond recording time
- Scrape interval: typically 15-30s

### Tracing

- 100% sampling by default (suitable for testing)
- For high-volume production, use sampling:
  ```go
  sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)) // 10%
  ```
- Span creation overhead: ~0.1ms per span

### Logging

- Structured JSON: ~0.05ms per log entry
- Avoid verbose logging in hot paths
- Log level impacts volume and performance

## See Also

- [Jaeger Setup Guide](../guides/jaeger-setup.md) - Deploy distributed tracing
- [Architecture](architecture.md) - Observability integration points
- [Protocols](protocols.md) - Trace propagation across protocols

