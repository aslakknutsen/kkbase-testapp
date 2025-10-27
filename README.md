# TestApp - Synthetic Application Generator

Generate complex, realistic Kubernetes applications for testing monitoring systems, service meshes, and platform tools.

## Features

- Multi-protocol support (HTTP and gRPC)
- Runtime behavior injection (latency, errors, CPU, memory)
- Full observability (OpenTelemetry traces, Prometheus metrics, structured logs)
- Gateway API support with automatic manifest generation
- Call chain tracing across protocols
- Simple YAML DSL for complex topologies

## Quick Start

```bash
# Build tools
make build

# Generate a 3-tier application
./testgen generate examples/simple-web/app.yaml

# Deploy to Kubernetes
kubectl apply -f output/simple-web/

# Test with behavior injection
curl 'http://localhost:8080/?behavior=latency=500ms,error=0.1'
```

## Documentation

- [Quick Start Guide](docs/getting-started/quickstart.md) - Get running in 5 minutes
- [Architecture](docs/concepts/architecture.md) - How TestApp works
- [Behavior Testing](docs/guides/behavior-testing.md) - Inject failures and latency
- [Complete Documentation](docs/) - Full documentation index

## Examples

| Example | Services | Features | Documentation |
|---------|----------|----------|---------------|
| [simple-web](examples/simple-web/) | 3 | Basic 3-tier app | [README](examples/simple-web/README.md) |
| [ecommerce](examples/ecommerce/) | 8 | Multi-namespace, mixed protocols | [README](examples/ecommerce/README.md) |
| [ecommerce-full](examples/ecommerce-full/) | 12 | Saga pattern, event-driven | [README](examples/ecommerce-full/README.md) |
| [microservices](examples/microservices/) | 15+ | Complex mesh topology | [README](examples/microservices/README.md) |

## Use Cases

- Test monitoring systems (Prometheus, Grafana, custom solutions)
- Validate service mesh configurations (Istio, Linkerd, Gateway API)
- Chaos engineering and failure injection
- Performance testing with realistic topologies
- Training and demos

## Architecture

TestApp consists of two components:

1. **TestService** - Multi-protocol service runtime (HTTP/gRPC)
   - Configurable behavior engine
   - Full observability
   - Upstream call chains

2. **TestGen** - Manifest generator CLI
   - YAML DSL parser
   - Kubernetes resource generator
   - Gateway API resource generator

See [Architecture Documentation](docs/concepts/architecture.md) for details.

## Example Request

```bash
curl 'http://localhost:8080/?behavior=product-api:latency=500ms,order-api:error=0.5'
```

Response includes complete call chain with timing:

```json
{
  "service": {"name": "frontend", "protocol": "http"},
  "duration": "550ms",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "upstream_calls": [
    {"name": "product-api", "duration": "500ms", "behaviors_applied": ["latency:fixed:500ms"]},
    {"name": "order-api", "code": 500, "behaviors_applied": ["error:500:0.50"]}
  ]
}
```

## Building

```bash
# Build binaries
make build

# Build Docker image
make docker-build

# Run tests
make test
```

## License

Apache 2.0
