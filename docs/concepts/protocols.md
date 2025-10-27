# Multi-Protocol Support

TestService supports both HTTP and gRPC protocols, allowing you to build realistic microservice topologies with mixed protocol communication.

## Protocol Support Matrix

| Caller Protocol | Upstream Protocol | Status | Implementation | Trace Propagation |
|-----------------|-------------------|--------|----------------|-------------------|
| HTTP | HTTP | Supported | `http.Client` | W3C Headers |
| HTTP | gRPC | Supported | `grpc.Dial` | gRPC Metadata |
| gRPC | HTTP | Supported | `http.Client` | W3C Headers |
| gRPC | gRPC | Supported | `grpc.Dial` | gRPC Metadata |

All protocol combinations work seamlessly with:
- Full distributed tracing
- Nested call chains
- Behavior injection
- Prometheus metrics
- Structured logging

## Response Format Consistency

Both HTTP and gRPC return identical field structures, ensuring consistent observability regardless of protocol.

### Field Comparison

| Field | HTTP (JSON) | gRPC (Protobuf) | Notes |
|-------|-------------|-----------------|-------|
| **Service Info** |
| service.name | string | string | Service name |
| service.version | string | string | Version (default: 1.0.0) |
| service.namespace | string | string | Kubernetes namespace |
| service.pod | string | string | Pod name |
| service.node | string | string | Node name |
| service.protocol | "http" or "grpc" | "http" or "grpc" | Protocol used |
| **Timing** |
| start_time | RFC3339Nano | RFC3339Nano | Request start |
| end_time | RFC3339Nano | RFC3339Nano | Request end |
| duration | Go duration string | Go duration string | Total duration |
| **Response** |
| code | int | int32 | HTTP/gRPC status code |
| body | string | string | Response message |
| **Tracing** |
| trace_id | hex string | hex string | OpenTelemetry trace ID |
| span_id | hex string | hex string | OpenTelemetry span ID |
| **Upstream Calls** |
| upstream_calls | array | repeated | Nested upstream responses |
| upstream_calls[].name | string | string | Upstream service name |
| upstream_calls[].uri | string | string | Full upstream URI |
| upstream_calls[].protocol | string | string | Protocol used |
| upstream_calls[].duration | string | string | Call duration |
| upstream_calls[].code | int | int32 | Status code |
| upstream_calls[].error | string | string | Error message if any |
| upstream_calls[].upstream_calls | recursive | recursive | Nested calls |
| **Behaviors** |
| behaviors_applied | array of strings | repeated string | Applied behaviors |

## Example Responses

### HTTP Response (JSON)

```json
{
  "service": {
    "name": "order-api",
    "version": "1.0.0",
    "namespace": "orders",
    "pod": "order-api-abc123",
    "node": "node-1",
    "protocol": "http"
  },
  "start_time": "2025-10-24T12:34:56.789123456Z",
  "end_time": "2025-10-24T12:34:56.891234567Z",
  "duration": "102.111111ms",
  "code": 200,
  "body": "Hello from order-api (HTTP)",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "upstream_calls": [
    {
      "name": "product-api",
      "uri": "http://product-api.products.svc.cluster.local:8080",
      "protocol": "http",
      "duration": "45.123456ms",
      "code": 200
    },
    {
      "name": "payment-api",
      "uri": "grpc://payment-api.payments.svc.cluster.local:9090",
      "protocol": "grpc",
      "duration": "38.765432ms",
      "code": 200
    }
  ],
  "behaviors_applied": ["latency:fixed:10ms"]
}
```

### gRPC Response (Protobuf, shown as JSON)

```json
{
  "service": {
    "name": "order-api",
    "version": "1.0.0",
    "namespace": "orders",
    "pod": "order-api-abc123",
    "node": "node-1",
    "protocol": "grpc"
  },
  "start_time": "2025-10-24T12:34:56.789123456Z",
  "end_time": "2025-10-24T12:34:56.891234567Z",
  "duration": "102.111111ms",
  "code": 200,
  "body": "Hello from order-api (gRPC)",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "upstream_calls": [
    {
      "name": "product-api",
      "uri": "http://product-api.products.svc.cluster.local:8080",
      "protocol": "http",
      "duration": "45.123456ms",
      "code": 200
    },
    {
      "name": "payment-api",
      "uri": "grpc://payment-api.payments.svc.cluster.local:9090",
      "protocol": "grpc",
      "duration": "38.765432ms",
      "code": 200
    }
  ],
  "behaviors_applied": ["latency:fixed:10ms"]
}
```

The only differences are:
- `service.protocol` field value
- `body` message includes protocol indicator
- Wire format (JSON vs Protobuf)

## Protocol Detection

The upstream caller automatically detects protocol from the URI scheme:

```go
if strings.HasPrefix(upstream.URL, "http://") || 
   strings.HasPrefix(upstream.URL, "https://") {
    // Use HTTP client
    return c.callHTTP(ctx, name, upstream, behaviorStr)
}
if strings.HasPrefix(upstream.URL, "grpc://") {
    // Use gRPC client
    return c.callGRPC(ctx, name, upstream, behaviorStr)
}
```

## Trace Propagation

### HTTP to HTTP

W3C trace context headers are automatically propagated:

```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
tracestate: vendor=value
```

### HTTP to gRPC

Trace context is extracted from HTTP headers and injected into gRPC metadata:

```
HTTP Headers (traceparent)
  ↓ Extract
OpenTelemetry Context
  ↓ Inject
gRPC Metadata (traceparent)
```

### gRPC to HTTP

Trace context is extracted from gRPC metadata and injected into HTTP headers:

```
gRPC Metadata (traceparent)
  ↓ Extract
OpenTelemetry Context
  ↓ Inject
HTTP Headers (traceparent)
```

### gRPC to gRPC

Trace context propagates through gRPC metadata:

```
gRPC Metadata (traceparent)
  ↓ Extract/Inject
gRPC Metadata (traceparent)
```

## When to Use Each Protocol

### Use HTTP When

- Building external-facing APIs
- Simple request/response patterns
- RESTful semantics needed
- Wide client compatibility required
- Human-readable debugging preferred

### Use gRPC When

- Building internal high-performance services
- Strong typing and schema validation needed
- Bidirectional streaming required
- Efficient binary protocol preferred
- Language-agnostic RPC needed

### Mixed Protocol Architectures

Common patterns:
- **API Gateway (HTTP) → Internal Services (gRPC)** - Public HTTP API, efficient internal communication
- **Frontend (HTTP) → Backend (gRPC) → Database (HTTP)** - Mix based on service requirements
- **Service Mesh** - All protocols supported with consistent observability

## Configuration

### HTTP Service

```yaml
services:
  - name: frontend
    protocols: [http]
    ports:
      http: 8080
      metrics: 9091
```

### gRPC Service

```yaml
services:
  - name: order-api
    protocols: [grpc]
    ports:
      grpc: 9090
      metrics: 9091
```

### Dual-Protocol Service

```yaml
services:
  - name: api-gateway
    protocols: [http, grpc]
    ports:
      http: 8080
      grpc: 9090
      metrics: 9091
```

## Upstream Configuration

Specify protocol in the upstream URL:

```yaml
services:
  - name: web
    upstreams:
      - name: api
        url: http://api.backend:8080
      - name: order-service
        url: grpc://order-service.orders:9090
```

Or using environment variable format:

```bash
UPSTREAMS=api:http://api.backend:8080,order:grpc://order-service.orders:9090
```

## Testing Mixed Protocols

### Test HTTP Service

```bash
kubectl port-forward svc/frontend 8080:8080
curl http://localhost:8080/
```

### Test gRPC Service

```bash
kubectl port-forward svc/order-api 9090:9090
grpcurl -plaintext localhost:9090 testservice.TestService/Call
```

### Test Protocol Translation

Create a chain: HTTP → gRPC → HTTP

```yaml
services:
  - name: web
    protocols: [http]
    upstreams: [order-api]
  - name: order-api
    protocols: [grpc]
    upstreams: [database]
  - name: database
    protocols: [http]
```

Request through web service:

```bash
curl http://web:8080/
```

Response shows the protocol chain:

```json
{
  "service": {"protocol": "http"},
  "upstream_calls": [
    {
      "name": "order-api",
      "protocol": "grpc",
      "upstream_calls": [
        {"name": "database", "protocol": "http"}
      ]
    }
  ]
}
```

## Benefits of Multi-Protocol Support

- **Realistic Testing** - Match production architectures
- **Protocol Flexibility** - Choose the right tool for each service
- **Seamless Integration** - No manual translation needed
- **Consistent Observability** - Same metrics/traces regardless of protocol
- **Easy Migration** - Test protocol changes without rewriting services

## Limitations

- gRPC connections use `WithInsecure()` (no TLS) - suitable for testing only
- No bidirectional streaming - only unary RPC calls
- No authentication between services

For production use, add:
- mTLS for gRPC connections
- Authentication (JWT, mTLS)
- Proper certificate management

## See Also

- [Architecture](architecture.md) - Overall system design
- [API Reference](../reference/api-reference.md) - HTTP and gRPC APIs
- [Observability](observability.md) - Tracing across protocols

