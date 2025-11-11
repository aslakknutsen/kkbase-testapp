# TestApp Documentation

Complete documentation for TestApp - a synthetic application generator for testing Kubernetes monitoring systems.

## Quick Links

- [Quick Start Guide](getting-started/quickstart.md) - Get running in 5 minutes
- [Examples](../examples/) - Browse example applications
- [Behavior Testing](guides/behavior-testing.md) - Learn behavior injection

## Getting Started

Start here if you're new to TestApp.

- **[Quick Start](getting-started/quickstart.md)** - Build, generate, deploy, and test your first application

## Core Concepts

Understand how TestApp works.

- **[Architecture](concepts/architecture.md)** - System design and components
- **[Multi-Protocol Support](concepts/protocols.md)** - HTTP and gRPC communication
- **[Observability](concepts/observability.md)** - Metrics, traces, and logs

## Guides

Step-by-step guides for common tasks.

- **[Behavior Testing](guides/behavior-testing.md)** - Inject latency, errors, and resource patterns
- **[Path-Based Routing](guides/path-routing.md)** - Route requests based on URL paths
- **[Jaeger Setup](guides/jaeger-setup.md)** - Deploy distributed tracing
- **[Istio Setup](guides/istio-setup.md)** - Configure Istio service mesh
- **[Testing Strategies](guides/testing-strategies.md)** - Unit, integration, and E2E testing

## Reference

Complete API and syntax references.

- **[DSL Specification](reference/dsl-spec.md)** - YAML DSL complete reference
- **[Behavior Syntax](reference/behavior-syntax.md)** - Behavior string format
- **[Environment Variables](reference/environment-variables.md)** - Runtime configuration
- **[CLI Reference](reference/cli-reference.md)** - testgen commands and flags
- **[API Reference](reference/api-reference.md)** - HTTP and gRPC APIs

## Examples

Complete example applications with documentation.

| Example | Services | Complexity | Description |
|---------|----------|------------|-------------|
| [simple-web](../examples/simple-web/) | 3 | Basic | 3-tier app: frontend → api → database |
| [ecommerce](../examples/ecommerce/) | 8 | Intermediate | Multi-namespace with mixed protocols |
| [ecommerce-full](../examples/ecommerce-full/) | 12 | Advanced | Saga pattern with event-driven architecture |
| [microservices](../examples/microservices/) | 15+ | Advanced | Complex mesh topology |

## Developer Documentation

For developers extending or contributing to TestApp.

- **[Behavior Engine Reference](../pkg/service/behavior/QUICK_REFERENCE.md)** - Developer guide
- **[Test Scenarios](../pkg/service/behavior/TEST_SCENARIOS.md)** - Unit test documentation

## Documentation Map

```
docs/
├── getting-started/
│   └── quickstart.md                  # 5-minute getting started
│
├── concepts/
│   ├── architecture.md                # System design
│   ├── protocols.md                   # HTTP/gRPC support
│   └── observability.md               # Metrics, traces, logs
│
├── guides/
│   ├── behavior-testing.md            # Behavior injection
│   ├── path-routing.md                # URL-based routing
│   ├── jaeger-setup.md                # Distributed tracing
│   └── testing-strategies.md          # Testing approaches
│
└── reference/
    ├── dsl-spec.md                    # YAML DSL reference
    ├── behavior-syntax.md             # Behavior format
    ├── environment-variables.md       # Env vars
    ├── cli-reference.md               # testgen CLI
    └── api-reference.md               # HTTP/gRPC APIs
```

## Common Tasks

### Create a New Application

```bash
./testgen init my-app
vim my-app.yaml
./testgen generate my-app.yaml
kubectl apply -f output/my-app/
```

See: [CLI Reference](reference/cli-reference.md)

### Inject Behaviors

```bash
curl 'http://service/?behavior=latency=200ms,error=0.1'
```

See: [Behavior Testing Guide](guides/behavior-testing.md)

### Set Up Tracing

```bash
helm install jaeger jaegertracing/jaeger
kubectl apply -f output/my-app/
```

See: [Jaeger Setup Guide](guides/jaeger-setup.md)

### Run Load Tests

```bash
hey -n 1000 -c 10 'http://service/?behavior=latency=50-150ms'
```

See: [Testing Strategies](guides/testing-strategies.md)

## Use Cases

- **Testing Monitoring Systems** - Validate Prometheus, Grafana, or custom monitoring
- **Service Mesh Validation** - Test Istio, Linkerd, or Gateway API configurations
- **Performance Testing** - Create realistic load scenarios
- **Chaos Engineering** - Inject failures to test resilience
- **Training** - Demonstrate Kubernetes concepts with real applications

## Support

### Troubleshooting

Check these guides for common issues:
- [Quick Start - Troubleshooting](getting-started/quickstart.md#troubleshooting)
- [Jaeger Setup - Troubleshooting](guides/jaeger-setup.md#troubleshooting)
- [CLI Reference - Troubleshooting](reference/cli-reference.md#troubleshooting)

### Additional Resources

- [Main README](../README.md) - Project overview
- [CHANGELOG](../CHANGELOG.md) - Version history
- [Examples](../examples/) - Sample applications

## Contributing

Contributions welcome! Areas for enhancement:
- Additional behavior patterns
- More example applications
- Enhanced observability features
- Additional protocol support

## License

Apache 2.0

