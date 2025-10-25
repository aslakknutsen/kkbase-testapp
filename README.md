# TestApp - Synthetic Application Generator

TestApp is a tool for generating complex, realistic Kubernetes applications for testing monitoring systems, service meshes, and platform tools. It creates both the application runtime (TestService) and all necessary Kubernetes/Gateway API manifests from a simple YAML DSL.

## Features

### TestService Binary
- **Multi-protocol support**: HTTP and gRPC
- **Configurable behavior**: Latency, error injection, CPU/memory patterns
- **Call chain tracing**: Returns complete call chain with timing
- **Full observability**: OpenTelemetry traces, Prometheus metrics, structured logs
- **Upstream dependencies**: Automatically calls configured upstream services

### DSL & Generator
- **Simple YAML DSL**: Define complex applications declaratively
- **Kubernetes manifests**: Deployments, StatefulSets, DaemonSets, Services
- **Gateway API**: HTTPRoute, GRPCRoute, Gateway, TLS certificates
- **Validation**: Circular dependency detection, reference validation
- **Multiple examples**: Simple web app to complex microservices mesh

## Quick Start

### 1. Build the Tools

```bash
# Build TestService
cd cmd/testservice
go build -o ../../testservice

# Build TestGen
cd ../testgen
go build -o ../../testgen
```

### 2. Generate an Example Application

```bash
# Generate manifests from the simple-web example
./testgen generate examples/simple-web/app.yaml -o ./output

# Review generated manifests
ls -R output/simple-web/
```

### 3. Deploy to Kubernetes

```bash
# Apply all manifests
kubectl apply -f output/simple-web/

# Check status
kubectl get pods
kubectl get httproute
kubectl get gateway
```

### 4. Test the Application

```bash
# Port forward to the frontend service
kubectl port-forward svc/frontend 8080:8080

# Make a request
curl http://localhost:8080/

# With behavior directives
curl 'http://localhost:8080/?behavior=latency=500ms,error=0.5'
```

## DSL Reference

### Basic Structure

```yaml
app:
  name: my-app
  namespaces:
    - default
    - backend

services:
  - name: frontend
    namespace: default
    replicas: 2
    type: Deployment  # or StatefulSet, DaemonSet
    protocols: [http, grpc]
    upstreams: [backend]
    ingress:
      enabled: true
      host: myapp.local
      tls: true
    behavior:
      latency: "10-50ms"
      errorRate: 0.02
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
      limits:
        cpu: "500m"
        memory: "512Mi"

traffic:
  - name: load-gen
    target: frontend
    rate: "100/s"
    pattern: steady
```

### Service Configuration

#### Basic Fields
- `name`: Service name (required)
- `namespace`: Kubernetes namespace (default: "default")
- `replicas`: Number of replicas (default: 1, ignored for DaemonSet)
- `type`: Workload type - `Deployment`, `StatefulSet`, or `DaemonSet`
- `protocols`: List of protocols - `http` and/or `grpc`

#### Ports
```yaml
ports:
  http: 8080
  grpc: 9090
  metrics: 9091
```

#### Upstreams
List of service names to call when this service receives a request:
```yaml
upstreams:
  - backend-api
  - database
```

#### Behavior
Default behavior for all requests to this service:
```yaml
behavior:
  latency: "10-50ms"      # Range of latency
  errorRate: 0.05         # 5% error rate
  cpu: "spike:5s:80"      # CPU pattern
  memory: "leak-slow:10m" # Memory pattern
```

#### Storage (for StatefulSets)
```yaml
storage:
  size: 10Gi
  storageClass: fast-ssd
```

#### Ingress
```yaml
ingress:
  enabled: true
  host: myapp.example.com
  tls: true
  paths:
    - /
    - /api/v1
```

#### Resources
```yaml
resources:
  requests:
    cpu: "100m"
    memory: "128Mi"
  limits:
    cpu: "1000m"
    memory: "1Gi"
```

### Traffic Generation

```yaml
traffic:
  - name: load-generator
    type: load-generator
    target: frontend
    rate: "100/s"
    pattern: steady  # or spiky, diurnal
    duration: "1h"   # 0 = continuous
```

## Behavior Syntax

TestService supports runtime behavior modification via query parameters or headers:

### Query Parameter
```bash
curl 'http://service:8080/?behavior=latency=200ms,error=503:0.1'
```

### Header
```bash
curl -H 'X-Behavior: latency=200ms,error=503:0.1' http://service:8080/
```

### Behavior Primitives

#### Latency
- `latency=100ms` - Fixed 100ms delay
- `latency=50-200ms` - Random delay between 50-200ms

#### Errors
- `error=503` - Always return 503
- `error=0.1` - 10% probability of 500 error
- `error=503:0.2` - 20% probability of 503 error

#### CPU Load
- `cpu=spike` - CPU spike for 5s at 80% intensity
- `cpu=spike:10s:90` - CPU spike for 10s at 90% intensity
- `cpu=steady:30s:50` - Steady 50% CPU for 30s

#### Memory
- `memory=leak-slow` - Slow memory leak (10MB over 10m)
- `memory=leak-slow:5m` - Slow leak over 5 minutes
- `memory=leak-fast` - Fast memory leak

## Response Format

TestService returns a JSON response with complete call chain:

```json
{
  "service": {
    "name": "frontend",
    "version": "1.0.0",
    "namespace": "default",
    "pod": "frontend-7d8f9c-xyz",
    "node": "node-1",
    "protocol": "http"
  },
  "start_time": "2025-10-23T12:00:00.000Z",
  "end_time": "2025-10-23T12:00:00.150Z",
  "duration": "150ms",
  "code": 200,
  "body": "Hello from frontend",
  "trace_id": "abc123...",
  "span_id": "def456...",
  "upstream_calls": [
    {
      "name": "backend",
      "uri": "http://backend:8080",
      "protocol": "http",
      "duration": "100ms",
      "code": 200,
      "upstream_calls": [...]
    }
  ],
  "behaviors_applied": ["latency:range", "error:503:0.10"]
}
```

## Observability

### Metrics (Prometheus)

TestService exposes metrics on the `/metrics` endpoint (default port 9091):

- `testservice_requests_total` - Total requests by service, method, status, protocol
- `testservice_request_duration_seconds` - Request duration histogram
- `testservice_upstream_calls_total` - Upstream calls by service, upstream, status
- `testservice_upstream_duration_seconds` - Upstream duration histogram
- `testservice_active_requests` - Current active requests
- `testservice_behavior_applied_total` - Behaviors applied by type

### Traces (OpenTelemetry)

Set the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable to send traces to your collector:

```yaml
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "otel-collector:4317"
```

Traces include:
- Parent span for each request
- Child spans for upstream calls
- Span events for behaviors applied
- Full W3C trace context propagation

### Logs (Structured JSON)

All logs are JSON-formatted with fields:
- `service`, `namespace`, `pod`, `node`
- `trace_id`, `span_id`
- `level`, `msg`, `timestamp`
- Custom fields per log entry

## Examples

### Simple Web App
3-tier application: frontend → api → database

```bash
./testgen generate examples/simple-web/app.yaml
```

### E-Commerce
Multi-namespace application with mixed HTTP/gRPC protocols:
- Frontend namespace: web service
- Orders namespace: order-api (gRPC), order-db (StatefulSet)
- Products namespace: product-api, product-db, cache
- Payments namespace: payment-api, payment-db

```bash
./testgen generate examples/ecommerce/app.yaml
```

### Microservices Mesh
Complex topology with 15+ services across 3 namespaces demonstrating:
- Multiple ingress points
- Mixed protocols
- Deep call chains
- Various StatefulSet databases

```bash
./testgen generate examples/microservices/app.yaml
```

## TestGen CLI

### Commands

#### Generate
```bash
testgen generate <dsl-file> [flags]

Flags:
  -o, --output-dir string   Output directory (default "./output")
  --validate-only          Only validate, don't generate
  -i, --image string       TestService image (default "testservice:latest")
```

#### Validate
```bash
testgen validate <dsl-file>
```

#### Init
Create a new application template:
```bash
testgen init my-app
# Creates my-app.yaml
```

#### Examples
List available examples:
```bash
testgen examples
```

## Environment Variables

### TestService

- `SERVICE_NAME` - Service name
- `SERVICE_VERSION` - Version (default: "1.0.0")
- `HTTP_PORT` - HTTP port (default: 8080)
- `GRPC_PORT` - gRPC port (default: 9090)
- `METRICS_PORT` - Metrics port (default: 9091)
- `UPSTREAMS` - Comma-separated upstreams: `api:http://api:8080,db:http://db:5432`
- `DEFAULT_BEHAVIOR` - Default behavior string
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OpenTelemetry collector endpoint
- `LOG_LEVEL` - Log level: debug, info, warn, error (default: info)
- `CLIENT_TIMEOUT_MS` - Upstream call timeout in ms (default: 30000)

Downward API variables (auto-populated):
- `NAMESPACE` - Pod namespace
- `POD_NAME` - Pod name
- `NODE_NAME` - Node name

## Building Container Images

### TestService

```bash
cd testapp
docker build -t testservice:latest -f Dockerfile.testservice .
```

### TestGen (optional)

```bash
docker build -t testgen:latest -f Dockerfile.testgen .
```

## Architecture

```
┌─────────────┐
│  DSL File   │
│  (YAML)     │
└──────┬──────┘
       │
       v
┌──────────────────┐
│   Parser         │
│   - Validation   │
│   - Defaults     │
└──────┬───────────┘
       │
       v
┌──────────────────┐
│  Generators      │
│  - K8s          │
│  - Gateway API   │
└──────┬───────────┘
       │
       v
┌──────────────────┐
│   Manifests      │
│   - Deployment   │
│   - Service      │
│   - HTTPRoute    │
│   - etc.         │
└──────────────────┘

Applied to K8s:
┌─────────────────────────────────┐
│  TestService Pods               │
│  - HTTP/gRPC servers            │
│  - Behavior engine              │
│  - Upstream client              │
│  - Telemetry (metrics/traces)   │
└─────────────────────────────────┘
```

## Use Cases

1. **Testing Monitoring Systems**: Generate realistic application topologies to validate your monitoring setup
2. **Service Mesh Validation**: Test Istio, Linkerd, or Gateway API configurations
3. **Performance Testing**: Create load scenarios with configurable error rates and latency
4. **Chaos Engineering**: Use behavior directives to inject failures
5. **Training**: Demonstrate Kubernetes concepts with real applications
6. **Development**: Test platform tools without needing actual applications

## Contributing

Contributions welcome! Areas for enhancement:
- Additional behavior primitives
- More sophisticated traffic generation
- Scenario automation
- Additional protocol support (WebSocket, etc.)
- Enhanced observability features

## License

Apache 2.0

