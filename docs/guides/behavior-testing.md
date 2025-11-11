# Behavior Testing Guide

TestService includes a powerful behavior engine that allows runtime modification of service behavior for testing, chaos engineering, and performance analysis.

## Overview

Behaviors can be injected via:
- **Query parameters** (HTTP): `?behavior=latency=200ms`
- **Headers** (HTTP): `X-Behavior: latency=200ms`
- **Request fields** (gRPC): `CallRequest.Behavior`

Behaviors propagate through the entire call chain, allowing you to simulate failures deep in your architecture.

## Basic Behavior Syntax

### Latency

Add artificial latency to simulate slow services or network delays.

**Fixed latency:**
```bash
curl 'http://localhost:8080/?behavior=latency=100ms'
```

**Range latency (random):**
```bash
curl 'http://localhost:8080/?behavior=latency=50-200ms'
# Or with explicit units on both sides
curl 'http://localhost:8080/?behavior=latency=50ms-200ms'
```

**Examples:**
- `latency=5ms` - Fast local cache
- `latency=50-100ms` - Database query
- `latency=200-500ms` - External API call
- `latency=1s-3s` - Slow third-party service

### Error Injection

Inject errors to test failure handling and resilience.

**Probability only (default 500 error):**
```bash
curl 'http://localhost:8080/?behavior=error=0.5'  # 50% error rate
```

**Specific error code (100% rate):**
```bash
curl 'http://localhost:8080/?behavior=error=503'  # Always return 503
```

**Probability with custom code:**
```bash
curl 'http://localhost:8080/?behavior=error=503:0.3'  # 30% chance of 503
curl 'http://localhost:8080/?behavior=error=429:0.05'  # 5% chance of 429
```

### CPU Load

Simulate CPU-intensive operations.

**Default spike:**
```bash
curl 'http://localhost:8080/?behavior=cpu=spike'  # 5s at 80%
```

**Custom duration and intensity:**
```bash
curl 'http://localhost:8080/?behavior=cpu=spike:10s:90'  # 10s at 90%
```

### Memory Patterns

Simulate memory leaks or allocation.

**Memory leak:**
```bash
curl 'http://localhost:8080/?behavior=memory=leak-slow'   # 10MB over 10m
curl 'http://localhost:8080/?behavior=memory=leak-fast'   # Faster leak
curl 'http://localhost:8080/?behavior=memory=leak-slow:5m'  # Custom duration
```

### Pod Crash Testing (Panic)

Trigger pod crashes to test resilience and recovery mechanisms.

**⚠️ WARNING:** Panic behavior will actually crash your pods! Start with low probabilities to avoid excessive disruption.

**Probability-based crash:**
```bash
curl 'http://localhost:8080/?behavior=panic=0.1'   # 10% chance to crash
curl 'http://localhost:8080/?behavior=panic=0.5'   # 50% chance to crash
curl 'http://localhost:8080/?behavior=panic=1.0'   # Always crash (100%)
```

**Use Cases:**

- **Liveness probe testing** - Verify Kubernetes restarts crashed pods
- **Circuit breaker validation** - Confirm circuit breakers open on pod failures
- **Pod restart behavior** - Test application initialization and startup time
- **Cascading failure scenarios** - See how failures propagate through the system
- **Service mesh resilience** - Validate Istio/Linkerd retry and failover behavior

**Examples:**

Test pod restart:
```bash
# Low probability for gradual testing
curl 'http://localhost:8080/?behavior=order-api:panic=0.2'

# Watch pods restart
kubectl get pods -w
```

Combine with latency (latency applied before crash):
```bash
curl 'http://localhost:8080/?behavior=order-api:latency=100ms,panic=0.3'
```

Target specific service in call chain:
```bash
curl 'http://localhost:8080/?behavior=payment-api:panic=0.1'
```

Test cascading failures with crashes:
```bash
curl '/?behavior=order-api:panic=0.5,product-api:error=0.3'
```

**Best Practices:**

- Start with low probabilities (0.1 or less) for initial testing
- Use in non-production environments only
- Monitor pod restart counts: `kubectl get pods`
- Check logs before crash: `kubectl logs <pod> --previous`
- Combine with metrics to validate alerting

**What Happens:**

1. Service receives request with panic behavior
2. Behavior engine evaluates probability
3. If triggered, service calls `panic()` immediately
4. Pod crashes and exits
5. Kubernetes detects failure via liveness probe
6. Kubernetes automatically restarts the pod
7. New pod comes up and handles subsequent requests

**Observability:**

Panic behavior shows in responses before crash:
```json
{
  "behaviors_applied": ["panic:0.50"],
  "upstream_calls": []
}
```

Note: If panic triggers, the response may not be received by the client.

### Combined Behaviors

Apply multiple behaviors at once using commas:

```bash
curl 'http://localhost:8080/?behavior=latency=100ms,error=0.05,cpu=spike'
```

## Service-Targeted Behaviors

Target specific services in the call chain for precise testing.

### Syntax

```
service-name:behavior=value
```

### Examples

**Target single service:**
```bash
curl 'http://localhost:8080/?behavior=product-api:latency=500ms'
```

**Target multiple services:**
```bash
curl 'http://localhost:8080/?behavior=product-api:latency=300ms,order-api:error=0.5'
```

**Mix global and targeted:**
```bash
curl 'http://localhost:8080/?behavior=latency=10ms,product-api:latency=500ms'
```
Result:
- `product-api`: 500ms latency (specific behavior)
- All other services: 10ms latency (global behavior)

### Precedence Rules

1. **Service-specific overrides global** - If a service has a targeted behavior, the global behavior is ignored for that service
2. **Last wins** - If the same service appears multiple times, the last one takes precedence
3. **No merging** - Service-specific behaviors completely replace global, they don't merge

**Example: Override behavior**
```bash
# Global latency, but product-api gets error instead
curl '/?behavior=latency=100ms,product-api:error=0.5'
```
Result:
- `product-api`: Only error behavior (global latency ignored)
- Other services: Only latency behavior

**Example: Chain behaviors on same service**
```bash
# Multiple behaviors for the same service (no new prefix)
curl '/?behavior=product-api:error=0.5,latency=100ms'
```
Result:
- `product-api`: Both error AND latency

**Example: Multiple services**
```bash
curl '/?behavior=product-api:latency=500ms,order-api:error=0.3'
```
Result:
- `product-api`: 500ms latency
- `order-api`: 30% errors
- Other services: No behaviors

### Propagation Through Call Chains

Behaviors propagate to all downstream services:

```
Client Request: ?behavior=order-api:error=0.5
    ↓
Web Service
  - Parses chain, finds no "web" behavior
  - Calls order-api, passes full behavior string
    ↓
Order API Service
  - Parses chain, finds "order-api:error=0.5"
  - Applies error behavior
  - Calls upstreams, passes full behavior string
    ↓
Product API Service
  - Parses chain, finds no "product-api" behavior
  - No behavior applied
```

This enables testing failures deep in the call chain without modifying intermediate services.

## Real-World Testing Scenarios

### Scenario 1: Slow Database

Simulate database slowdown:

```bash
curl '/?behavior=product-db:latency=500-1500ms,order-db:latency=500-1500ms'
```

**Observe:**
- Services calling databases become slow
- Timeout handling
- Connection pool saturation

### Scenario 2: Flaky External API

Simulate unreliable third-party service:

```bash
curl '/?behavior=payment-api:error=503:0.2,latency=500-1000ms'
```

**Observe:**
- 20% of payment calls fail
- High latency even on success
- Retry behavior
- Circuit breaker activation

### Scenario 3: Cascading Failures

Simulate cascading failure across services:

```bash
curl '/?behavior=order-api:error=0.7,product-api:latency=300ms,payment-api:latency=400ms'
```

**Observe:**
- Order service fails frequently
- Downstream services slow down
- Overall system degradation

### Scenario 4: Rate Limiting

Simulate rate limiting on a service:

```bash
curl '/?behavior=payment-api:error=429:0.3'
```

**Observe:**
- 30% of requests get 429 (Too Many Requests)
- Client retry with backoff
- Rate limit handling

### Scenario 5: Testing Timeouts

Make a service extremely slow to test timeout handling:

```bash
curl '/?behavior=order-api:latency=5s'
```

**Observe:**
- Timeout errors if client timeout < 5s
- Graceful timeout handling
- No resource leaks

### Scenario 6: Global Baseline + Specific Issues

Simulate realistic production with one problematic service:

```bash
curl '/?behavior=latency=10ms,error=0.01,product-api:latency=500ms,error=0.2'
```

**Observe:**
- All services have 10ms latency + 1% errors (baseline)
- Product API has additional 500ms latency + 20% errors
- Realistic distributed system behavior

### Scenario 7: Pod Crash Testing

Test resilience to pod crashes:

```bash
curl '/?behavior=order-api:panic=0.3'
```

**Observe:**
- Pods restarting via liveness probes
- Kubernetes automatic recovery
- Request redistribution to healthy pods
- Circuit breaker activation
- Service mesh failover behavior

Combined with other failures:
```bash
curl '/?behavior=order-api:panic=0.2,product-api:error=0.3,latency=100ms'
```

### Scenario 8: Complete System Chaos

Stress test with realistic chaos across all services:

```bash
curl '/?behavior=latency=20-100ms,error=0.05,payment-api:error=0.1,order-db:latency=100-500ms,cache:error=0.15'
```

**Observe:**
- Mixed successes, failures, and latency
- System resilience under stress
- Monitoring/alerting effectiveness

Extreme chaos with pod crashes:
```bash
curl '/?behavior=order-api:panic=0.1,product-api:error=0.5,payment-api:latency=500ms'
```

## Testing Error Handling

### Circuit Breakers

Trigger circuit breaker with high error rate:

```bash
# Run multiple times to trip circuit breaker
for i in {1..20}; do
  curl '/?behavior=order-api:error=0.8'
done
```

### Retry Logic

Test retry behavior with intermittent failures:

```bash
curl '/?behavior=order-api:error=0.5'
```

Watch logs to see retry attempts.

### Fallback Mechanisms

Test fallback by making primary service fail:

```bash
curl '/?behavior=primary-api:error=503'
```

Verify fallback service is called.

## Performance Testing

### Baseline

Measure baseline performance without behaviors:

```bash
curl 'http://localhost:8080/'
```

### Realistic Load

Add realistic network latency:

```bash
curl '/?behavior=latency=10-50ms'
```

### Stress Test

Simulate high load conditions:

```bash
curl '/?behavior=latency=100-300ms'
```

### Worst Case

Combine high latency and errors:

```bash
curl '/?behavior=latency=500ms,error=0.3'
```

## Observability

### Behaviors Applied Field

All responses include `behaviors_applied` showing what was executed:

```json
{
  "service": {"name": "order-api"},
  "code": 500,
  "behaviors_applied": [
    "error:500:0.50",
    "latency:fixed:100ms"
  ],
  "upstream_calls": [...]
}
```

This helps verify:
- Behaviors were applied correctly
- Which behaviors affected each service
- Debugging test scenarios

### Detailed Behavior Reporting

The format includes complete details:

- Latency: `latency:fixed:100ms` or `latency:range:50ms-200ms`
- Error: `error:503:0.30` (code:probability)
- Panic: `panic:0.50` (probability)
- CPU: `cpu:spike:5s:intensity=90`
- Memory: `memory:leak-slow:10485760:10m0s`
- Custom: `custom:key=value`

## Integration with Load Testing

Use behavior injection with load testing tools:

### hey

```bash
# Baseline
hey -n 1000 -c 10 http://localhost:8080/

# With latency
hey -n 1000 -c 10 'http://localhost:8080/?behavior=latency=50-150ms'

# With errors
hey -n 1000 -c 10 'http://localhost:8080/?behavior=error=0.2'
```

### k6

```javascript
import http from 'k6/http';

export default function () {
  // Test with different behaviors
  http.get('http://localhost:8080/?behavior=order-api:error=0.2');
}
```

### wrk

```bash
wrk -t2 -c10 -d30s 'http://localhost:8080/?behavior=latency=100ms'
```

## Common Mistakes

### Wrong: Using comma for error code
```bash
error=0.5,code=503  # Won't work
```

**Correct:**
```bash
error=503:0.5
```

### Wrong: Expecting global to merge with specific
```bash
latency=100ms,order-api:error=0.5
```
**Incorrect assumption:** order-api gets both latency and error
**Reality:** order-api gets ONLY error (specific overrides global)

**Correct way to apply both:**
```bash
order-api:error=0.5,latency=100ms
```

### Wrong: Mixed units
```bash
latency=100ms-200  # Won't parse correctly
```

**Correct:**
```bash
latency=100-200ms   # Unit on max
latency=100ms-200ms # Explicit units
```

## Environment Variable Configuration

Set default behaviors via environment variable:

```yaml
env:
  - name: DEFAULT_BEHAVIOR
    value: "latency=10-30ms"
```

This applies to all requests unless overridden by query parameter.

## Developer Reference

For developers implementing behavior engines or extending functionality, see:

- [Behavior Engine Code Reference](../../pkg/service/behavior/QUICK_REFERENCE.md)
- [Test Scenarios](../../pkg/service/behavior/TEST_SCENARIOS.md)

## See Also

- [Testing Strategies](testing-strategies.md) - Overall testing approach
- [DSL Reference](../reference/dsl-spec.md) - Configure default behaviors in DSL
- [API Reference](../reference/api-reference.md) - HTTP and gRPC APIs
- [Behavior Syntax Reference](../reference/behavior-syntax.md) - Complete syntax reference

