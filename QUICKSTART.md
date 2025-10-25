# TestApp Quick Start Guide

Get up and running with TestApp in 5 minutes!

## Prerequisites

- Go 1.21+
- `protoc` (Protocol Buffer compiler)
- Docker (optional, for building images)
- Kubernetes cluster (for deployment)

## 1. Build the Tools

```bash
cd testapp
make build
```

This will:
- Generate protobuf code
- Build `testservice` binary
- Build `testgen` binary

## 2. Generate Your First App

```bash
# Generate from the simple-web example
./testgen generate examples/simple-web/app.yaml

# View generated manifests
ls -R output/simple-web/
```

You should see:
```
output/simple-web/
├── 00-namespaces.yaml
├── 10-services/
│   ├── frontend-deployment.yaml
│   ├── frontend-service.yaml
│   ├── frontend-servicemonitor.yaml
│   ├── api-deployment.yaml
│   ├── api-service.yaml
│   ├── api-servicemonitor.yaml
│   ├── database-statefulset.yaml
│   ├── database-service.yaml
│   └── database-servicemonitor.yaml
├── 20-gateway/
│   ├── gateway.yaml
│   ├── certificates.yaml
│   └── frontend-httproute.yaml
└── README.md
```

## 3. Deploy to Kubernetes

```bash
# Apply all manifests
kubectl apply -f output/simple-web/

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l part-of=simple-web --timeout=120s

# Check status
kubectl get pods
kubectl get gateway
kubectl get httproute
```

## 4. Test the Application

### Option A: Port Forward

```bash
# Forward the frontend service
kubectl port-forward svc/frontend 8080:8080

# Make a request
curl http://localhost:8080/

# Request with behavior
curl 'http://localhost:8080/?behavior=latency=200ms'
```

### Option B: Via Gateway (if ingress is configured)

```bash
# Get gateway address
kubectl get gateway simple-web-gateway

# Make request (adjust host/IP as needed)
curl -H "Host: web.local" http://<gateway-ip>/
```

## 5. View Observability Data

### Metrics

```bash
# Forward metrics port
kubectl port-forward svc/frontend 9091:9091

# View metrics
curl http://localhost:9091/metrics | grep testservice
```

### Logs

```bash
# View structured logs
kubectl logs -l app=frontend --tail=50
```

### Traces (if OTEL collector is configured)

Check your Jaeger/Zipkin UI for distributed traces with the trace IDs from responses.

## 6. Try Complex Examples

### E-Commerce (Multi-namespace, Mixed Protocols)

```bash
./testgen generate examples/ecommerce/app.yaml
kubectl apply -f output/ecommerce/

# View topology
kubectl get pods -n frontend
kubectl get pods -n orders
kubectl get pods -n products
kubectl get pods -n payments
```

### Microservices Mesh (15+ Services)

```bash
./testgen generate examples/microservices/app.yaml
kubectl apply -f output/microservices-mesh/

# This creates a complex topology across 3 namespaces
kubectl get pods --all-namespaces -l part-of=microservices-mesh
```

## 7. Create Your Own App

```bash
# Initialize a new app
./testgen init my-awesome-app

# Edit my-awesome-app.yaml
vim my-awesome-app.yaml

# Generate manifests
./testgen generate my-awesome-app.yaml

# Deploy
kubectl apply -f output/my-awesome-app/
```

## Behavior Testing

TestService supports runtime behavior modification:

```bash
# Add latency
curl 'http://localhost:8080/?behavior=latency=500ms'

# Inject errors
curl 'http://localhost:8080/?behavior=error=503:0.5'

# Combined behaviors
curl 'http://localhost:8080/?behavior=latency=100-500ms,error=0.1'

# CPU spike
curl 'http://localhost:8080/?behavior=cpu=spike:10s:90'
```

## Response Format

Every response includes the complete call chain:

```json
{
  "service": {
    "name": "frontend",
    "version": "1.0.0",
    "namespace": "default",
    "pod": "frontend-7d8f9c-xyz",
    "protocol": "http"
  },
  "start_time": "2025-10-23T12:00:00.000Z",
  "end_time": "2025-10-23T12:00:00.150Z",
  "duration": "150ms",
  "code": 200,
  "trace_id": "abc123...",
  "upstream_calls": [
    {
      "name": "api",
      "uri": "http://api:8080",
      "duration": "80ms",
      "code": 200,
      "upstream_calls": [
        {
          "name": "database",
          "uri": "http://database:8080",
          "duration": "50ms",
          "code": 200
        }
      ]
    }
  ],
  "behaviors_applied": ["latency:range"]
}
```

## Cleanup

```bash
# Delete an application
kubectl delete -f output/simple-web/

# Or delete by namespace
kubectl delete namespace default
```

## Building Docker Image

```bash
# Build testservice image
make docker-build

# Or manually
docker build -t testservice:latest -f Dockerfile .

# Push to your registry
docker tag testservice:latest your-registry/testservice:latest
docker push your-registry/testservice:latest

# Update DSL to use your image
./testgen generate examples/simple-web/app.yaml --image your-registry/testservice:latest
```

## Next Steps

1. **Configure Observability**: Set up Prometheus, Jaeger, and Grafana
2. **Test Your Monitoring**: Use the generated apps to validate your monitoring setup
3. **Chaos Testing**: Use behavior directives to inject failures and test resilience
4. **Performance Testing**: Create load scenarios with different patterns
5. **Custom Apps**: Create DSLs that match your production topology

## Troubleshooting

### Pods not starting?

```bash
kubectl describe pod <pod-name>
kubectl logs <pod-name>
```

### Can't pull image?

Build and load locally:
```bash
make docker-build
kind load docker-image testservice:latest  # for kind
```

### Gateway not working?

Ensure you have a Gateway API controller installed:
```bash
# For Envoy Gateway
helm install eg oci://docker.io/envoyproxy/gateway-helm --version v0.0.0-latest

# Or use your preferred Gateway API implementation
```

## Support

For issues, questions, or contributions, please refer to the main README.md.

Happy Testing! 🚀

