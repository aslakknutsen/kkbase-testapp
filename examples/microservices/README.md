# Microservices Mesh Example

Complex microservices topology with 15+ services demonstrating deep call chains and service mesh patterns. Spans 3 namespaces (public, internal, data) with mixed protocols, multiple StatefulSet databases, and realistic cross-namespace dependencies.

## Architecture

```
┌─────────────────────────────────────────────┐
│  Gateway: https://mesh.local                │
└──────────────┬──────────────────────────────┘
               │
    ┌──────────┴──────────┐
    │                     │
┌───▼────────┐      ┌────▼──────────┐
│ web-frontend│      │ api-gateway   │  Public Namespace
│  (HTTP)     │      │  (HTTP)       │
└───┬────────┘      └────┬──────────┘
    │                    │
    │    ┌───────────────┴────────────────┐
    │    │                                 │
┌───▼────▼────┐    ┌──────────────┐  ┌───▼──────────┐
│ user-service │    │ order-service│  │ product-      │
│   (gRPC)     │    │   (gRPC)     │  │ service (HTTP)│  Internal
│              │    │              │  │               │  Namespace
└───┬──────────┘    └──┬───────┬───┘  └─┬────────┬───┘
    │                  │       │        │        │
    │    ┌─────────────┘       │        │        │
    │    │                     │        │        │
┌───▼────▼───┐    ┌───────────▼────┐   │   ┌────▼──────┐
│auth-service│    │payment-service │   │   │inventory-  │
│  (HTTP)    │    │    (gRPC)      │   │   │service     │
└────────────┘    └────────────────┘   │   │  (HTTP)    │
                                       │   └─┬──────────┘
                                       │     │
    ┌──────────────────────────────────┴─────┴──────┐
    │                                                 │
┌───▼────────┐  ┌──────────┐  ┌──────────┐  ┌──────▼────┐
│ user-db    │  │product-db│  │order-db  │  │inventory- │  Data
│StatefulSet │  │StatefulSet│ │StatefulSet│ │db         │  Namespace
│  (HTTP)    │  │  (HTTP)  │  │  (HTTP)  │  │StatefulSet│
└────────────┘  └──────────┘  └──────────┘  └───────────┘
                      │
                ┌─────▼──────┐
                │ cache      │
                │ (HTTP)     │
                └────────────┘
```

## Services

| Service | Namespace | Type | Protocol | Purpose |
|---------|-----------|------|----------|---------|
| web-frontend | public | Deployment | HTTP | Frontend entry point |
| api-gateway | public | Deployment | HTTP | API entry point |
| user-service | internal | Deployment | gRPC | User management |
| order-service | internal | Deployment | gRPC | Order processing |
| product-service | internal | Deployment | HTTP | Product catalog |
| auth-service | internal | Deployment | HTTP | Authentication |
| payment-service | internal | Deployment | gRPC | Payment processing |
| inventory-service | internal | Deployment | HTTP | Inventory tracking |
| user-db | data | StatefulSet | HTTP | User data |
| product-db | data | StatefulSet | HTTP | Product data |
| order-db | data | StatefulSet | HTTP | Order data |
| inventory-db | data | StatefulSet | HTTP | Inventory data |
| cache | data | Deployment | HTTP | Caching layer |

## Features

- Complex service dependencies
- Deep call chains
- Multiple protocol translations
- Cross-namespace service mesh
- Multiple ingress patterns
- Resource limits and requests
- StatefulSet patterns
- DaemonSet monitoring agents
- Service-to-service authentication patterns

## Quick Deploy

```bash
# Generate manifests
./testgen generate examples/microservices/app.yaml

# Deploy
kubectl apply -f output/microservices-mesh/

# Wait for ready (may take several minutes)
kubectl wait --for=condition=ready pod -l part-of=microservices-mesh --all-namespaces --timeout=600s
```

## Testing

### Port Forward to Gateway

```bash
kubectl port-forward -n public svc/api-gateway 8080:8080
```

### Basic Request

```bash
curl http://localhost:8080/
```

### Verify Deep Call Chain

```bash
curl -s http://localhost:8080/ | jq '.upstream_calls | length'
```

Shows multiple levels of nested upstream calls.

### Test Targeted Behaviors

```bash
# Target service deep in chain
curl '/?behavior=database-primary:latency=1s'

# Multiple targets across namespaces
curl '/?behavior=api-service:latency=200ms,cache-service:error=0.2'
```

## Observability

```bash
# View all services
kubectl get pods --all-namespaces -l part-of=microservices-mesh

# View metrics
kubectl get servicemonitor -l part-of=microservices-mesh --all-namespaces

# View deep traces (15+ spans)
TRACE_ID=$(curl -s http://localhost:8080/ | jq -r '.trace_id')
echo "http://localhost:16686/trace/$TRACE_ID"
```

## Use Cases

- Test monitoring system scalability (15+ services)
- Validate service mesh configurations
- Test complex trace visualization
- Performance testing with deep call stacks
- Resource limit testing
- Cross-namespace policy testing

## Cleanup

```bash
kubectl delete -f output/microservices-mesh/
```

## See Also

- [Architecture Concepts](../../docs/concepts/architecture.md)
- [Multi-Protocol Support](../../docs/concepts/protocols.md)
- [Behavior Testing](../../docs/guides/behavior-testing.md)

