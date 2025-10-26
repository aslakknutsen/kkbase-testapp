# Jaeger Tracing Setup for TestServices

This guide explains how to deploy Jaeger in a Kind cluster and configure your TestServices to send distributed traces.

## Overview

Your TestServices already have OpenTelemetry tracing built-in via `pkg/service/telemetry/telemetry.go`. The services use OTLP/gRPC exporters and automatically read the trace collector endpoint from the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable.

## Prerequisites

- Kind cluster running
- Helm 3 installed
- kubectl configured to access your cluster

## Installation

### Step 1: Deploy Jaeger using Helm

Create the observability namespace:

```bash
kubectl create namespace observability
```

Add the Jaeger Helm repository:

```bash
helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
helm repo update
```

Install Jaeger using the provided values file:

```bash
helm upgrade --install jaeger jaegertracing/jaeger \
  --namespace observability \
  --values deploy/jaeger-values.yaml
```

Create a service to expose OTLP ports (required because the Helm chart doesn't expose OTLP ports by default):

```bash
kubectl apply -f deploy/jaeger-otlp-service.yaml
```

**Note**: The `jaeger-otlp-service.yaml` creates a ClusterIP service that exposes ports 4317 (OTLP/gRPC) and 4318 (OTLP/HTTP) from the all-in-one Jaeger pod.

Verify the deployment:

```bash
kubectl get pods -n observability
kubectl get svc -n observability
```

You should see:
- `jaeger-xxx` pod running (all-in-one)
- `jaeger-collector-xxx` pod running
- `jaeger-query-xxx` pod running
- `jaeger-query` service (UI access)
- `jaeger-collector-otlp` service (OTLP endpoint on port 4317)

### Step 2: Access the Jaeger UI

Port-forward the Jaeger UI service to your local machine:

```bash
kubectl port-forward -n observability svc/jaeger-query 16686:16686
```

Open your browser and navigate to: `http://localhost:16686`

### Step 3: Deploy Your TestServices

The generated manifests already include the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable pointing to `jaeger-collector.observability.svc.cluster.local:4317`.

Deploy your services:

```bash
# For ecommerce example
kubectl apply -f output/ecommerce/ecommerce/

# For simple-web example
kubectl apply -f output/simple-web/simple-web/
```

## Viewing Traces

1. Access the Jaeger UI at `http://localhost:16686`
2. Select a service from the "Service" dropdown (e.g., `web`, `order-api`, `product-api`)
3. Click "Find Traces" button
4. Generate some traffic to your services (use ingress or port-forward)
5. Refresh the traces list to see distributed traces

### Understanding Trace Data

Each trace shows:
- **Span hierarchy**: Parent-child relationships between service calls
- **Timing**: Duration of each operation
- **Tags**: Service name, namespace, HTTP method, status codes, etc.
- **Logs**: Any logged events during the request
- **Baggage**: Context propagated across services

## Architecture

### Service Communication

```
┌─────────────────┐
│   Jaeger UI     │ :16686
└────────┬────────┘
         │
┌────────▼────────────────────┐
│  jaeger-query service       │
└─────────────────────────────┘

┌─────────────────────────────┐
│  jaeger-collector service   │ :4317 (OTLP/gRPC)
└────────┬────────────────────┘
         │
    ┌────▼─────┐
    │  Jaeger  │
    │  Storage │ (in-memory)
    └──────────┘
```

### Trace Flow

```
TestService → OTLP/gRPC → Jaeger Collector → Storage → Jaeger Query → UI
```

### Environment Variable

All TestService pods automatically receive:

```yaml
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: "jaeger-collector-otlp.observability.svc.cluster.local:4317"
```

This is configured in `pkg/generator/k8s/generator.go` and automatically added to all generated manifests.

## Troubleshooting

### No traces appearing in Jaeger UI

**Check 1: Verify Jaeger is running**

```bash
kubectl get pods -n observability
kubectl logs -n observability deployment/jaeger
```

**Check 2: Verify services have OTEL endpoint configured**

```bash
kubectl get deployment -n <namespace> <service-name> -o yaml | grep OTEL_EXPORTER_OTLP_ENDPOINT
```

You should see:
```yaml
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: "jaeger-collector-otlp.observability.svc.cluster.local:4317"
```

**Check 3: Verify network connectivity**

Test connectivity from a service pod to Jaeger:

```bash
kubectl exec -n <namespace> <pod-name> -- nc -zv jaeger-collector.observability.svc.cluster.local 4317
```

**Check 4: Check service logs**

Look for telemetry initialization errors:

```bash
kubectl logs -n <namespace> <pod-name> | grep -i "tracer\|otel\|telemetry"
```

Successful initialization should show:
```json
{"level":"info","service":"<service-name>","message":"Telemetry initialized"}
```

**Check 5: Verify traffic is being generated**

Make sure you're actually sending requests to your services:

```bash
# Port-forward to a service
kubectl port-forward -n <namespace> svc/<service-name> 8080:8080

# Make a request
curl http://localhost:8080/health
```

### Traces are incomplete

**Issue**: Some spans are missing from the trace chain.

**Solution**: Verify that trace context is being propagated correctly:

1. Check that all services are running the latest version with tracing enabled
2. For HTTP: Ensure `traceparent` headers are being propagated
3. For gRPC: Ensure metadata is being propagated (handled automatically by `pkg/service/grpc/trace.go`)

### High memory usage

**Issue**: Jaeger all-in-one using too much memory.

**Solution**: The all-in-one deployment uses in-memory storage. For production or long-running tests, consider:

1. Setting memory limits in `deploy/jaeger-values.yaml`:
   ```yaml
   allInOne:
     resources:
       limits:
         memory: 1Gi
       requests:
         memory: 512Mi
   ```

2. Using persistent storage (Elasticsearch, Cassandra) for production setups

### DNS resolution fails

**Issue**: Services cannot resolve `jaeger-collector.observability.svc.cluster.local`

**Solution**: 

1. Verify CoreDNS is running:
   ```bash
   kubectl get pods -n kube-system | grep coredns
   ```

2. Test DNS resolution from a pod:
   ```bash
   kubectl exec -n <namespace> <pod-name> -- nslookup jaeger-collector.observability.svc.cluster.local
   ```

3. Ensure the observability namespace exists:
   ```bash
   kubectl get namespace observability
   ```

## Configuration Reference

### Helm Values (`deploy/jaeger-values.yaml`)

```yaml
provisionDataStore:
  cassandra: false    # Don't provision Cassandra
  elasticsearch: false # Don't provision Elasticsearch
  kafka: false        # Don't provision Kafka

allInOne:
  enabled: true       # Use all-in-one deployment (collector+query+storage)
  image:
    repository: jaegertracing/all-in-one
    tag: "1.52"
    pullPolicy: IfNotPresent
  args:
    - "--collector.otlp.enabled=true"  # Enable OTLP receiver
  extraEnv:
    - name: COLLECTOR_OTLP_ENABLED
      value: "true"

storage:
  type: memory        # Use in-memory storage for testing
```

### Service Endpoints

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| `jaeger-query` | 16686 | HTTP | Jaeger UI |
| `jaeger-collector-otlp` | 4317 | gRPC | OTLP traces (used by TestServices) |
| `jaeger-collector-otlp` | 4318 | HTTP | OTLP traces (alternative) |
| `jaeger-collector` | 14250 | gRPC | Jaeger native format |
| `jaeger-agent` | 6831 | UDP | Jaeger Thrift compact |

## Advanced Configuration

### Sampling

By default, all traces are sampled (100% sampling). To change this, modify `pkg/service/telemetry/telemetry.go`:

```go
tp := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(exporter),
    sdktrace.WithResource(res),
    sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)), // 10% sampling
)
```

### Using Persistent Storage

For production deployments, use Elasticsearch or Cassandra:

```yaml
# deploy/jaeger-values.yaml
provisionDataStore:
  cassandra: true

storage:
  type: cassandra
  cassandra:
    host: cassandra
    port: 9042
```

### Multiple Jaeger Instances

To deploy Jaeger per namespace instead of shared:

1. Change the namespace in `deploy/jaeger-values.yaml`
2. Update `pkg/generator/k8s/generator.go` to use the service's own namespace:
   ```go
   b.WriteString(fmt.Sprintf("          value: \"jaeger-collector.%s.svc.cluster.local:4317\"\n", svc.Namespace))
   ```

## Cleanup

To remove Jaeger:

```bash
helm uninstall jaeger -n observability
kubectl delete namespace observability
```

## References

- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/instrumentation/go/)
- [OTLP Specification](https://opentelemetry.io/docs/reference/specification/protocol/)
- [Jaeger Helm Chart](https://github.com/jaegertracing/helm-charts)

