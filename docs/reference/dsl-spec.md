# DSL Specification

Complete reference for the TestApp YAML DSL.

## Document Structure

```yaml
app:
  name: string           # Application name (required)
  namespaces: []string   # Kubernetes namespaces to create

services: []Service      # List of services (required)

traffic: []TrafficGen    # Traffic generators (optional)

scenarios: []Scenario    # Time-based scenarios (optional, future)
```

## App Section

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Application name, used for labels |
| `namespaces` | []string | No | Kubernetes namespaces to create |

### Example

```yaml
app:
  name: my-application
  namespaces:
    - default
    - backend
    - database
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

### Example

```yaml
services:
  - name: frontend
    namespace: default
    type: Deployment
    replicas: 2
    protocols: [http]
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

### Advanced Format (with URLs)

Explicit URLs and optional path routing:

```yaml
upstreams:
  - name: order-api
    url: grpc://order-api.orders.svc.cluster.local:9090
    paths: [/orders, /cart]
  - name: product-api
    url: http://product-api.products.svc.cluster.local:8080
    paths: [/products]
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Upstream service name |
| `url` | string | No | Full URL (auto-generated if omitted) |
| `paths` | []string | No | HTTP path prefixes for routing |

### URL Generation

If `url` is omitted, it's generated as:

- HTTP: `http://<name>.<namespace>.svc.cluster.local:<http-port>`
- gRPC: `grpc://<name>.<namespace>.svc.cluster.local:<grpc-port>`

## Behavior Configuration

Default behaviors applied to all requests for this service.

### Fields

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `latency` | string | Fixed or range latency | `"10ms"`, `"10-50ms"` |
| `errorRate` | float | Error probability (0.0-1.0) | `0.05` (5%) |
| `cpu` | string | CPU pattern | `"spike:5s:80"` |
| `memory` | string | Memory pattern | `"leak-slow:10m"` |

### Example

```yaml
services:
  - name: api
    behavior:
      latency: "10-50ms"
      errorRate: 0.02
      cpu: "200m"
```

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

