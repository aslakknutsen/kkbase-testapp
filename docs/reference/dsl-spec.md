# DSL Specification

Complete reference for the TestApp YAML DSL.

## Document Structure

```yaml
app:
  name: string             # Application name (required)
  namespaces: []string     # Kubernetes namespaces to create
  providers: Provider      # Ingress and mesh providers (optional)
  meshDefaults: MeshConfig # Default mesh configuration (optional)

services: []Service        # List of services (required)

traffic: []TrafficGen      # Traffic generators (optional)

scenarios: []Scenario      # Time-based scenarios (optional, future)
```

## App Section

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Application name, used for labels |
| `namespaces` | []string | No | Kubernetes namespaces to create |
| `providers` | ProviderConfig | No | Ingress and mesh provider configuration |
| `meshDefaults` | MeshConfig | No | Default mesh settings for all services |

### Example

```yaml
app:
  name: my-application
  namespaces:
    - default
    - backend
    - database
  providers:
    ingress: gateway-api
    mesh: istio
  meshDefaults:
    timeout: 5s
    retries:
      attempts: 3
      perTryTimeout: 1s
    circuitBreaker:
      consecutiveErrors: 5
    loadBalancing: ROUND_ROBIN
```

## Provider Configuration

Defines which providers to use for ingress and service mesh functionality.

### Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `ingress` | string | No | "gateway-api" | Ingress provider: `gateway-api`, `istio-gateway`, `k8s-ingress`, `openshift-routes`, `none` |
| `mesh` | string | No | "" | Mesh provider: `istio`, `linkerd`, `gateway-api-mesh`, `none` |

### Example

```yaml
providers:
  ingress: gateway-api  # Use Gateway API for ingress
  mesh: istio          # Use Istio for service mesh
```

**Provider Options:**

**Ingress Providers:**
- `gateway-api` - Kubernetes Gateway API (default)
- `istio-gateway` - Istio Gateway + VirtualService
- `k8s-ingress` - Traditional Kubernetes Ingress
- `openshift-routes` - OpenShift Routes
- `none` - No ingress resources generated

**Mesh Providers:**
- `istio` - Istio service mesh (VirtualService, DestinationRule)
- `linkerd` - Linkerd service mesh (future)
- `gateway-api-mesh` - Gateway API for mesh routing (future)
- `none` or empty - No mesh resources generated

## Mesh Configuration

Default mesh configuration applied to all services unless overridden at service level.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `timeout` | string | No | Request timeout (e.g., "5s") |
| `retries` | RetryConfig | No | Retry policy configuration |
| `circuitBreaker` | CircuitBreakerConfig | No | Circuit breaker settings |
| `loadBalancing` | string | No | Load balancing algorithm |
| `trafficSplit` | []TrafficSplit | No | Traffic splitting for canary deployments |
| `mtls` | string | No | mTLS mode (provider-specific) |

### Retry Configuration

| Field | Type | Description |
|-------|------|-------------|
| `attempts` | int | Number of retry attempts |
| `perTryTimeout` | string | Timeout per retry attempt |
| `retryOn` | string | Conditions to retry on (default: "5xx,reset,connect-failure,refused-stream") |

### Circuit Breaker Configuration

| Field | Type | Description |
|-------|------|-------------|
| `consecutiveErrors` | int | Number of consecutive errors before circuit opens |
| `interval` | string | Interval for checking errors (e.g., "30s") |
| `baseEjectionTime` | string | Minimum ejection duration (e.g., "30s") |
| `maxEjectionPercent` | int | Maximum percentage of hosts that can be ejected |

### Traffic Split Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | string | Yes | Version identifier |
| `weight` | int | Yes | Traffic weight percentage (0-100) |
| `subset` | string | No | Subset name for routing |

### Example

```yaml
meshDefaults:
  timeout: 5s
  retries:
    attempts: 3
    perTryTimeout: 1s
    retryOn: "5xx,reset,connect-failure"
  circuitBreaker:
    consecutiveErrors: 5
    interval: 30s
    baseEjectionTime: 30s
    maxEjectionPercent: 50
  loadBalancing: ROUND_ROBIN  # ROUND_ROBIN, LEAST_REQUEST, RANDOM, PASSTHROUGH
  mtls: STRICT                # STRICT, PERMISSIVE, DISABLE
```

## Service Definition

### Core Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | - | Service name |
| `namespace` | string | No | "default" | Kubernetes namespace |
| `type` | string | No | "Deployment" | Workload type: `Deployment`, `StatefulSet`, `DaemonSet` |
| `replicas` | int | No | 1 | Number of replicas (ignored for DaemonSet) |
| `protocols` | []string | No | ["http"] | Protocols: `http`, `grpc` |
| `mesh` | MeshConfig | No | - | Service-level mesh configuration (overrides app defaults) |

### Example

```yaml
services:
  - name: frontend
    namespace: default
    type: Deployment
    replicas: 2
    protocols: [http]
    mesh:
      timeout: 10s  # Override app default
      retries:
        attempts: 5
```

## Ports Configuration

### Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `http` | int | No | 8080 | HTTP server port |
| `grpc` | int | No | 9090 | gRPC server port |
| `metrics` | int | No | 9091 | Metrics endpoint port |

### Example

```yaml
services:
  - name: api
    ports:
      http: 8080
      grpc: 9090
      metrics: 9091
```

## Upstreams Configuration

### Simple Format

List of service names:

```yaml
upstreams:
  - api
  - cache
  - database
```

### Advanced Format

Explicit routing with path matching, forward paths, and weighted groups:

```yaml
upstreams:
  - name: order-api           # Unique ID (also used as service name if service omitted)
    match: [/orders, /cart]   # Incoming paths that trigger this upstream
  - name: payment-ok          # Unique ID for behavior targeting
    service: message-bus      # Target service name
    path: /events/PaymentOK   # Forward path to call
    group: payment-outcome    # Weighted selection group
  - name: payment-fail
    service: message-bus
    path: /events/PaymentFail
    group: payment-outcome
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique ID for this upstream entry (used for behavior targeting) |
| `service` | string | No | Target service name (defaults to `name`) |
| `match` | []string | No | Incoming HTTP path prefixes that trigger this upstream |
| `path` | string | No | Explicit forward path to call on upstream |
| `group` | string | No | Weighted selection group - upstreams in same group are mutually exclusive |
| `probability` | float | No | Independent call probability (0.0-1.0), only for ungrouped upstreams |

### Weighted Groups

Upstreams with the same `group` are mutually exclusive - only one is called per request, selected based on weights.

Default: equal distribution within group. Override via behavior:

```yaml
behavior:
  upstreamWeights:
    payment-ok: 85
    payment-fail: 15
```

Or at runtime: `?behavior=upstreamWeights=payment-ok:85;payment-fail:15`

### URL Generation

URLs are auto-generated based on the target service:

- HTTP: `http://<service>.<namespace>.svc.cluster.local:<http-port>`
- gRPC: `grpc://<service>.<namespace>.svc.cluster.local:<grpc-port>`

## Behavior Configuration

Default behaviors applied to all requests for this service.

### Fields

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `latency` | string | Fixed or range latency | `"10ms"`, `"10-50ms"` |
| `errorRate` | float | Error probability (0.0-1.0) | `0.05` (5%) |
| `cpu` | string | CPU pattern | `"spike:5s:80"` |
| `memory` | string | Memory pattern | `"leak-slow:10m"` |
| `upstreamWeights` | map[string]int | Weights for grouped upstreams | see below |

### Example

```yaml
services:
  - name: api
    behavior:
      latency: "10-50ms"
      errorRate: 0.02
```

### Upstream Weights

For services with grouped upstreams, specify weights:

```yaml
services:
  - name: payment
    upstreams:
      - name: payment-ok
        service: message-bus
        path: /events/PaymentOK
        group: outcome
      - name: payment-fail
        service: message-bus
        path: /events/PaymentFail
        group: outcome
    behavior:
      upstreamWeights:
        payment-ok: 85
        payment-fail: 15
```

Weights are relative. Unspecified upstreams in a group share remaining weight equally.

## Storage Configuration

For StatefulSets only.

### Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `size` | string | No | "1Gi" | PVC size |
| `storageClass` | string | No | "" | StorageClass name |

### Example

```yaml
services:
  - name: database
    type: StatefulSet
    storage:
      size: 10Gi
      storageClass: fast-ssd
```

## Ingress Configuration

Configure Gateway API resources (HTTPRoute/GRPCRoute).

### Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | bool | No | false | Enable ingress |
| `host` | string | No | "" | Hostname for routing |
| `tls` | bool | No | false | Enable TLS (self-signed cert) |
| `paths` | []string | No | ["/"] | HTTP path prefixes |

### Example

```yaml
services:
  - name: frontend
    ingress:
      enabled: true
      host: myapp.example.com
      tls: true
      paths:
        - /
        - /api/v1
```

## Resources Configuration

Kubernetes resource requests and limits.

### Fields

```yaml
resources:
  requests:
    cpu: string      # e.g., "100m"
    memory: string   # e.g., "128Mi"
  limits:
    cpu: string      # e.g., "1000m"
    memory: string   # e.g., "1Gi"
```

### Example

```yaml
services:
  - name: api
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
      limits:
        cpu: "500m"
        memory: "512Mi"
```

## Labels Configuration

Custom labels for Kubernetes resources.

```yaml
services:
  - name: payment-api
    labels:
      team: payments
      tier: backend
      pii: "true"
```

## Service-Level Mesh Configuration

Services can override app-level mesh defaults or disable mesh entirely.

### Override Defaults

```yaml
services:
  - name: api-gateway
    mesh:
      timeout: 10s          # Override default
      retries:
        attempts: 5         # Override default
        perTryTimeout: 2s
      circuitBreaker:
        consecutiveErrors: 10  # More lenient than default
```

### Disable Mesh

```yaml
services:
  - name: legacy-service
    mesh:
      enabled: false  # Explicitly opt out of mesh
```

### Traffic Splitting (Canary Deployments)

```yaml
services:
  - name: frontend
    mesh:
      trafficSplit:
        - version: v1
          weight: 90
          subset: v1
        - version: v2
          weight: 10
          subset: v2
```

**Mesh Behavior:**
- If `app.providers.mesh` is set (e.g., "istio"), mesh is **auto-enabled** for all services
- Services inherit `app.meshDefaults` configuration
- Service-level `mesh` config overrides specific fields from defaults
- Set `mesh.enabled: false` to explicitly opt out a service
- If no mesh provider is configured, mesh settings are ignored

## Complete Service Example

```yaml
services:
  - name: order-api
    namespace: orders
    type: Deployment
    replicas: 3
    protocols: [http, grpc]
    
    ports:
      http: 8080
      grpc: 9090
      metrics: 9091
    
    upstreams:
      - name: product-api
        url: http://product-api.products:8080
      - name: payment-api
        url: grpc://payment-api.payments:9090
      - name: order-db
    
    behavior:
      latency: "10-50ms"
      errorRate: 0.01
    
    mesh:
      timeout: 10s
      retries:
        attempts: 5
        perTryTimeout: 2s
      circuitBreaker:
        consecutiveErrors: 10
    
    resources:
      requests:
        cpu: "200m"
        memory: "256Mi"
      limits:
        cpu: "1000m"
        memory: "1Gi"
    
    labels:
      team: orders
      tier: backend
    
    ingress:
      enabled: true
      host: api.example.com
      tls: true
      paths: [/api/v1/orders]
```

## Traffic Generation

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Traffic generator name |
| `type` | string | Generator type |
| `target` | string | Target service name |
| `rate` | string | Request rate (e.g., "100/s") |
| `pattern` | string | Traffic pattern: `steady`, `spiky`, `diurnal` |
| `duration` | string | Duration (0 = continuous) |
| `paths` | []string | List of URL paths to call (optional) |
| `pathPattern` | string | How to distribute across paths: `round-robin` (default), `random`, `sequential` |
| `behavior` | string | Behavior injection query parameter (optional) |

### Examples

**Single Path:**
```yaml
traffic:
  - name: load-gen
    target: frontend
    rate: "100/s"
    pattern: steady
    duration: "1h"
```

**Multiple Paths with Round-Robin:**
```yaml
traffic:
  - name: api-load
    target: api-gateway
    rate: "200/s"
    pattern: steady
    duration: "2h"
    paths:
      - /api/v1/products
      - /api/v1/cart
      - /api/v1/checkout
    pathPattern: round-robin
```

**Random Path Selection:**
```yaml
traffic:
  - name: random-load
    target: api-gateway
    rate: "100/s"
    pattern: spiky
    duration: "1h"
    paths:
      - /api/v1/search
      - /api/v1/reviews
    pathPattern: random
```

**With Behavior Injection:**
```yaml
traffic:
  - name: chaos-load
    target: frontend
    rate: "50/s"
    pattern: steady
    duration: "1h"
    behavior: "latency=500ms,error=0.1"
```

The `behavior` field injects runtime behaviors into the generated traffic. The behavior string is appended as a query parameter to the target URL and propagates through the entire call chain. This enables testing of cascading failures, latency injection, and error scenarios without requiring in-process load generation.

### Implementation Details

Traffic generation is implemented using [Fortio](https://github.com/fortio/fortio), a load testing tool designed for service mesh testing.

**Generated Resources:**
- Kubernetes Job that runs Alpine Linux with Fortio binary
- ConfigMap containing pattern-specific wrapper scripts
- Jobs are created in the target service's namespace
- Fortio binary is downloaded at Job startup for maximum compatibility

**Pattern Behaviors:**

| Pattern | Behavior | Use Case |
|---------|----------|----------|
| `steady` | Constant rate throughout duration | Baseline performance testing |
| `spiky` | Alternates between 3x bursts (5s) and 0.2x baseline (25s) | Testing autoscaling and resilience |
| `diurnal` | 24-hour sine wave: peak during business hours (9am-5pm), low at night | Production-like traffic simulation |

**Target Resolution:**
- Automatically constructs service URLs: `{protocol}://{service}.{namespace}.svc.cluster.local:{port}`
- Protocol (HTTP/gRPC) and port determined from target service configuration
- Jobs run within the cluster for accurate service mesh testing

**Path Distribution:**
- **round-robin** (default): Distributes rate evenly across all paths in parallel
- **random**: Randomly selects a path for each request interval
- **sequential**: Cycles through paths in order, one at a time
- If no `paths` specified, calls the root URL

**Example Generated Job:**
```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: api-traffic
  namespace: sf-gateway
spec:
  ttlSecondsAfterFinished: 300
  template:
    spec:
      containers:
      - name: load-generator
        image: alpine:latest
        command: ["/bin/sh", "-c"]
        args:
          - |
            cd /tmp
            wget -q https://github.com/fortio/fortio/releases/download/v1.73.0/fortio-linux_amd64-1.73.0.tgz
            tar -xzf fortio-linux_amd64-1.73.0.tgz
            mv usr/bin/fortio /usr/local/bin/fortio
            chmod +x /usr/local/bin/fortio
            /bin/sh /scripts/run.sh
```

## Validation Rules

### Service Names
- Must be valid Kubernetes names
- Alphanumeric, hyphens, lowercase
- Max 63 characters

### Namespace References
- All namespaces must be declared in `app.namespaces`
- Cross-namespace upstream references require namespace specification

### Upstream References
- Referenced services must exist in `services` list
- Circular dependencies are detected and rejected

### Protocol Compatibility
- Services must declare protocols they support
- Upstreams must match available protocols

### StatefulSet Requirements
- Must have `storage` configuration
- `replicas` must be specified

### DaemonSet Requirements
- `replicas` field is ignored

## See Also

- [CLI Reference](cli-reference.md) - Using testgen commands
- [Behavior Syntax](behavior-syntax.md) - Behavior string format
- [Environment Variables](environment-variables.md) - Runtime configuration
- [Examples](../../examples/) - Complete DSL examples

