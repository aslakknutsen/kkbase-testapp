# TestApp Implementation Summary

This document provides a technical overview of the TestApp implementation for the kkbase monitoring system.

## What Was Built

TestApp is a synthetic application generator consisting of two main components:

1. **TestService** - A multi-protocol service binary (HTTP & gRPC)
2. **TestGen** - A CLI tool for generating Kubernetes/Gateway API manifests from YAML DSL

## Architecture Overview

```
User DSL (YAML)
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

## Component Details

### 1. TestService Binary

**Location**: `cmd/testservice/main.go`

**Core Packages**:
- `pkg/service/http/` - HTTP server with behavior parsing
- `pkg/service/grpc/` - gRPC server implementation
- `pkg/service/behavior/` - Behavior engine (latency, errors, CPU, memory)
- `pkg/service/telemetry/` - OpenTelemetry, Prometheus, structured logging
- `proto/testservice/` - Protocol Buffer definitions

**Key Features**:
- Multi-protocol support (HTTP + gRPC simultaneously)
- Runtime behavior modification via query params or headers
- Complete call chain tracing with nested upstream responses
- W3C trace context propagation
- Prometheus metrics for requests, upstreams, and behaviors
- Structured JSON logging with trace correlation
- Environment-based configuration

**Supported Behaviors**:
- Latency: fixed, range (`50-200ms`)
- Error injection: rate-based with status codes (`503:0.1`)
- CPU load: spike, steady, ramp patterns
- Memory: leak-slow, leak-fast patterns

### 2. TestGen CLI

**Location**: `cmd/testgen/main.go`

**Commands**:
- `generate` - Generate manifests from DSL
- `validate` - Validate DSL without generating
- `apply` - Generate and apply (wrapper)
- `delete` - Generate delete commands
- `init` - Create starter template
- `examples` - List available examples

**Core Packages**:
- `pkg/dsl/types/` - DSL data structures
- `pkg/dsl/parser/` - YAML parsing and validation
- `pkg/generator/k8s/` - Kubernetes manifest generation
- `pkg/generator/gateway/` - Gateway API manifest generation

**Validation Features**:
- Circular dependency detection
- Upstream reference validation
- Protocol compatibility checks
- StatefulSet requirements validation

### 3. DSL Schema

**Location**: `pkg/dsl/types/types.go`

**Top-Level Structure**:
```yaml
app:              # Application metadata
  name: string
  namespaces: []string

services:         # Service definitions
  - name: string
    namespace: string
    replicas: int
    type: Deployment|StatefulSet|DaemonSet
    protocols: [http, grpc]
    upstreams: []string
    behavior: {...}
    ingress: {...}
    resources: {...}
    storage: {...}

traffic:          # Traffic generators (future)
  - name: string
    target: string
    rate: string
    pattern: string

scenarios:        # Time-based scenarios (future)
  - name: string
    at: duration
    action: string
```

### 4. Manifest Generators

#### Kubernetes Generator (`pkg/generator/k8s/`)

Generates:
- **Deployment** - Standard workloads with replicas
- **StatefulSet** - Stateful services with PVC templates
- **DaemonSet** - Node-level services
- **Service** - ClusterIP services with appropriate ports
- **ServiceMonitor** - Prometheus Operator CRD
- **Namespace** - Multiple namespaces

Features:
- Downward API for pod/node/namespace info
- Resource requests/limits
- Health probes (liveness/readiness)
- Upstream configuration via env vars
- Behavior strings as env vars
- Label management

#### Gateway API Generator (`pkg/generator/gateway/`)

Generates:
- **Gateway** - HTTP/HTTPS listeners
- **HTTPRoute** - Path-based routing with backend refs
- **GRPCRoute** - gRPC service routing
- **TLS Secrets** - Self-signed certificates (RSA 2048)
- **ReferenceGrant** - Cross-namespace service references

Features:
- Automatic TLS certificate generation
- Multi-namespace support
- Hostname-based routing
- Path prefix matching

## Example Applications

### 1. Simple-Web (`examples/simple-web/`)
- **Services**: 3 (frontend, api, database)
- **Topology**: frontend → api → database (HTTP)
- **Features**: Basic 3-tier app with StatefulSet
- **Use Case**: Quick validation, basic testing

### 2. E-Commerce (`examples/ecommerce/`)
- **Services**: 8 across 4 namespaces
- **Topology**: Complex multi-namespace with mixed protocols
- **Namespaces**: frontend, orders, products, payments
- **Protocols**: HTTP + gRPC
- **Features**: Cross-namespace calls, ReferenceGrants, multiple databases
- **Use Case**: Gateway API testing, multi-namespace monitoring

### 3. Microservices Mesh (`examples/microservices/`)
- **Services**: 15+ across 3 namespaces
- **Topology**: Deep call chains, multiple ingress points
- **Namespaces**: public, internal, data
- **Features**: Mixed protocols, multiple StatefulSets, complex dependencies
- **Use Case**: Large-scale monitoring testing, service mesh validation

## Observability Stack

### Metrics (Prometheus)
- Counter: `testservice_requests_total` (service, method, status, protocol)
- Histogram: `testservice_request_duration_seconds` (service, method, protocol)
- Counter: `testservice_upstream_calls_total` (service, upstream, status)
- Histogram: `testservice_upstream_duration_seconds` (service, upstream)
- Gauge: `testservice_active_requests` (service, protocol)
- Counter: `testservice_behavior_applied_total` (service, behavior_type)

### Traces (OpenTelemetry)
- OTLP gRPC exporter
- W3C trace context propagation (HTTP headers + gRPC metadata)
- Parent/child span relationships for upstream calls
- Span events for behavior application
- Attributes: service.name, namespace, http.method, status_code

### Logs (Structured)
- Format: JSON
- Library: Zap
- Fields: service, namespace, pod, node, trace_id, span_id, level, msg, timestamp
- Events: request_start, request_end, upstream_call, behavior_applied

## Code Statistics

```
Total Files: 30+
Total Lines: ~3,500+

Breakdown:
- TestService: ~1,200 lines
- TestGen: ~800 lines
- DSL/Parser: ~400 lines
- Generators: ~1,000 lines
- Examples: ~300 lines
- Documentation: ~600 lines
```

## Dependencies

**Core**:
- `google.golang.org/grpc` - gRPC framework
- `google.golang.org/protobuf` - Protocol Buffers
- `go.uber.org/zap` - Structured logging
- `github.com/prometheus/client_golang` - Metrics
- `go.opentelemetry.io/otel` - Distributed tracing
- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/spf13/cobra` - CLI framework

## Testing the System

### Unit Testing
```bash
cd testapp
go test ./...
```

### Integration Testing
```bash
# Generate example
./testgen generate examples/simple-web/app.yaml

# Deploy to local cluster
kubectl apply -f output/simple-web/

# Test request
kubectl port-forward svc/frontend 8080:8080
curl http://localhost:8080/
```

### Validation Testing
```bash
# Test DSL validation
./testgen validate examples/simple-web/app.yaml

# Test circular dependency detection
# (create a DSL with A→B→C→A and validate)
```

## Future Enhancements

### Completed
- ✅ Multi-protocol service (HTTP + gRPC)
- ✅ Behavior engine (latency, errors, CPU, memory)
- ✅ Full observability (metrics, traces, logs)
- ✅ DSL parser with validation
- ✅ Kubernetes manifest generation
- ✅ Gateway API support
- ✅ Multiple complex examples
- ✅ Comprehensive documentation

### Pending (Nice-to-Have)
- ⏳ Traffic generator manifests (k6 integration)
- ⏳ Time-based scenarios (automated chaos)
- ⏳ WebSocket protocol support
- ⏳ Custom dashboard generation (Grafana)
- ⏳ Helm chart output format
- ⏳ More sophisticated behavior patterns

## Use Cases for kkbase

1. **Topology Validation**: Generate apps that match production topologies to test kkbase's graph building
2. **Gateway API Testing**: Validate kkbase's HTTPRoute/GRPCRoute relationship tracking
3. **Multi-namespace**: Test cross-namespace service discovery and relationships
4. **Behavior Testing**: Inject failures to test kkbase's error tracking
5. **Scale Testing**: Deploy microservices mesh (15+ services) to test kkbase performance
6. **Protocol Testing**: Mixed HTTP/gRPC to validate kkbase's protocol detection
7. **Observability Integration**: Validate kkbase's metrics/traces/logs correlation

## Build & Deployment

### Local Development
```bash
make build          # Build binaries
make proto          # Regenerate protobuf
make clean          # Clean artifacts
make examples       # Generate all examples
```

### Docker
```bash
make docker-build   # Build testservice image
docker push your-registry/testservice:latest
```

### Production Considerations
- Container image size: ~20MB (alpine-based)
- Resource usage: 100m CPU, 128Mi memory (configurable)
- Startup time: <5 seconds
- Health checks: /health endpoint

## Summary

TestApp successfully provides:
1. ✅ A lightweight, multi-protocol test service
2. ✅ Complete observability stack integration
3. ✅ Flexible behavior modification
4. ✅ Simple YAML DSL for defining complex apps
5. ✅ Automatic Kubernetes + Gateway API manifest generation
6. ✅ Multiple realistic example applications
7. ✅ Call chain tracing for debugging

This gives kkbase a powerful tool for testing and validating its monitoring capabilities across diverse application topologies and scenarios.

