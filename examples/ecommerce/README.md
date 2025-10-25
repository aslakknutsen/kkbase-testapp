# E-Commerce Example Application

A complex multi-namespace e-commerce application demonstrating TestApp's capabilities with mixed HTTP/gRPC protocols and cross-namespace service mesh patterns.

## Architecture

```
┌─────────────────────────────────────────────────┐
│  Gateway: https://shop.local                    │
└──────────────────┬──────────────────────────────┘
                   │
                   ↓
         ┌─────────────────┐
         │  web (HTTP)     │  Frontend Namespace
         │  Port: 8080     │
         └────┬────────┬───┘
              │        │
    ┌─────────┘        └────────┐
    │                           │
    ↓                           ↓
┌────────────┐          ┌──────────────┐
│ order-api  │          │ product-api  │
│ (gRPC)     │◄────────►│ (HTTP)       │
│ 9090       │          │ 8080         │
│ Orders NS  │          │ Products NS  │
└─┬────┬───┬─┘          └──┬───────┬───┘
  │    │   │               │       │
  │    │   │               ↓       ↓
  │    │   │          ┌────────┐ ┌────────┐
  │    │   │          │prod-db │ │prod-   │
  │    │   │          │(HTTP)  │ │cache   │
  │    │   │          └────────┘ │(HTTP)  │
  │    │   │                     └────────┘
  │    │   └──────────┐
  │    │              ↓
  │    │      ┌──────────────┐
  │    │      │ payment-api  │
  │    │      │ (gRPC) 9090  │
  │    │      │ Payments NS  │
  │    │      └──────┬───────┘
  │    │             │
  │    │             ↓
  │    │      ┌──────────────┐
  │    │      │ payment-db   │
  │    │      │ (HTTP)       │
  │    │      └──────────────┘
  │    │
  │    └────────────┐
  │                 │
  ↓                 ↓
┌──────────┐   ┌──────────┐
│order-db  │   │  ...     │
│(HTTP)    │   │          │
└──────────┘   └──────────┘
```

## Services

| Service | Namespace | Type | Protocol | Purpose |
|---------|-----------|------|----------|---------|
| web | frontend | Deployment | HTTP | Web frontend (entry point) |
| order-api | orders | Deployment | gRPC | Order processing |
| product-api | products | Deployment | HTTP | Product catalog |
| payment-api | payments | Deployment | gRPC | Payment processing |
| product-db | products | StatefulSet | HTTP | Product database |
| product-cache | products | Deployment | HTTP | Product cache |
| order-db | orders | StatefulSet | HTTP | Order database |
| payment-db | payments | StatefulSet | HTTP | Payment database |

## Features Demonstrated

✅ **Multi-Namespace Architecture**
- 4 namespaces (frontend, orders, products, payments)
- Cross-namespace service communication
- ReferenceGrants for Gateway API

✅ **Mixed Protocols**
- HTTP services (web, product-api, databases, cache)
- gRPC services (order-api, payment-api)
- Protocol translation (HTTP→gRPC→HTTP)

✅ **Service Types**
- Deployments for stateless services
- StatefulSets for databases with persistent storage

✅ **Gateway API**
- TLS-enabled HTTPRoute
- Cross-namespace backend references
- Certificate management

✅ **Observability**
- Full distributed tracing across protocols
- Prometheus metrics for all services
- ServiceMonitor CRDs for automatic scraping

✅ **Behavior Patterns**
- Configurable latency per service
- Error injection rates
- Resource limits (CPU/memory)

## Quick Start

### 1. Deploy the Application

```bash
# Generate manifests
cd testapp
./testgen generate examples/ecommerce/app.yaml -o output

# Apply to cluster
kubectl apply -f output/ecommerce/

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l part-of=ecommerce --all-namespaces --timeout=300s
```

### 2. Test Locally

```bash
# Port forward the web service
kubectl port-forward -n frontend svc/web 8080:8080

# Make a test request
curl http://localhost:8080/

# Or run the automated test suite
./examples/ecommerce/test-features.sh
```

### 3. Test via Gateway (if configured)

```bash
# Add to /etc/hosts
echo "127.0.0.1 shop.local" | sudo tee -a /etc/hosts

# Test via HTTPS
curl https://shop.local/ --insecure
```

## Testing Resources

📚 **[EXAMPLE_URLS.md](EXAMPLE_URLS.md)** - Comprehensive testing guide with 27+ examples
- Basic requests
- Behavior injection (latency, errors, CPU, memory)
- Cross-protocol testing
- Load testing scenarios
- Advanced chaos testing
- Observability integration

🎯 **[QUICK_REFERENCE.md](QUICK_REFERENCE.md)** - Quick reference card
- Behavior syntax table
- Common test URLs
- jq filters for response parsing
- Prometheus queries
- Troubleshooting commands

🔧 **[test-features.sh](test-features.sh)** - Automated test script
```bash
# Run all feature tests
./examples/ecommerce/test-features.sh

# Verbose output
VERBOSE=true ./examples/ecommerce/test-features.sh

# Custom URL
BASE_URL=https://shop.local ./examples/ecommerce/test-features.sh
```

## Example Requests

### Basic Request
```bash
curl http://localhost:8080/
```

Returns full call chain with trace IDs:
```json
{
  "service": {"name": "web", "protocol": "http"},
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "upstream_calls": [
    {
      "name": "order-api",
      "protocol": "grpc",
      "upstream_calls": [
        {"name": "product-api", "protocol": "http"},
        {"name": "payment-api", "protocol": "grpc"},
        {"name": "order-db", "protocol": "http"}
      ]
    },
    {"name": "product-api", "protocol": "http"}
  ]
}
```

### Inject Latency
```bash
curl "http://localhost:8080/?behavior=latency=500ms"
```

### Inject Errors
```bash
curl "http://localhost:8080/?behavior=error=0.5"  # 50% error rate
```

### Combined Behaviors
```bash
curl "http://localhost:8080/?behavior=latency=50-200ms,error=0.05,cpu=200m"
```

## Observability

### View Metrics
```bash
# Port forward metrics port
kubectl port-forward -n frontend svc/web 9091:9091

# View raw metrics
curl http://localhost:9091/metrics

# Filter request counts
curl http://localhost:9091/metrics | grep testservice_requests_total
```

### View Traces (Jaeger)
```bash
# Get trace ID from response
TRACE_ID=$(curl -s http://localhost:8080/ | jq -r '.trace_id')

# Open in Jaeger (if deployed)
open "http://localhost:16686/trace/$TRACE_ID"
```

### Prometheus Queries
```promql
# Request rate
rate(testservice_requests_total{service="web"}[5m])

# Error rate
sum(rate(testservice_requests_total{status=~"5.."}[5m])) 
/ sum(rate(testservice_requests_total[5m])) * 100

# P95 latency
histogram_quantile(0.95, 
  rate(testservice_request_duration_seconds_bucket[5m]))
```

## Load Testing

### Simple Load
```bash
# 100 requests
for i in {1..100}; do 
  curl -s http://localhost:8080/ > /dev/null & 
done
```

### Realistic Load Pattern
```bash
# 10 req/s for 60 seconds
for i in {1..600}; do
  curl -s http://localhost:8080/ > /dev/null &
  sleep 0.1
done
```

### Progressive Error Injection
```bash
for rate in 0.01 0.05 0.10 0.20; do
  echo "Testing error rate: $rate"
  for i in {1..50}; do
    curl -s "http://localhost:8080/?behavior=error=$rate" > /dev/null &
  done
  sleep 10
done
```

## Cleanup

```bash
# Delete all resources
kubectl delete -f output/ecommerce/

# Or use testgen
./testgen delete examples/ecommerce/app.yaml
```

## Configuration

See [app.yaml](app.yaml) for the full DSL definition including:
- Service configurations
- Namespace assignments
- Protocol specifications
- Upstream relationships
- Behavior settings
- Resource limits
- Gateway/Ingress configuration

## Troubleshooting

### Pods not starting
```bash
kubectl get pods --all-namespaces | grep ecommerce
kubectl describe pod -n frontend <pod-name>
kubectl logs -n frontend <pod-name>
```

### Service connectivity issues
```bash
# Test from within cluster
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://web.frontend.svc.cluster.local:8080/

# Check DNS
kubectl run -it --rm debug --image=busybox --restart=Never -- \
  nslookup web.frontend.svc.cluster.local
```

### View full call chain
```bash
curl -s http://localhost:8080/ | jq '.upstream_calls'
```

## Next Steps

- 🚀 **Load Test**: Use k6 or similar to generate realistic traffic
- 📊 **Monitor**: Connect to your kkbase monitoring system
- 🔍 **Trace**: Deploy Jaeger to visualize distributed traces
- 📈 **Dashboard**: Create Grafana dashboards for the application
- 🧪 **Chaos**: Use behavior injection to test failure scenarios
- 🔄 **CI/CD**: Integrate manifest generation into your pipeline

---

**Need Help?** Check the main [TestApp README](../../README.md) or the [example URLs guide](EXAMPLE_URLS.md).

