# Simple Web Example

Basic 3-tier application demonstrating TestApp fundamentals. This example shows a simple web frontend calling a backend API, which in turn calls a database. Perfect for learning core concepts before exploring more complex topologies.

## Architecture

```
┌─────────────┐
│   Gateway   │ https://web.local
└──────┬──────┘
       │
┌──────▼──────┐
│  frontend   │ HTTP :8080
│  (2 pods)   │
└──────┬──────┘
       │
┌──────▼──────┐
│     api     │ HTTP :8080
│  (2 pods)   │
└──────┬──────┘
       │
┌──────▼──────┐
│  database   │ HTTP :8080
│ StatefulSet │
│   (1 pod)   │
└─────────────┘
```

## Services

| Service | Type | Protocol | Replicas | Purpose |
|---------|------|----------|----------|---------|
| frontend | Deployment | HTTP | 2 | Web frontend |
| api | Deployment | HTTP | 2 | Backend API |
| database | StatefulSet | HTTP | 1 | Database (simulated) |

## Features

- Basic Deployment and StatefulSet
- Service-to-service communication
- HTTPRoute with Gateway API
- TLS certificates (self-signed)
- Prometheus ServiceMonitors
- Basic behavior injection

## Quick Deploy

```bash
# Generate manifests
./testgen generate examples/simple-web/app.yaml

# Deploy
kubectl apply -f output/simple-web/

# Wait for ready
kubectl wait --for=condition=ready pod -l part-of=simple-web --timeout=120s
```

## Testing

### Basic Request

```bash
kubectl port-forward svc/frontend 8080:8080
curl http://localhost:8080/
```

### With Behaviors

```bash
# Add latency
curl 'http://localhost:8080/?behavior=latency=200ms'

# Inject errors
curl 'http://localhost:8080/?behavior=error=0.5'

# Target specific service
curl 'http://localhost:8080/?behavior=api:latency=500ms'
```

### Verify Call Chain

```bash
curl -s http://localhost:8080/ | jq '.upstream_calls[].name'
# Output: "api"
# Then api calls database
```

## Cleanup

```bash
kubectl delete -f output/simple-web/
```

## See Also

- [Behavior Testing Guide](../../docs/guides/behavior-testing.md)
- [Quick Start](../../docs/getting-started/quickstart.md)
- [DSL Reference](../../docs/reference/dsl-spec.md)

