# TestApp Architecture

## Overview

TestApp is a synthetic application generator for testing monitoring systems. It consists of two main components:

1. **TestService** - A multi-protocol service binary that can simulate complex application behavior
2. **TestGen** - A CLI tool that generates Kubernetes manifests from YAML DSL definitions

## Component Architecture

```
┌───────────────────────────────────────────────────────────────┐
│                         TestService Binary                     │
├───────────────────────────────────────────────────────────────┤
│                                                                │
│  ┌──────────────────┐    ┌──────────────────┐                │
│  │  HTTP Server     │    │  gRPC Server     │                │
│  │  Port: 8080      │    │  Port: 9090      │                │
│  │  - /            │    │  - Call() RPC    │                │
│  │  - /health      │    │                  │                │
│  │  - /ready       │    │                  │                │
│  └────────┬─────────┘    └────────┬─────────┘                │
│           │                       │                           │
│           └───────────┬───────────┘                           │
│                       ↓                                       │
│           ┌────────────────────────┐                          │
│           │   Upstream Caller      │                          │
│           │   (pkg/service/client) │                          │
│           │   - HTTP Client        │                          │
│           │   - gRPC Client        │                          │
│           │   - Trace Propagation  │                          │
│           └────────────────────────┘                          │
│                       ↑                                       │
│           ┌───────────┴───────────┐                           │
│           │                       │                           │
│  ┌────────┴─────────┐   ┌────────┴─────────┐                │
│  │ Behavior Engine  │   │   Telemetry      │                │
│  │ - Latency        │   │ - OTEL Traces    │                │
│  │ - Errors         │   │ - Prometheus     │                │
│  │ - CPU/Memory     │   │ - JSON Logs      │                │
│  └──────────────────┘   └──────────────────┘                │
│                                                                │
│  Metrics Server (Port 9091) - /metrics                        │
└───────────────────────────────────────────────────────────────┘
```

## Key Design Patterns

### 1. Unified Upstream Caller

**Pattern**: Single Responsibility + Strategy Pattern

Both HTTP and gRPC servers use a shared `client.Caller` that:
- Detects protocol from URL scheme (`http://` vs `grpc://`)
- Routes to appropriate client implementation
- Handles trace propagation uniformly
- Returns standardized `client.Result`

**Benefits**:
- No code duplication
- Consistent behavior across protocols
- Easy to add new protocols
- Centralized testing

### 2. Protocol-Agnostic Response Format

**Pattern**: Adapter Pattern

```go
// Internal standardized format
type client.Result struct {
    Name, URI, Protocol string
    Duration time.Duration
    Code int
    Error string
    UpstreamCalls []Result
}
```

Each server converts to its own format:
- HTTP → `service.UpstreamCall` (JSON)
- gRPC → `pb.UpstreamCall` (Protobuf)

### 3. Behavior Engine

**Pattern**: Command Pattern + Interpreter Pattern

Behaviors are parsed from strings and executed:

```
"latency=10-50ms,error=0.05,cpu=200m"
  ↓
[LatencyBehavior, ErrorBehavior, CPUBehavior]
  ↓
Execute() → Side effects (sleep, return error, burn CPU)
```

### 4. Observability Integration

**Pattern**: Decorator Pattern

Every operation is wrapped with observability:

```go
ctx, span := tracer.Start(ctx, "operation")
defer span.End()

// Do work
result := doWork()

// Record metrics
metrics.RecordDuration(duration)

// Log
logger.Info("completed", zap.Duration("duration", duration))
```

## Protocol Support Matrix

| Source | Target | Implementation | Trace Propagation |
|--------|--------|----------------|-------------------|
| HTTP   | HTTP   | `http.Client`  | W3C Headers       |
| HTTP   | gRPC   | `grpc.Dial`    | gRPC Metadata     |
| gRPC   | HTTP   | `http.Client`  | W3C Headers       |
| gRPC   | gRPC   | `grpc.Dial`    | gRPC Metadata     |

All combinations support:
- Full distributed tracing
- Nested call chains
- Behavior injection at each hop
- Prometheus metrics
- Structured logging

## Manifest Generation Flow

```
DSL YAML File
     ↓
┌─────────────────┐
│  DSL Parser     │  Validates syntax, references, circular deps
│  (types/parser) │  
└────────┬────────┘
         ↓
┌─────────────────┐
│  K8s Generator  │  Creates: Deployment, Service, StatefulSet, etc.
│  (generator/k8s)│  
└────────┬────────┘
         ↓
┌─────────────────┐
│ Gateway Gen     │  Creates: Gateway, HTTPRoute, GRPCRoute, TLS
│ (generator/     │  
│  gateway)       │  
└────────┬────────┘
         ↓
   YAML Manifests (output/)
```

## Health Check Strategy

**Design Decision**: Always run HTTP server for health checks

Even for gRPC-only services:
- HTTP server runs on port 8080
- Exposes `/health` (liveness) and `/ready` (readiness)
- Health port NOT exposed in Service for gRPC-only
- Used internally by Kubernetes probes only

**Rationale**:
- Kubernetes has better HTTP probe support
- Simpler than gRPC health checks
- Consistent across all services
- No dependency on gRPC health check protocol

## Trace Context Propagation

### HTTP → HTTP
```
Request → W3C Headers (traceparent, tracestate)
  ↓
OTEL Propagator.Extract()
  ↓
Pass context to upstream HTTP client
  ↓
OTEL Propagator.Inject() → Upstream headers
```

### HTTP → gRPC
```
Request → W3C Headers
  ↓
OTEL Propagator.Extract()
  ↓
Pass context to gRPC client
  ↓
OTEL Propagator.Inject() → gRPC metadata
```

### gRPC → HTTP
```
gRPC Request → metadata
  ↓
OTEL Propagator.Extract() from metadata
  ↓
Pass context to HTTP client
  ↓
OTEL Propagator.Inject() → HTTP headers
```

### gRPC → gRPC
```
gRPC Request → metadata
  ↓
OTEL Propagator.Extract() from metadata
  ↓
Pass context to gRPC client
  ↓
OTEL Propagator.Inject() → gRPC metadata
```

## Resource Naming Conventions

### Labels
All resources include:
```yaml
labels:
  app: <service-name>
  version: v1
  part-of: <app-name>
  # Plus custom labels from DSL
```

### Service DNS
Format: `<service>.<namespace>.svc.cluster.local:<port>`

Examples:
- `frontend.simple-web.svc.cluster.local:8080` (HTTP)
- `order-api.orders.svc.cluster.local:9090` (gRPC)

### Gateway Routes
- HTTPRoute: `<service>-httproute`
- GRPCRoute: `<service>-grpcroute`
- Gateway: `<app-name>-gateway`

## Configuration Strategy

**Environment Variables** (preferred for runtime config):
```bash
SERVICE_NAME=frontend
NAMESPACE=simple-web
POD_NAME=frontend-xyz
NODE_NAME=node-1
HTTP_PORT=8080
GRPC_PORT=9090
METRICS_PORT=9091
UPSTREAMS=api:http://api.simple-web:8080
DEFAULT_BEHAVIOR=latency=10ms,error=0.01
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
```

**Request-Time Behavior** (dynamic overrides):
```bash
# HTTP
curl "http://service:8080/?behavior=latency=500ms,error=0.5"

# gRPC
# Passed in CallRequest.Behavior field
```

## Security Considerations

Current implementation:
- ⚠️ gRPC uses `WithInsecure()` - No TLS
- ⚠️ No authentication between services
- ⚠️ Self-signed certs for Gateway TLS

**Note**: This is a testing tool. For production use:
- Enable mTLS
- Add authentication (JWT, mTLS)
- Use proper certificate management
- Implement RBAC

## Performance Characteristics

### Resource Usage (per instance)
- CPU: 10-20m idle, configurable behavior
- Memory: 32-64Mi base
- Connections: 1 per upstream
- Request latency: <1ms + behavior overhead

### Scalability
- Tested: 50+ services, 100+ pods
- Limited by: Kubernetes API, OTEL collector capacity
- Bottleneck: Usually the monitoring system, not TestApp

### Metrics Volume
Per service per second at 100 req/s:
- Traces: ~100 spans/s
- Metrics: ~10 time series
- Logs: ~200 log lines/s

## Extension Points

### Adding New Protocols
1. Implement in `client.Caller.Call()`
2. Add protocol detection
3. Handle trace propagation
4. Convert to `client.Result`

### Adding New Behaviors
1. Define in `behavior/types.go`
2. Parse in `behavior/parser.go`
3. Execute in `behavior/engine.go`

### Adding New Resource Types
1. Define in `dsl/types/types.go`
2. Validate in `dsl/parser/parser.go`
3. Generate in `generator/k8s/` or `generator/gateway/`

## Testing Strategy

### Unit Tests
- Behavior engine parsing and execution
- DSL parser validation
- Manifest generator output

### Integration Tests
- Generate→Apply→Verify pods running
- Cross-protocol upstream calls
- Trace propagation end-to-end

### E2E Tests
- Deploy full example app
- Send traffic through Gateway
- Verify traces in Jaeger
- Verify metrics in Prometheus

---

**Summary**: TestApp is designed as a flexible, observable, and protocol-agnostic synthetic application generator with minimal code duplication and maximum extensibility.

