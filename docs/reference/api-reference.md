# API Reference

Complete reference for TestService HTTP and gRPC APIs.

## HTTP API

TestService exposes an HTTP server on port 8080 (configurable via `HTTP_PORT`).

### Endpoints

#### GET /

Main request handler. Returns service information and upstream call chain.

**Request:**
```http
GET /?behavior=latency=100ms HTTP/1.1
Host: localhost:8080
X-Behavior: latency=100ms
```

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `behavior` | string | No | Behavior string to apply |

**Headers:**

| Header | Type | Required | Description |
|--------|------|----------|-------------|
| `X-Behavior` | string | No | Alternative to query parameter |
| `traceparent` | string | No | W3C trace context (auto-propagated) |
| `tracestate` | string | No | W3C trace state (auto-propagated) |

**Response:**

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
  "start_time": "2025-10-27T12:34:56.789123456Z",
  "end_time": "2025-10-27T12:34:56.891234567Z",
  "duration": "102.111111ms",
  "code": 200,
  "body": "All ok",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "upstream_calls": [
    {
      "name": "api",
      "uri": "http://api:8080",
      "protocol": "http",
      "duration": "45.123456ms",
      "code": 200,
      "error": "",
      "upstream_calls": []
    }
  ],
  "behaviors_applied": ["latency:fixed:100ms"]
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Success |
| 404 | No upstream matches path (path routing) |
| 500 | Server error or injected error behavior |
| 502 | Bad Gateway - upstream service returned non-2xx status |
| 503 | Service unavailable or injected error |
| 429 | Rate limit (injected error behavior) |

**Error Response Details:**
- **502 Bad Gateway**: Returned when an upstream service returns a status code >= 300. The error message includes the failing service name and status code.
- **500 Internal Server Error**: Returned for server errors or when error injection behavior is applied via the `error` behavior parameter.

**Examples:**

Basic request:
```bash
curl http://localhost:8080/
```

With latency:
```bash
curl 'http://localhost:8080/?behavior=latency=200ms'
```

With header:
```bash
curl -H 'X-Behavior: latency=200ms' http://localhost:8080/
```

With targeted behavior:
```bash
curl 'http://localhost:8080/?behavior=api:latency=500ms,error=0.1'
```

#### GET /health

Liveness probe endpoint. Checks if service is alive.

**Request:**
```http
GET /health HTTP/1.1
Host: localhost:8080
```

**Response:**
```json
{"status": "ok"}
```

**Status Codes:**
- 200: Service is alive
- 500: Service is not healthy

**Usage:**

Kubernetes liveness probe:
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
```

#### GET /ready

Readiness probe endpoint. Checks if service is ready to accept traffic.

**Request:**
```http
GET /ready HTTP/1.1
Host: localhost:8080
```

**Response:**
```json
{"status": "ready"}
```

**Status Codes:**
- 200: Service is ready
- 503: Service is not ready

**Usage:**

Kubernetes readiness probe:
```yaml
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

#### GET /metrics

Prometheus metrics endpoint.

**Request:**
```http
GET /metrics HTTP/1.1
Host: localhost:9091
```

**Response:**

```
# HELP testservice_requests_total Total requests received
# TYPE testservice_requests_total counter
testservice_requests_total{service="frontend",method="GET",status="200",protocol="http"} 1234

# HELP testservice_request_duration_seconds Request duration
# TYPE testservice_request_duration_seconds histogram
testservice_request_duration_seconds_bucket{service="frontend",le="0.005"} 100
testservice_request_duration_seconds_bucket{service="frontend",le="0.01"} 200
...
```

**Port:** 9091 (configurable via `METRICS_PORT`)

## gRPC API

TestService exposes a gRPC server on port 9090 (configurable via `GRPC_PORT`).

### Service Definition

**Package:** `testservice`

**Service:** `TestService`

**Protocol Buffer:**

```protobuf
syntax = "proto3";

package testservice;

service TestService {
  rpc Call(CallRequest) returns (ServiceResponse);
}

message CallRequest {
  string behavior = 1;
}

message ServiceResponse {
  ServiceInfo service = 1;
  string start_time = 2;
  string end_time = 3;
  string duration = 4;
  int32 code = 5;
  string body = 6;
  string trace_id = 7;
  string span_id = 8;
  repeated UpstreamCall upstream_calls = 9;
  repeated string behaviors_applied = 10;
}

message ServiceInfo {
  string name = 1;
  string version = 2;
  string namespace = 3;
  string pod = 4;
  string node = 5;
  string protocol = 6;
}

message UpstreamCall {
  string name = 1;
  string uri = 2;
  string protocol = 3;
  string duration = 4;
  int32 code = 5;
  string error = 6;
  repeated UpstreamCall upstream_calls = 7;
  repeated string behaviors_applied = 8;
}
```

### RPC: Call

Main RPC method. Equivalent to HTTP GET /.

**Request:**

```protobuf
message CallRequest {
  string behavior = 1;  // Optional behavior string
}
```

**Response:**

```protobuf
message ServiceResponse {
  ServiceInfo service = 1;
  string start_time = 2;
  string end_time = 3;
  string duration = 4;
  int32 code = 5;
  string body = 6;
  string trace_id = 7;
  string span_id = 8;
  repeated UpstreamCall upstream_calls = 9;
  repeated string behaviors_applied = 10;
}
```

**Metadata:**

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `traceparent` | string | No | W3C trace context |
| `tracestate` | string | No | W3C trace state |

**Examples:**

Using grpcurl:

```bash
# Basic call
grpcurl -plaintext localhost:9090 testservice.TestService/Call

# With behavior
grpcurl -plaintext -d '{"behavior":"latency=200ms"}' \
  localhost:9090 testservice.TestService/Call

# With metadata
grpcurl -plaintext \
  -H 'traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01' \
  localhost:9090 testservice.TestService/Call
```

Using Go client:

```go
import (
    "context"
    pb "github.com/aslakknutsen/kkbase-testapp/proto/testservice"
    "google.golang.org/grpc"
)

conn, _ := grpc.Dial("localhost:9090", grpc.WithInsecure())
client := pb.NewTestServiceClient(conn)

resp, err := client.Call(context.Background(), &pb.CallRequest{
    Behavior: "latency=100ms",
})
```

## Response Fields

### ServiceInfo

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Service name |
| `version` | string | Service version |
| `namespace` | string | Kubernetes namespace |
| `pod` | string | Pod name |
| `node` | string | Node name |
| `protocol` | string | "http" or "grpc" |

### Timing

| Field | Type | Description |
|-------|------|-------------|
| `start_time` | string | RFC3339Nano timestamp |
| `end_time` | string | RFC3339Nano timestamp |
| `duration` | string | Go duration format (e.g., "102.111ms") |

### Response Data

| Field | Type | Description |
|-------|------|-------------|
| `code` | int | HTTP status code or gRPC-equivalent |
| `body` | string | Response message |

### Tracing

| Field | Type | Description |
|-------|------|-------------|
| `trace_id` | string | OpenTelemetry trace ID (hex) |
| `span_id` | string | OpenTelemetry span ID (hex) |

### Upstream Calls

| Field | Type | Description |
|-------|------|-------------|
| `upstream_calls` | array | Nested upstream call information |
| `name` | string | Upstream service name |
| `uri` | string | Full upstream URI |
| `protocol` | string | "http" or "grpc" |
| `duration` | string | Call duration |
| `code` | int | Status code |
| `error` | string | Error message (if any) |
| `upstream_calls` | array | Recursive upstream calls |

### Behaviors

| Field | Type | Description |
|-------|------|-------------|
| `behaviors_applied` | array | List of applied behaviors with details |

Format: `type:subtype:value`

Examples:
- `latency:fixed:100ms`
- `error:503:0.30`
- `cpu:spike:5s:intensity=90`

## Client Timeouts

Default timeout: 30 seconds (configurable via `CLIENT_TIMEOUT_MS`)

**HTTP Client:**
- Connection timeout: 5 seconds
- Request timeout: 30 seconds

**gRPC Client:**
- Connection timeout: 5 seconds
- RPC timeout: 30 seconds

## Error Handling

### HTTP Errors

Errors return appropriate HTTP status codes with JSON body:

```json
{
  "error": "upstream call failed",
  "code": 503,
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736"
}
```

### gRPC Errors

Errors return gRPC status codes with error details in the response:

```go
code: 13  // INTERNAL
message: "upstream call failed"
```

## Rate Limiting

No built-in rate limiting. Use Kubernetes or service mesh policies for rate limiting.

## Authentication

No built-in authentication. This is a testing tool. For production use:
- Add JWT validation
- Implement mTLS
- Use API gateway for authentication

## See Also

- [Protocols](../concepts/protocols.md) - HTTP and gRPC details
- [Behavior Syntax](behavior-syntax.md) - Behavior string format
- [Observability](../concepts/observability.md) - Metrics and tracing
- [Proto File](../../proto/testservice/service.proto) - Complete protocol definition

