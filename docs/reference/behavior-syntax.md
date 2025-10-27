# Behavior Syntax Reference

Complete reference for TestService behavior string syntax.

## Overview

Behaviors modify service behavior at runtime for testing. They can be specified via:
- Query parameters: `?behavior=latency=200ms`
- HTTP headers: `X-Behavior: latency=200ms`
- gRPC request field: `CallRequest.Behavior`

## Basic Syntax

### Single Behavior

```
behavior-type=value
```

Example: `latency=100ms`

### Multiple Behaviors

Separate with commas:

```
behavior-type1=value1,behavior-type2=value2
```

Example: `latency=100ms,error=0.05`

## Latency Behaviors

Add artificial delay to responses.

### Fixed Latency

```
latency=<duration>
```

**Examples:**
- `latency=100ms` - 100 millisecond delay
- `latency=1s` - 1 second delay
- `latency=500us` - 500 microsecond delay

**Duration Units:** `ns`, `us`, `ms`, `s`, `m`, `h`

### Range Latency

Random delay within range:

```
latency=<min>-<max>
```

**Examples:**
- `latency=50-200ms` - Random 50-200ms
- `latency=50ms-200ms` - Same as above (explicit units)
- `latency=100-500ms` - Random 100-500ms
- `latency=1s-3s` - Random 1-3 seconds

## Error Behaviors

Inject errors into responses.

### Probability Only

```
error=<probability>
```

Default error code is 500.

**Examples:**
- `error=0.5` - 50% error rate (500)
- `error=0.1` - 10% error rate (500)
- `error=1.0` - 100% error rate (500)

**Probability:** 0.0 to 1.0 (0% to 100%)

### Specific Error Code

Always return specified code:

```
error=<code>
```

**Examples:**
- `error=503` - Always return 503
- `error=429` - Always return 429
- `error=404` - Always return 404

### Code with Probability

```
error=<code>:<probability>
```

**Examples:**
- `error=503:0.3` - 30% chance of 503
- `error=429:0.05` - 5% chance of 429 (rate limiting)
- `error=404:0.1` - 10% chance of 404

## CPU Behaviors

Simulate CPU-intensive operations.

### Default Spike

```
cpu=spike
```

5 second spike at 80% intensity.

### Custom Duration

```
cpu=spike:<duration>
```

**Examples:**
- `cpu=spike:10s` - 10 second spike
- `cpu=spike:30s` - 30 second spike

### Custom Duration and Intensity

```
cpu=spike:<duration>:<intensity>
```

**Examples:**
- `cpu=spike:5s:90` - 5 seconds at 90%
- `cpu=spike:10s:50` - 10 seconds at 50%

## Memory Behaviors

Simulate memory allocation and leaks.

### Memory Leak (Slow)

```
memory=leak-slow
memory=leak-slow:<duration>
```

**Examples:**
- `memory=leak-slow` - 10MB over 10 minutes
- `memory=leak-slow:5m` - Leak over 5 minutes

### Memory Leak (Fast)

```
memory=leak-fast
memory=leak-fast:<duration>
```

Faster memory leak pattern.

## Service-Targeted Behaviors

Apply behaviors to specific services in the call chain.

### Syntax

```
<service-name>:<behavior>=<value>
```

### Single Target

**Examples:**
- `product-api:latency=500ms`
- `order-api:error=0.5`
- `database:latency=1s`

### Multiple Targets

**Examples:**
- `product-api:latency=300ms,order-api:error=0.5`
- `db-1:latency=500ms,db-2:latency=800ms`

### Chaining Behaviors for Same Service

**Examples:**
- `order-api:error=0.5,latency=100ms` - Both error AND latency
- `product-api:latency=500ms,error=0.1,cpu=spike` - Three behaviors

Note: No new service prefix means continue previous service.

### Global + Targeted Mix

**Examples:**
- `latency=10ms,product-api:latency=500ms`
  - All services: 10ms
  - product-api: 500ms (overrides global)

- `error=0.05,order-api:error=0.3,latency=200ms`
  - All services: 5% errors
  - order-api: 30% errors + 200ms latency

## Precedence Rules

1. **Service-specific overrides global** - If a service has targeted behavior, global is ignored
2. **No merging** - Targeted behavior completely replaces global for that service
3. **Last wins** - Multiple references to same service, last takes precedence

**Example:**

```
latency=100ms,order-api:error=0.5
```

Result:
- `order-api`: Only error (global latency ignored)
- Other services: Only latency

To apply both to order-api:

```
order-api:error=0.5,latency=100ms
```

## Complete Examples

### Simple Scenarios

**Database slowdown:**
```
latency=500ms
```

**Flaky service:**
```
latency=100-500ms,error=0.2
```

**High error rate:**
```
error=0.8
```

### Targeted Scenarios

**Slow product service:**
```
product-api:latency=1s
```

**Failing payment service:**
```
payment-api:error=503:0.5
```

**Multiple services degraded:**
```
product-api:latency=500ms,order-api:error=0.3,payment-api:latency=800ms
```

### Complex Scenarios

**Baseline + specific issues:**
```
latency=10ms,error=0.01,product-api:latency=500ms,error=0.2
```

Result:
- All services: 10ms latency + 1% errors
- product-api: 500ms latency + 20% errors (global ignored)

**Cascading failures:**
```
order-api:error=0.7,product-api:latency=300ms,payment-api:latency=400ms
```

**Chaos testing:**
```
latency=20-100ms,error=0.05,payment-api:error=0.1,db:latency=100-500ms
```

## Default Behavior

Set default behavior via environment variable:

```yaml
env:
  - name: DEFAULT_BEHAVIOR
    value: "latency=10-30ms"
```

Request-time behaviors override defaults.

## Observability

Applied behaviors appear in responses:

```json
{
  "behaviors_applied": [
    "latency:fixed:100ms",
    "error:503:0.30"
  ]
}
```

Format includes details:
- Latency: `latency:fixed:100ms` or `latency:range:50ms-200ms`
- Error: `error:503:0.30` (code:probability)
- CPU: `cpu:spike:5s:intensity=90`
- Memory: `memory:leak-slow:10485760:10m0s`

## Common Mistakes

**Wrong: Comma for error code**
```
error=0.5,code=503  # Won't work
```

**Correct:**
```
error=503:0.5
```

**Wrong: Expecting merge**
```
latency=100ms,order-api:error=0.5
# order-api gets only error, not latency
```

**Correct:**
```
order-api:error=0.5,latency=100ms
# order-api gets both
```

**Wrong: Mixed units**
```
latency=100ms-200  # Parse error
```

**Correct:**
```
latency=100-200ms  # or 100ms-200ms
```

## URL Encoding

When using query parameters, encode special characters:

```bash
# Spaces and special chars
curl 'http://localhost:8080/?behavior=latency%3D100ms%2Cerror%3D0.5'

# Or use quotes
curl 'http://localhost:8080/?behavior=latency=100ms,error=0.5'
```

## See Also

- [Behavior Testing Guide](../guides/behavior-testing.md) - Usage examples and scenarios
- [DSL Reference](dsl-spec.md) - Configure default behaviors
- [Developer Reference](../../pkg/service/behavior/QUICK_REFERENCE.md) - Implementation details

