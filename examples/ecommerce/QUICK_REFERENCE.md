# TestApp E-Commerce - Quick Reference Card

## Quick Start
```bash
# Port forward
kubectl port-forward -n frontend svc/web 8080:8080

# Basic test
curl http://localhost:8080/
```

## Behavior Syntax

### Global Behaviors (Apply to ALL services)

| Behavior | Syntax | Example |
|----------|--------|---------|
| **Latency** | `latency=<duration>` | `latency=100ms` |
| Latency Range | `latency=<min>-<max>` | `latency=50-200ms` |
| **Error Rate** | `error=<rate>` | `error=0.05` (5%) |
| Error Code | `error=<rate>,code=<code>` | `error=1.0,code=503` |
| **CPU** | `cpu=<amount>` | `cpu=200m` |
| CPU Duration | `cpu=<amount>,duration=<time>` | `cpu=500m,duration=5s` |
| **Memory** | `memory=<amount>` | `memory=100Mi` |
| Memory Leak | `memory=leak` | `memory=leak` |
| **Combined** | Comma-separated | `latency=100ms,error=0.1,cpu=200m` |

### Targeted Behaviors (Apply to SPECIFIC service)

| Pattern | Syntax | Example |
|---------|--------|---------|
| **Single Target** | `service:behavior=value` | `product-api:latency=500ms` |
| **Multiple Behaviors** | `service:b1,service:b2` | `order-api:latency=100ms,order-api:error=0.5` |
| **Multiple Services** | `s1:b1,s2:b2` | `product-api:latency=300ms,order-api:error=0.5` |
| **Mixed** | `service:b1,global` | `product-api:latency=500ms,error=0.05` |

### Precedence Rules
- Targeted behavior **overrides** global for that service
- Global applies to all services **without** targeted behavior
- Last wins for same service: `order-api:latency=100ms,order-api:latency=200ms` → 200ms

## Common Test URLs

### Global Behaviors
```bash
# 1. Baseline - No behavior
curl http://localhost:8080/

# 2. Add 500ms latency (ALL services)
curl "http://localhost:8080/?behavior=latency=500ms"

# 3. Random latency (100-1000ms, ALL services)
curl "http://localhost:8080/?behavior=latency=100-1000ms"

# 4. 50% error rate (ALL services)
curl "http://localhost:8080/?behavior=error=0.5"

# 5. Specific error code (503, ALL services)
curl "http://localhost:8080/?behavior=error=1.0,code=503"

# 6. CPU spike (ALL services)
curl "http://localhost:8080/?behavior=cpu=300m"

# 7. Realistic production (ALL services)
curl "http://localhost:8080/?behavior=latency=50-200ms,error=0.05"

# 8. Degraded service (ALL services)
curl "http://localhost:8080/?behavior=latency=500-2000ms,error=0.2,cpu=300m"
```

### Targeted Behaviors
```bash
# 9. Target product-api only
curl "http://localhost:8080/?behavior=product-api:latency=500ms"

# 10. Target order-api only
curl "http://localhost:8080/?behavior=order-api:error=1.0"

# 11. Multiple targets
curl "http://localhost:8080/?behavior=product-api:latency=300ms,order-api:latency=100ms"

# 12. Targeted + global
curl "http://localhost:8080/?behavior=product-api:latency=500ms,error=0.05"

# 13. Database bottleneck
curl "http://localhost:8080/?behavior=product-db:latency=2000ms,order-db:latency=2000ms"

# 14. Complex scenario
curl "http://localhost:8080/?behavior=\
product-api:latency=200-800ms,\
product-api:error=0.1,\
order-api:latency=100ms,\
latency=10ms,\
error=0.01"
```

### Other
```bash
# 15. Using header instead of query param
curl -H "X-Behavior: product-api:latency=100ms" http://localhost:8080/

# 16. Health checks
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

## Response Structure

```json
{
  "service": {
    "name": "web",
    "protocol": "http",
    "namespace": "frontend",
    "pod": "web-xyz",
    "node": "node-1"
  },
  "trace_id": "4bf92f35...",  // ← Copy for Jaeger
  "span_id": "00f067aa...",
  "code": 200,
  "duration": "123.456ms",
  "upstream_calls": [          // ← Full call chain
    {
      "name": "order-api",
      "protocol": "grpc",      // ← Protocol mixing
      "code": 200,
      "duration": "89ms",
      "upstream_calls": [...]  // ← Nested
    }
  ],
  "behaviors_applied": ["latency"]
}
```

## jq Filters

```bash
# Extract trace ID
curl -s http://localhost:8080/ | jq -r '.trace_id'

# View call chain
curl -s http://localhost:8080/ | jq '.upstream_calls[] | {name, protocol, code, duration}'

# List all services
curl -s http://localhost:8080/ | jq 'def services: .service.name, (.upstream_calls[]? | services); [services] | unique'

# Check for errors
curl -s http://localhost:8080/ | jq 'select(.code >= 400)'

# Extract timing info
curl -s http://localhost:8080/ | jq '{service: .service.name, duration, upstreams: [.upstream_calls[].duration]}'
```

## Load Testing

```bash
# Simple loop - 100 requests
for i in {1..100}; do curl -s http://localhost:8080/ > /dev/null & done

# With delays - 10 req/s
for i in {1..100}; do curl -s http://localhost:8080/ > /dev/null & sleep 0.1; done

# Progressive error rates
for rate in 0.01 0.05 0.10 0.20; do
  for i in {1..50}; do
    curl -s "http://localhost:8080/?behavior=error=$rate" > /dev/null &
  done
  sleep 5
done
```

## Direct Service Testing

```bash
# gRPC service (requires grpcurl)
grpcurl -plaintext -d '{"behavior": "latency=100ms"}' \
  order-api.orders.svc.cluster.local:9090 \
  testservice.TestService/Call

# HTTP service (from pod)
kubectl run -it --rm curl --image=curlimages/curl --restart=Never -- \
  curl http://product-api.products.svc.cluster.local:8080/
```

## Metrics

```bash
# View raw metrics
curl http://localhost:9091/metrics

# Request counts
curl http://localhost:9091/metrics | grep testservice_requests_total

# Error rates
curl http://localhost:9091/metrics | grep 'status="[45]'

# Upstream calls
curl http://localhost:9091/metrics | grep testservice_upstream_calls_total
```

## Prometheus Queries

```promql
# Request rate per service
rate(testservice_requests_total[5m])

# Error rate percentage
sum(rate(testservice_requests_total{status=~"5.."}[5m])) 
/ sum(rate(testservice_requests_total[5m])) * 100

# P95 latency
histogram_quantile(0.95, rate(testservice_request_duration_seconds_bucket[5m]))

# Active requests
testservice_active_requests

# Upstream call failures
sum(rate(testservice_upstream_calls_total{code!="200"}[5m])) by (service, upstream)
```

## Troubleshooting

```bash
# Check pod status
kubectl get pods -n frontend
kubectl get pods -n orders
kubectl get pods -n products
kubectl get pods -n payments

# View logs
kubectl logs -n frontend -l app=web --tail=50 -f

# Check upstream connectivity
curl -s http://localhost:8080/ | jq '.upstream_calls[] | select(.error != "") | {name, error}'

# Verify trace propagation
curl -s http://localhost:8080/ | jq '{trace_id, upstreams: [.upstream_calls[].name]}'

# Test individual service
kubectl port-forward -n products svc/product-api 8081:8080
curl http://localhost:8081/
```

## Key Endpoints

| Service | Type | Namespace | Port | Purpose |
|---------|------|-----------|------|---------|
| web | HTTP | frontend | 8080 | Entry point |
| order-api | gRPC | orders | 9090 | Orders |
| product-api | HTTP | products | 8080 | Products |
| payment-api | gRPC | payments | 9090 | Payments |
| *-db | HTTP | various | 8080 | Databases |

## Architecture Diagram

```
┌──────────────────────────────────────────────────────┐
│ Gateway/Ingress: https://shop.local                 │
└───────────────────────┬──────────────────────────────┘
                        │
                        ↓
         ┌──────────────────────────┐
         │   web (HTTP)             │
         │   frontend namespace     │
         └──────────┬────────┬──────┘
                    │        │
        ┌───────────┘        └───────────┐
        │                                │
        ↓                                ↓
┌───────────────┐              ┌────────────────┐
│ order-api     │              │ product-api    │
│ (gRPC)        │←────────────→│ (HTTP)         │
│ orders ns     │              │ products ns    │
└───┬───┬───┬───┘              └────┬───────┬───┘
    │   │   │                       │       │
    │   │   └──────────┐            │       │
    │   │              ↓            ↓       ↓
    │   │    ┌──────────────┐  ┌────────┐ ┌────────┐
    │   │    │ payment-api  │  │prod-db │ │prod-   │
    │   │    │ (gRPC)       │  │(HTTP)  │ │cache   │
    │   │    │ payments ns  │  └────────┘ │(HTTP)  │
    │   │    └──────┬───────┘              └────────┘
    │   │           │
    │   │           ↓
    │   │    ┌──────────────┐
    │   │    │ payment-db   │
    │   │    │ (HTTP)       │
    │   │    └──────────────┘
    │   │
    │   └──────────────────────┐
    │                          │
    ↓                          ↓
┌──────────────┐         ┌──────────────┐
│ order-db     │         │ (other)      │
│ (HTTP)       │         │ upstreams    │
└──────────────┘         └──────────────┘
```

---

**Remember**: All services use the same TestService binary with different configurations!

