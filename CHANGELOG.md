# TestApp Changelog

## [Unreleased]

### Added
- **Targeted Behavior Chains** - Target specific services in call chains
  - New syntax: `service-name:behavior=value` (e.g., `product-api:latency=500ms`)
  - Mix targeted and global behaviors
  - Behavior chain propagates through entire call chain
  - Backward compatible with existing syntax
  - See [Behavior Testing Guide](docs/guides/behavior-testing.md) for details
- Added `imagePullPolicy: Always` to all generated workloads (Deployment, StatefulSet, DaemonSet)
  - Makes it easy to update the testservice image without manually deleting pods
  - Kubernetes will always pull the latest image on pod restart
- Separated liveness and readiness probe endpoints for better probe semantics
  - Liveness probe: `/health` - checks if the service process is alive (restarts pod if fails)
  - Readiness probe: `/ready` - checks if service is ready to receive traffic (removes from load balancer if fails)
  - Both endpoints exposed on HTTP port 8080 for all services

### Refactored
- Extracted shared upstream calling logic into `pkg/service/client` package
  - Unified client component eliminates code duplication
  - Fixed nil pointer panic in gRPC server

### Improved
- gRPC server properly extracts and returns trace IDs
- Complete field consistency between HTTP and gRPC responses
- See [Protocol Documentation](docs/concepts/protocols.md) for details

### Fixed
- YAML indentation in generated manifests (resolves `unknown field` errors)
- Latency range parsing supports shorthand notation (`5-20ms`)
- Health probe configuration for all service types
- HTTP→gRPC and gRPC→gRPC upstream calls
- Protocol URL handling and trace propagation

## [Initial Release]

### Added
- TestService multi-protocol service binary (HTTP + gRPC)
- Behavior engine supporting latency, errors, CPU, and memory patterns
- Full observability stack (OTEL traces, Prometheus metrics, structured logs)
- TestGen CLI for manifest generation
- YAML-based DSL for defining applications
- Kubernetes manifest generator (Deployment, StatefulSet, DaemonSet, Service, ServiceMonitor)
- Gateway API manifest generator (Gateway, HTTPRoute, GRPCRoute, TLS certificates)
- Three example applications (simple-web, ecommerce, microservices)
- Comprehensive documentation (README, QUICKSTART, IMPLEMENTATION_SUMMARY)
- Makefile for build automation
- Dockerfile for container images

