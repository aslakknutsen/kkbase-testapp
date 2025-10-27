# Environment Variables Reference

Complete reference for TestService and TestGen environment variables.

## TestService Runtime Variables

Environment variables that configure TestService behavior at runtime.

### Service Identity

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SERVICE_NAME` | Yes | - | Service name |
| `SERVICE_VERSION` | No | "1.0.0" | Service version |
| `NAMESPACE` | No | "default" | Kubernetes namespace (from downward API) |
| `POD_NAME` | No | "" | Pod name (from downward API) |
| `NODE_NAME` | No | "" | Node name (from downward API) |

**Example:**
```yaml
env:
  - name: SERVICE_NAME
    value: "frontend"
  - name: SERVICE_VERSION
    value: "2.1.0"
  - name: NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
  - name: POD_NAME
    valueFrom:
      fieldRef:
        fieldPath: metadata.name
  - name: NODE_NAME
    valueFrom:
      fieldRef:
        fieldPath: spec.nodeName
```

### Server Ports

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `HTTP_PORT` | No | 8080 | HTTP server port |
| `GRPC_PORT` | No | 9090 | gRPC server port |
| `METRICS_PORT` | No | 9091 | Metrics endpoint port |

**Example:**
```yaml
env:
  - name: HTTP_PORT
    value: "8080"
  - name: GRPC_PORT
    value: "9090"
  - name: METRICS_PORT
    value: "9091"
```

### Upstream Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `UPSTREAMS` | No | "" | Comma-separated upstream services |

**Format:**
```
name:url,name2:url2
```

**With paths:**
```
name:url:path1,path2|name2:url2:path3
```

**Example:**
```yaml
env:
  - name: UPSTREAMS
    value: "api:http://api.backend:8080,cache:http://cache:8080"
```

**Complex example:**
```yaml
env:
  - name: UPSTREAMS
    value: "order-api:grpc://order-api.orders:9090:/orders,/cart|product-api:http://product-api.products:8080:/products"
```

### Behavior Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DEFAULT_BEHAVIOR` | No | "" | Default behavior string |

Applied to all requests unless overridden by query parameter.

**Example:**
```yaml
env:
  - name: DEFAULT_BEHAVIOR
    value: "latency=10-30ms"
```

### Observability

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | No | "" | OpenTelemetry collector endpoint |
| `LOG_LEVEL` | No | "info" | Log level: debug, info, warn, error |

**Example:**
```yaml
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "otel-collector:4317"
  - name: LOG_LEVEL
    value: "info"
```

### Client Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CLIENT_TIMEOUT_MS` | No | 30000 | Upstream call timeout in milliseconds |

**Example:**
```yaml
env:
  - name: CLIENT_TIMEOUT_MS
    value: "5000"
```

## Complete TestService Example

```yaml
env:
  # Identity
  - name: SERVICE_NAME
    value: "frontend"
  - name: SERVICE_VERSION
    value: "1.0.0"
  - name: NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
  - name: POD_NAME
    valueFrom:
      fieldRef:
        fieldPath: metadata.name
  - name: NODE_NAME
    valueFrom:
      fieldRef:
        fieldPath: spec.nodeName
  
  # Ports
  - name: HTTP_PORT
    value: "8080"
  - name: GRPC_PORT
    value: "9090"
  - name: METRICS_PORT
    value: "9091"
  
  # Upstreams
  - name: UPSTREAMS
    value: "api:http://api.backend:8080,cache:http://cache:8080"
  
  # Behavior
  - name: DEFAULT_BEHAVIOR
    value: "latency=10-30ms"
  
  # Observability
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "jaeger-collector-otlp.observability:4317"
  - name: LOG_LEVEL
    value: "info"
  
  # Client
  - name: CLIENT_TIMEOUT_MS
    value: "30000"
```

## TestGen CLI Variables

Environment variables that affect TestGen behavior.

### Image Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TESTSERVICE_IMAGE` | No | "testservice:latest" | TestService container image |

**Usage:**
```bash
export TESTSERVICE_IMAGE=myregistry/testservice:v2
./testgen generate examples/simple-web/app.yaml
```

Or use CLI flag:
```bash
./testgen generate examples/simple-web/app.yaml --image myregistry/testservice:v2
```

## Kubernetes Downward API

TestGen automatically configures the downward API to populate runtime information.

### Pod Information

```yaml
- name: POD_NAME
  valueFrom:
    fieldRef:
      fieldPath: metadata.name

- name: POD_NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace

- name: POD_IP
  valueFrom:
    fieldRef:
      fieldPath: status.podIP
```

### Node Information

```yaml
- name: NODE_NAME
  valueFrom:
    fieldRef:
      fieldPath: spec.nodeName

- name: NODE_IP
  valueFrom:
    fieldRef:
      fieldPath: status.hostIP
```

### Container Information

```yaml
- name: CPU_REQUEST
  valueFrom:
    resourceFieldRef:
      containerName: testservice
      resource: requests.cpu

- name: MEMORY_REQUEST
  valueFrom:
    resourceFieldRef:
      containerName: testservice
      resource: requests.memory
```

## Configuration Precedence

Configuration precedence (highest to lowest):

1. **Request-time behavior** (query param, header, gRPC field)
2. **DEFAULT_BEHAVIOR** environment variable
3. **DSL default behavior** (in app.yaml)
4. **No behavior** (default service response)

## Best Practices

**Do:**
- Use downward API for pod/namespace/node information
- Set OTEL endpoint for distributed tracing
- Configure reasonable timeouts
- Use semantic versioning for SERVICE_VERSION
- Set appropriate LOG_LEVEL (info for production, debug for troubleshooting)

**Don't:**
- Hardcode pod names or IPs
- Set very short CLIENT_TIMEOUT_MS (<1000ms)
- Use DEBUG log level in production
- Forget to configure UPSTREAMS

## Troubleshooting

### Service can't find upstreams

Check UPSTREAMS variable:
```bash
kubectl exec pod-name -- env | grep UPSTREAMS
```

### Traces not appearing

Check OTEL endpoint:
```bash
kubectl exec pod-name -- env | grep OTEL_EXPORTER_OTLP_ENDPOINT
```

Test connectivity:
```bash
kubectl exec pod-name -- nc -zv jaeger-collector-otlp.observability 4317
```

### Wrong service name in metrics

Check SERVICE_NAME:
```bash
kubectl exec pod-name -- env | grep SERVICE_NAME
```

## See Also

- [DSL Reference](dsl-spec.md) - Configure in YAML
- [Behavior Syntax](behavior-syntax.md) - DEFAULT_BEHAVIOR format
- [Observability](../concepts/observability.md) - OTEL configuration
- [Jaeger Setup](../guides/jaeger-setup.md) - Tracing configuration

