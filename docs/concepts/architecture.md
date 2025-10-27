# Architecture

TestApp consists of two main components working together to generate and run synthetic applications.

## Overview

```
DSL YAML File
     ↓
testgen (Parser + Validators)
     ↓
Manifest Generators
     ├── Kubernetes (Deployments, Services, StatefulSets)
     └── Gateway API (Gateway, HTTPRoute, GRPCRoute, TLS)
     ↓
Kubernetes Cluster
     ↓
TestService Pods
     ├── HTTP Server (port 8080)
     ├── gRPC Server (port 9090)
     ├── Metrics (port 9091)
     ├── Behavior Engine
     ├── Upstream Client
     └── Telemetry (OTEL, Prometheus, Zap)
```

## TestService Binary

The runtime service that handles requests and simulates application behavior.

### Component Structure

```
┌───────────────────────────────────────────────────────────┐
│                     TestService Binary                     │
├───────────────────────────────────────────────────────────┤
│                                                            │
│  ┌──────────────────┐    ┌──────────────────┐            │
│  │  HTTP Server     │    │  gRPC Server     │            │
│  │  Port: 8080      │    │  Port: 9090      │            │
│  │  - /            │    │  - Call() RPC    │            │
│  │  - /health      │    │                  │            │
│  │  - /ready       │    │                  │            │
│  └────────┬─────────┘    └────────┬─────────┘            │
│           │                       │                       │
│           └───────────┬───────────┘                       │
│                       ↓                                   │
│           ┌────────────────────────┐                      │
│           │   Upstream Caller      │                      │
│           │   - HTTP Client        │                      │
│           │   - gRPC Client        │                      │
│           │   - Trace Propagation  │                      │
│           └────────────────────────┘                      │
│                       ↑                                   │
│           ┌───────────┴───────────┐                       │
│           │                       │                       │
│  ┌────────┴─────────┐   ┌────────┴─────────┐            │
│  │ Behavior Engine  │   │   Telemetry      │            │
│  │ - Latency        │   │ - OTEL Traces    │            │
│  │ - Errors         │   │ - Prometheus     │            │
│  │ - CPU/Memory     │   │ - JSON Logs      │            │
│  └──────────────────┘   └──────────────────┘            │
│                                                            │
│  Metrics Server (Port 9091) - /metrics                    │
└───────────────────────────────────────────────────────────┘
```

### Key Packages

- **`pkg/service/http/`** - HTTP server with behavior parsing
- **`pkg/service/grpc/`** - gRPC server implementation
- **`pkg/service/behavior/`** - Behavior engine (latency, errors, CPU, memory)
- **`pkg/service/client/`** - Unified upstream caller
- **`pkg/service/telemetry/`** - OpenTelemetry, Prometheus, structured logging
- **`proto/testservice/`** - Protocol Buffer definitions

## TestGen CLI

The manifest generator that converts YAML DSL to Kubernetes resources.

### Pipeline

```
User DSL (YAML)
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
   YAML Manifests
```

### Core Packages

- **`pkg/dsl/types/`** - DSL data structures
- **`pkg/dsl/parser/`** - YAML parsing and validation
- **`pkg/generator/k8s/`** - Kubernetes manifest generation
- **`pkg/generator/gateway/`** - Gateway API manifest generation

### Validation Features

- Circular dependency detection
- Upstream reference validation
- Protocol compatibility checks
- StatefulSet requirements validation

## Design Patterns

### Unified Upstream Caller

Both HTTP and gRPC servers use a shared `client.Caller` component that:

- Detects protocol from URL scheme (`http://` vs `grpc://`)
- Routes to appropriate client implementation
- Handles trace propagation uniformly
- Returns standardized `client.Result`

**Benefits:**
- No code duplication between servers
- Consistent behavior across protocols
- Centralized testing
- Easy to add new protocols

### Protocol-Agnostic Response Format

Internal standardized format:

```go
type client.Result struct {
    Name, URI, Protocol string
    Duration time.Duration
    Code int
    Error string
    UpstreamCalls []Result
    BehaviorsApplied []string
}
```

Each server converts to its own format:
- HTTP → `service.UpstreamCall` (JSON)
- gRPC → `pb.UpstreamCall` (Protobuf)

This ensures identical response structures regardless of protocol.

### Behavior Engine

Behaviors are parsed from strings and executed using the command pattern:

```
"latency=10-50ms,error=0.05,cpu=spike"
  ↓
[LatencyBehavior, ErrorBehavior, CPUBehavior]
  ↓
Execute() → Side effects (sleep, return error, burn CPU)
```

Service-targeted behaviors allow precise testing:

```
"product-api:latency=500ms,order-api:error=0.5"
  ↓
Each service extracts only its applicable behavior
```

### Health Check Strategy

All services run HTTP server for health checks, even gRPC-only services:

- HTTP server runs on port 8080
- Exposes `/health` (liveness) and `/ready` (readiness)
- Health port not exposed in Service for gRPC-only
- Used internally by Kubernetes probes

**Rationale:**
- Kubernetes has better HTTP probe support
- Simpler than gRPC health check protocol
- Consistent across all services

### Observability Integration

Every operation is wrapped with observability:

```go
ctx, span := tracer.Start(ctx, "operation")
defer span.End()

// Do work
result := doWork()

// Record metrics
metrics.RecordDuration(duration)

// Log with correlation
logger.Info("completed", zap.String("trace_id", traceID))
```

## Protocol Support Matrix

| Caller | Upstream | Implementation | Trace Propagation |
|--------|----------|----------------|-------------------|
| HTTP   | HTTP     | `http.Client`  | W3C Headers       |
| HTTP   | gRPC     | `grpc.Dial`    | gRPC Metadata     |
| gRPC   | HTTP     | `http.Client`  | W3C Headers       |
| gRPC   | gRPC     | `grpc.Dial`    | gRPC Metadata     |

All combinations support:
- Full distributed tracing
- Nested call chains
- Behavior injection at each hop
- Prometheus metrics
- Structured logging

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

### Environment Variables

Runtime configuration via env vars:

```bash
SERVICE_NAME=frontend
NAMESPACE=simple-web
POD_NAME=frontend-xyz         # from downward API
NODE_NAME=node-1               # from downward API
HTTP_PORT=8080
GRPC_PORT=9090
METRICS_PORT=9091
UPSTREAMS=api:http://api.simple-web:8080
DEFAULT_BEHAVIOR=latency=10ms,error=0.01
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
```

### Request-Time Behavior

Dynamic overrides via query params or headers:

```bash
# HTTP
curl "http://service:8080/?behavior=latency=500ms,error=0.5"

# gRPC (passed in CallRequest.Behavior field)
```

## Dependencies

### Core
- `google.golang.org/grpc` - gRPC framework
- `google.golang.org/protobuf` - Protocol Buffers
- `go.uber.org/zap` - Structured logging
- `github.com/prometheus/client_golang` - Metrics
- `go.opentelemetry.io/otel` - Distributed tracing
- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/spf13/cobra` - CLI framework

### Template System

Manifest generation uses Go's `text/template` package with embedded template files:

- Templates stored in `pkg/generator/k8s/templates/` and `pkg/generator/gateway/templates/`
- Embedded via `//go:embed` directive
- Compiled into single binary

## Performance Characteristics

### Resource Usage (per instance)
- CPU: 10-20m idle, configurable with behavior
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

1. Define in `behavior/engine.go`
2. Parse in behavior string parser
3. Execute in `ApplyBehavior()`

### Adding New Resource Types

1. Define in `dsl/types/types.go`
2. Validate in `dsl/parser/parser.go`
3. Generate in `generator/k8s/` or `generator/gateway/`

## See Also

- [Protocol Support](protocols.md) - Detailed protocol behavior
- [Observability](observability.md) - Metrics, traces, and logs
- [Behavior Testing](../guides/behavior-testing.md) - Using the behavior engine

