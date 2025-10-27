# Quick Start Guide

Get TestApp running in 5 minutes.

## Prerequisites

- Go 1.21 or later
- Protocol Buffer compiler (`protoc`)
- Docker (optional, for building images)
- Kubernetes cluster (for deployment)
- kubectl configured to access your cluster

## Build the Tools

```bash
cd /path/to/kkbase-testapp
make build
```

This generates protobuf code and builds two binaries:
- `testservice` - The multi-protocol service runtime
- `testgen` - The manifest generator CLI

## Generate Your First Application

Use the simple-web example (3-tier app: frontend → api → database):

```bash
./testgen generate examples/simple-web/app.yaml -o output
```

Review the generated manifests:

```bash
ls -R output/simple-web/
```

You should see:
- `00-namespaces.yaml` - Namespace definitions
- `10-services/` - Deployments, Services, ServiceMonitors
- `20-gateway/` - Gateway, HTTPRoute, TLS certificates
- `README.md` - Deployment instructions

## Deploy to Kubernetes

```bash
# Apply all manifests
kubectl apply -f output/simple-web/

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l part-of=simple-web --timeout=120s

# Check status
kubectl get pods -l part-of=simple-web
kubectl get gateway
kubectl get httproute
```

## Test the Application

### Basic Request

```bash
# Port forward to the frontend service
kubectl port-forward svc/frontend 8080:8080

# Make a request
curl http://localhost:8080/
```

You'll see a JSON response with the complete call chain showing frontend → api → database.

### Test with Behavior Injection

Add latency:

```bash
curl 'http://localhost:8080/?behavior=latency=200ms'
```

Inject errors:

```bash
curl 'http://localhost:8080/?behavior=error=0.5'
```

Combined behaviors:

```bash
curl 'http://localhost:8080/?behavior=latency=50-200ms,error=0.05'
```

## View Observability Data

### Metrics

```bash
# Forward metrics port
kubectl port-forward svc/frontend 9091:9091

# View Prometheus metrics
curl http://localhost:9091/metrics | grep testservice
```

### Logs

```bash
# View structured JSON logs
kubectl logs -l app=frontend --tail=20
```

### Traces

If you have Jaeger deployed, extract the trace ID from responses:

```bash
TRACE_ID=$(curl -s http://localhost:8080/ | jq -r '.trace_id')
echo "View trace: http://localhost:16686/trace/$TRACE_ID"
```

## Try More Complex Examples

### E-Commerce (Multi-namespace, Mixed Protocols)

```bash
./testgen generate examples/ecommerce/app.yaml -o output
kubectl apply -f output/ecommerce/
```

8 services across 4 namespaces with HTTP and gRPC.

### Microservices Mesh (15+ Services)

```bash
./testgen generate examples/microservices/app.yaml -o output
kubectl apply -f output/microservices-mesh/
```

Complex topology demonstrating deep call chains and multiple ingress points.

## Cleanup

```bash
# Delete the application
kubectl delete -f output/simple-web/
```

## Next Steps

- **[Architecture](../concepts/architecture.md)** - Understand how TestApp works
- **[Behavior Testing](../guides/behavior-testing.md)** - Learn advanced behavior injection
- **[DSL Reference](../reference/dsl-spec.md)** - Create custom applications
- **[Jaeger Setup](../guides/jaeger-setup.md)** - Set up distributed tracing
- **[All Examples](../../examples/)** - Browse example applications

## Troubleshooting

**Pods not starting?**

```bash
kubectl describe pod <pod-name>
kubectl logs <pod-name>
```

**Can't pull image?**

Build and load locally:

```bash
make docker-build
kind load docker-image testservice:latest  # for kind clusters
```

**Gateway not working?**

Ensure you have a Gateway API controller installed (e.g., Envoy Gateway, Istio).

