# TestApp Changelog

## [Unreleased]

### Added
- **Targeted Behavior Chains** - Major new feature allowing behaviors to target specific services in call chains
  - New syntax: `service-name:behavior=value` (e.g., `product-api:latency=500ms`)
  - Can mix targeted and global behaviors: `product-api:latency=500ms,error=0.05`
  - Global behaviors apply to all services, targeted behaviors override for specific services
  - Behavior chain is propagated through entire call chain via HTTP query params and gRPC request fields
  - Each service extracts only the behavior applicable to itself using `ParseChain()` and `ForService()`
  - Enables precise testing: target specific bottlenecks, simulate cascading failures, test individual service degradation
  - Backward compatible: old syntax (no service prefix) still works as global behavior
  - See `TARGETED_BEHAVIOR.md` and `examples/ecommerce/EXAMPLE_URLS.md` for detailed examples
  - Implementation:
    - Added `ServiceBehavior` and `BehaviorChain` types to `pkg/service/behavior/engine.go`
    - Added `ParseChain()` function to parse service-targeted syntax
    - Added `ForService()` method to extract behavior for specific service
    - Updated HTTP and gRPC servers to use `ParseChain()` instead of `Parse()`
    - Updated `client.Caller` to accept and propagate behavior string parameter
- Added `imagePullPolicy: Always` to all generated workloads (Deployment, StatefulSet, DaemonSet)
  - Makes it easy to update the testservice image without manually deleting pods
  - Kubernetes will always pull the latest image on pod restart
- Separated liveness and readiness probe endpoints for better probe semantics
  - Liveness probe: `/health` - checks if the service process is alive (restarts pod if fails)
  - Readiness probe: `/ready` - checks if service is ready to receive traffic (removes from load balancer if fails)
  - Both endpoints exposed on HTTP port 8080 for all services

### Refactored
- Extracted shared upstream calling logic into `pkg/service/client` package
  - Created unified `Caller` component that both HTTP and gRPC servers use
  - Eliminates code duplication between HTTP and gRPC upstream call logic
  - Centralizes protocol detection, trace propagation, and response conversion
  - Single source of truth for HTTP and gRPC client behavior
  - Easier to maintain and extend with new features
  - Reduced codebase size by ~150 lines
  - **CRITICAL FIX**: Fixed nil pointer panic in gRPC server - `caller` field now properly initialized

### Improved
- gRPC server now properly extracts and returns trace IDs (TraceID, SpanID) from span context
  - Matches HTTP server behavior for consistent observability
  - Makes it easier to correlate gRPC responses with distributed traces
  - Uses standard `trace.SpanFromContext()` pattern
- Ensured complete field consistency between HTTP and gRPC responses
  - Both return identical structures (service info, timing, trace IDs, upstream calls, behaviors)
  - Body messages now include protocol indicator: "Hello from X (HTTP)" and "Hello from X (gRPC)"
  - See RESPONSE_CONSISTENCY.md for detailed comparison

### Fixed
- Fixed YAML indentation issue in generated Deployment, StatefulSet, and DaemonSet manifests
  - `spec.template.metadata.labels` now has correct 8-space indentation (was incorrectly using 4 spaces)
  - `metadata.labels` correctly uses 4-space indentation
  - Updated `getLabels()` function to accept an indent parameter for flexible indentation levels
  - This resolves the `unknown field "spec.template.app"` error when applying manifests
- Fixed latency range parsing to support shorthand notation (e.g., `"5-20ms"`)
  - When min value lacks a unit, the parser now automatically extracts and applies the unit from max value
  - Supports both `"5-20ms"` (shorthand) and `"5ms-20ms"` (explicit) formats
  - This resolves the `time: missing unit in duration` error for behavior strings
- Fixed health probe configuration for all service types
  - TestService always exposes HTTP `/health` endpoint on port 8080 regardless of protocol
  - All workloads now get liveness/readiness probes on HTTP port 8080
  - HTTP port is set for all services (even gRPC-only) to support health checks
  - For gRPC-only services, HTTP port is used internally for health checks but not exposed in Service resource
  - This resolves the `port: Invalid value: 0: must be between 1 and 65535` error
- Fixed gRPC upstream call support in HTTP server
  - HTTP server now properly routes to gRPC client when upstream protocol is `grpc://`
  - Added `callUpstreamGRPC()` method to handle gRPC connections and trace propagation
  - Converts gRPC responses to standard call chain format
  - This resolves the `unsupported protocol scheme "grpc"` error when HTTP services call gRPC upstreams
- Fixed protocol URL handling in gRPC server for upstream calls
  - gRPC server now strips `grpc://` prefix before calling `grpc.Dial()`
  - Implemented full HTTP upstream call support in gRPC server (was previously a stub)
  - Both gRPC→gRPC and gRPC→HTTP calls now work correctly
  - Proper trace propagation for both protocols
  - This resolves the `too many colons in address` error when gRPC services call gRPC upstreams

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

