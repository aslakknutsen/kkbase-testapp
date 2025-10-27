# Jaeger Tracing Setup

Deploy Jaeger for distributed tracing and configure TestServices to send traces.

## Overview

TestServices have OpenTelemetry tracing built-in. They automatically send traces to the OTLP endpoint specified in the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable.

## Prerequisites

- Kubernetes cluster running
- Helm 3 installed
- kubectl configured

## Installation

### Deploy Jaeger

Create namespace:

```bash
kubectl create namespace observability
```

Add Helm repository:

```bash
helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
helm repo update
```

Install Jaeger:

```bash
helm upgrade --install jaeger jaegertracing/jaeger \
  --namespace observability \
  --values deploy/jaeger-values.yaml
```

Create OTLP service:

```bash
kubectl apply -f deploy/jaeger-otlp-service.yaml
```

Verify deployment:

```bash
kubectl get pods -n observability
kubectl get svc -n observability
```

Expected services:
- `jaeger-query` - UI (port 16686)
- `jaeger-collector-otlp` - OTLP endpoint (port 4317)

### Access Jaeger UI

```bash
kubectl port-forward -n observability svc/jaeger-query 16686:16686
```

Open: `http://localhost:16686`

### Deploy TestServices

Generated manifests automatically include the OTEL endpoint configuration:

```yaml
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: "jaeger-collector-otlp.observability.svc.cluster.local:4317"
```

Deploy your application:

```bash
kubectl apply -f output/simple-web/
```

## Viewing Traces

1. Access Jaeger UI at `http://localhost:16686`
2. Select service from dropdown (e.g., `frontend`, `api`)
3. Click "Find Traces"
4. Generate traffic to your services
5. View distributed traces

### Trace Information

Each trace shows:
- Span hierarchy (parent-child relationships)
- Timing (duration of each operation)
- Tags (service name, namespace, HTTP method, status)
- Span events (behaviors applied)

## Architecture

```
TestService → OTLP/gRPC (4317) → Jaeger Collector → Storage → Jaeger Query → UI (16686)
```

All TestService pods automatically send traces via OTLP/gRPC to the Jaeger collector.

## Troubleshooting

### No traces appearing

**Check Jaeger is running:**
```bash
kubectl get pods -n observability
kubectl logs -n observability deployment/jaeger
```

**Verify OTEL endpoint configured:**
```bash
kubectl get deployment frontend -o yaml | grep OTEL_EXPORTER_OTLP_ENDPOINT
```

**Test connectivity:**
```bash
kubectl exec -n default frontend-xxx -- nc -zv jaeger-collector-otlp.observability 4317
```

**Check service logs:**
```bash
kubectl logs frontend-xxx | grep -i "tracer\|otel\|telemetry"
```

Should see:
```json
{"level":"info","msg":"Telemetry initialized"}
```

**Generate traffic:**
```bash
kubectl port-forward svc/frontend 8080:8080
curl http://localhost:8080/
```

### Incomplete traces

Verify all services are running the same version and trace context is propagating. Check spans in Jaeger UI for missing services.

### High memory usage

Set resource limits in `deploy/jaeger-values.yaml`:

```yaml
allInOne:
  resources:
    limits:
      memory: 1Gi
    requests:
      memory: 512Mi
```

For production, use persistent storage (Elasticsearch, Cassandra).

### DNS resolution fails

Check CoreDNS and namespace:

```bash
kubectl get pods -n kube-system | grep coredns
kubectl get namespace observability
```

## Configuration

### Helm Values

`deploy/jaeger-values.yaml`:

```yaml
provisionDataStore:
  cassandra: false
  elasticsearch: false

allInOne:
  enabled: true
  image:
    repository: jaegertracing/all-in-one
    tag: "1.52"
  args:
    - "--collector.otlp.enabled=true"

storage:
  type: memory
```

### Service Endpoints

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| `jaeger-query` | 16686 | HTTP | UI |
| `jaeger-collector-otlp` | 4317 | gRPC | OTLP traces |
| `jaeger-collector-otlp` | 4318 | HTTP | OTLP HTTP |

## Advanced Configuration

### Sampling

Default is 100% sampling. To change, modify `pkg/service/telemetry/telemetry.go`:

```go
sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)) // 10% sampling
```

### Persistent Storage

For production, use Elasticsearch:

```yaml
provisionDataStore:
  elasticsearch: true

storage:
  type: elasticsearch
```

### Per-Namespace Jaeger

Deploy Jaeger instance per namespace and update generator to use local collector.

## Cleanup

```bash
helm uninstall jaeger -n observability
kubectl delete namespace observability
```

## See Also

- [Observability Concepts](../concepts/observability.md) - Metrics, traces, logs
- [Protocols](../concepts/protocols.md) - Trace propagation patterns
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/instrumentation/go/)

