# E-Commerce Example

Multi-namespace e-commerce application demonstrating mixed HTTP/gRPC protocols and cross-namespace communication. Features 8 services across 4 namespaces with both StatefulSets and Deployments, showcasing realistic microservices patterns.

## Architecture

```
┌─────────────────────────────────┐
│  Gateway: https://shop.local     │
└──────────────┬──────────────────┘
               │
        ┌──────▼──────┐
        │  web (HTTP) │  Frontend Namespace
        │  Port: 8080 │
        └──────┬──────┘
               │
      ┌────────┴────────┐
      │                 │
┌─────▼─────┐    ┌─────▼──────┐
│ order-api │    │product-api │
│  (gRPC)   │    │   (HTTP)   │
│   :9090   │    │   :8080    │
│ Orders NS │    │Products NS │
└─┬───┬───┬─┘    └─┬────────┬─┘
  │   │   │        │        │
  │   │   │        ▼        ▼
  │   │   │    ┌────────┐┌────────┐
  │   │   │    │prod-db ││ cache  │
  │   │   │    │(HTTP)  ││(HTTP)  │
  │   │   │    └────────┘└────────┘
  │   │   │
  │   │   └──────────┐
  │   │              ▼
  │   │      ┌──────────────┐
  │   │      │ payment-api  │
  │   │      │ (gRPC) :9090 │
  │   │      │ Payments NS  │
  │   │      └──────┬───────┘
  │   │             │
  │   │             ▼
  │   │      ┌──────────────┐
  │   │      │ payment-db   │
  │   │      │ (HTTP)       │
  │   │      └──────────────┘
  │   │
  │   └────────────┐
  │                │
  ▼                ▼
┌──────────┐   ┌──────────┐
│order-db  │   │  ...     │
│(HTTP)    │   │          │
└──────────┘   └──────────┘
```

## Services

| Service | Namespace | Type | Protocol | Upstreams |
|---------|-----------|------|----------|-----------|
| web | frontend | Deployment | HTTP | order-api, product-api |
| order-api | orders | Deployment | gRPC | product-api, payment-api, order-db |
| product-api | products | Deployment | HTTP | product-db, product-cache |
| payment-api | payments | Deployment | gRPC | payment-db |
| product-db | products | StatefulSet | HTTP | - |
| product-cache | products | Deployment | HTTP | - |
| order-db | orders | StatefulSet | HTTP | - |
| payment-db | payments | StatefulSet | HTTP | - |

## Features

- 4 namespaces (frontend, orders, products, payments)
- Mixed protocols (HTTP and gRPC)
- Cross-namespace service calls
- ReferenceGrants for Gateway API
- Multiple StatefulSets
- Complex call chains

## Quick Deploy

```bash
# Generate manifests
./testgen generate examples/ecommerce/app.yaml

# Deploy
kubectl apply -f output/ecommerce/

# Wait for ready
kubectl wait --for=condition=ready pod -l part-of=ecommerce --all-namespaces --timeout=300s
```

## Testing

```bash
# Port forward to web service
kubectl port-forward -n frontend svc/web 8080:8080

# Basic request
curl http://localhost:8080/
```

### With Behaviors

```bash
# Slow product service
curl 'http://localhost:8080/?behavior=product-api:latency=500ms'

# Failing order service
curl 'http://localhost:8080/?behavior=order-api:error=0.5'

# Multiple targets
curl 'http://localhost:8080/?behavior=product-api:latency=300ms,order-api:error=0.3'
```

### Verify Protocols

```bash
curl -s http://localhost:8080/ | jq '.upstream_calls[] | {name, protocol}'
```

Expected:
```json
{"name":"order-api","protocol":"grpc"}
{"name":"product-api","protocol":"http"}
```

## Observability

```bash
# View metrics
kubectl port-forward -n frontend svc/web 9091:9091
curl http://localhost:9091/metrics | grep testservice

# View traces in Jaeger
TRACE_ID=$(curl -s http://localhost:8080/ | jq -r '.trace_id')
echo "http://localhost:16686/trace/$TRACE_ID"

# View logs
kubectl logs -n frontend -l app=web --tail=20
```

## Cleanup

```bash
kubectl delete -f output/ecommerce/
```

## See Also

- [Behavior Testing Guide](../../docs/guides/behavior-testing.md)
- [Protocol Support](../../docs/concepts/protocols.md)
- [Jaeger Setup](../../docs/guides/jaeger-setup.md)
- [DSL Reference](../../docs/reference/dsl-spec.md)
