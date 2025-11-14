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

## Panic Behaviors

Trigger pod crash/restart for testing resilience.

### Syntax

```
panic=<probability>
```

**Examples:**
- `panic=0.5` - 50% chance to crash pod
- `panic=0.1` - 10% chance to crash pod
- `panic=1.0` - Always crash pod (100%)

**Use Cases:**
- Test pod restart behavior
- Test circuit breaker triggers
- Test service mesh resilience
- Test cascading failure scenarios
- Verify liveness probe responses

**Warning:** This will actually crash your pods! Use carefully with low probabilities initially.

## Crash on Invalid Config File

Trigger pod crash when mounted config files contain invalid content. Simulates config-related crashes for testing ConfigMap propagation and error handling.

### Syntax

```
crash-if-file=<file_path>:<invalid_content>
```

**File Path:** Absolute path to config file (typically mounted via ConfigMap)

**Invalid Content:** Semicolon-separated list of strings that trigger crash

### Examples

**Single invalid string:**
```
crash-if-file=/config/app.conf:invalid
```

**Multiple invalid strings:**
```
crash-if-file=/config/db.conf:bad;error;fail
```

**Combined with other behaviors:**
```
latency=100ms,crash-if-file=/config/app.conf:invalid
```

### Environment Variable Configuration

Set `CRASH_ON_FILE_CONTENT` to check on startup:

```bash
CRASH_ON_FILE_CONTENT="/config/app.conf:invalid"
```

**Multiple files (pipe-separated):**
```bash
CRASH_ON_FILE_CONTENT="/config/app.conf:invalid|/config/db.conf:bad;error"
```

### ConfigMap Scenario

This behavior is designed for realistic ConfigMap propagation testing:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  app.conf: |
    database_url=postgres://db:5432/myapp
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  template:
    spec:
      containers:
      - name: testservice
        env:
        - name: CRASH_ON_FILE_CONTENT
          value: "/config/app.conf:invalid"
        volumeMounts:
        - name: config
          mountPath: /config
      volumes:
      - name: config
        configMap:
          name: app-config
```

**To trigger crash:**
1. Update ConfigMap: `database_url=invalid`
2. Wait for kubelet sync (~60 seconds)
3. Pod crashes on next request

### Behavior Details

**Check Timing:**
- **Startup:** Checked once before servers start (if `CRASH_ON_FILE_CONTENT` set)
- **Per-Request:** Checked before each HTTP/gRPC request (if behavior injected)

**Match Logic:**
- Uses simple substring matching
- Any configured invalid string found in file triggers crash
- Case-sensitive

**Error Handling:**
- File read errors: Logged but don't crash (fail-safe)
- Missing files: Logged but don't crash

**Logging:**
- Clear error message with file path and matched content
- Uses `Fatal` log level before crash

### Use Cases

**ConfigMap Propagation Bug:**
```bash
# Scenario: Bad config value propagates to running pods
curl "/?behavior=crash-if-file=/config/app.conf:invalid"
```

**Database Connection Failure:**
```bash
CRASH_ON_FILE_CONTENT="/config/db.conf:connection_failed"
```

**Multi-Environment Config:**
```bash
# Crash if production config in dev environment
CRASH_ON_FILE_CONTENT="/config/env:production"
```

**Complex Scenarios:**
```bash
# Combine with other behaviors
curl "/?behavior=latency=500ms,crash-if-file=/config/app.conf:invalid,error=0.1"
```

### Runtime Injection

**HTTP:**
```bash
curl "http://service:8080/?behavior=crash-if-file=/config/app.conf:invalid"
```

**gRPC:**
```go
req := &pb.CallRequest{
    Behavior: "crash-if-file=/config/app.conf:invalid",
}
```

**Header:**
```bash
curl -H "X-Behavior: crash-if-file=/config/app.conf:invalid" http://service:8080/
```

### Observability

Applied behaviors appear in response (before crash):

```json
{
  "behaviors_applied": [
    "crash-if-file:/config/app.conf:invalid"
  ]
}
```

**Note:** Multiple invalid strings are separated by semicolons (`;`) to avoid conflicts with the comma-separated behavior syntax.

**Log Output:**
```
Fatal: Config file contains invalid content - crashing as configured
  file=/config/app.conf matched_content=invalid
```

## Error on Invalid Secret/Config File

Return HTTP/gRPC errors when mounted files (Secrets or ConfigMaps) contain invalid content. Unlike `crash-if-file`, this behavior lets the service continue running while returning errors on requests.

### Syntax

```
error-if-file=<file_path>:<invalid_content>:<error_code>
```

**File Path:** Absolute path to file (typically mounted via Secret or ConfigMap)

**Invalid Content:** Semicolon-separated list of strings that trigger error

**Error Code:** HTTP status code (optional, defaults to 401)

### Examples

**Single invalid string with default 401:**
```
error-if-file=/var/run/secrets/api-key:bad
```

**Single invalid string with custom code:**
```
error-if-file=/var/run/secrets/api-key:invalid:403
```

**Multiple invalid strings:**
```
error-if-file=/config/auth:bad;error;fail:401
```

**Combined with other behaviors:**
```
latency=100ms,error-if-file=/var/run/secrets/key:bad:401
```

### Environment Variable Configuration

Set `ERROR_ON_FILE_CONTENT` to apply on all requests:

```bash
ERROR_ON_FILE_CONTENT="/var/run/secrets/api-key:invalid:401"
```

**Multiple files (pipe-separated):**
```bash
ERROR_ON_FILE_CONTENT="/var/run/secrets/key:bad:401|/config/auth:fail:403"
```

### Secret Mounting Scenario

This behavior is designed for realistic Secret rotation and authentication testing:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: api-credentials
type: Opaque
stringData:
  api-key: "valid_key_123"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  template:
    spec:
      containers:
      - name: testservice
        env:
        - name: ERROR_ON_FILE_CONTENT
          value: "/var/run/secrets/api-key:bad_credential:401"
        volumeMounts:
        - name: credentials
          mountPath: /var/run/secrets
          readOnly: true
      volumes:
      - name: credentials
        secret:
          secretName: api-credentials
```

**To trigger errors:**
1. Update Secret: `api-key: "bad_credential"`
2. Wait for kubelet sync (~60 seconds)
3. Service returns 401 errors on all requests (but stays running)

### Behavior Details

**Check Timing:**
- **Startup:** Validates syntax and logs if condition is met (service still starts)
- **Per-Request:** Checked before each HTTP/gRPC request, returns error if matched

**Match Logic:**
- Uses simple substring matching
- Any configured invalid string found in file triggers error
- Case-sensitive

**Error Handling:**
- File read errors: Logged but don't return error (fail-safe)
- Missing files: Logged but don't return error

**HTTP Response:**
- Returns configured HTTP status code (default: 401)
- Response body includes validation failure message

**gRPC Response:**
- Maps HTTP codes to gRPC status codes:
  - `400` → `InvalidArgument`
  - `401` → `Unauthenticated`
  - `403` → `PermissionDenied`
  - `404` → `NotFound`
  - `409` → `AlreadyExists`
  - `429` → `ResourceExhausted`
  - `500` → `Internal`
  - `503` → `Unavailable`
  - `504` → `DeadlineExceeded`
  - Others → `Unknown`

**Logging:**
- Warning log level (service continues running)
- Includes file path, matched content, and error code

### Use Cases

**Secret Rotation with Bad Credentials:**
```bash
# Scenario: Rotated secret contains invalid credential
curl "/?behavior=error-if-file=/var/run/secrets/api-key:bad_credential:401"
```

**Database Connection String Validation:**
```bash
ERROR_ON_FILE_CONTENT="/config/db-connection:invalid_host:503"
```

**Multi-Environment Secret Validation:**
```bash
# Return 403 if production credentials in dev environment
ERROR_ON_FILE_CONTENT="/var/run/secrets/env:production:403"
```

**API Key Validation:**
```bash
# Return 401 for revoked API keys
error-if-file=/var/run/secrets/api-key:revoked;expired;invalid:401
```

**Complex Scenarios:**
```bash
# Combine with latency and other errors
curl "/?behavior=latency=500ms,error-if-file=/var/run/secrets/key:bad:401,error=0.1"
```

### Runtime Injection

**HTTP:**
```bash
curl "http://service:8080/?behavior=error-if-file=/var/run/secrets/key:bad:401"
```

**gRPC:**
```go
req := &pb.CallRequest{
    Behavior: "error-if-file=/var/run/secrets/key:bad:401",
}
```

**Header:**
```bash
curl -H "X-Behavior: error-if-file=/var/run/secrets/key:bad:401" http://service:8080/
```

### Observability

Applied behaviors appear in response:

```json
{
  "code": 401,
  "body": "File validation failed: File /var/run/secrets/key contains invalid content: 'bad'",
  "behaviors_applied": [
    "error-if-file:/var/run/secrets/key:bad:401"
  ]
}
```

**Note:** Multiple invalid strings are separated by semicolons (`;`) to avoid conflicts with the comma-separated behavior syntax.

**Log Output:**
```
Warn: File contains invalid content - returning error as configured
  file=/var/run/secrets/api-key matched_content=bad error_code=401
```

### Comparison: error-if-file vs crash-if-file

| Feature | error-if-file | crash-if-file |
|---------|---------------|---------------|
| **Service availability** | Stays running | Pod crashes |
| **Error response** | Returns HTTP/gRPC error | N/A (crashes) |
| **Default code** | 401 Unauthorized | N/A |
| **Use case** | Authentication failures, bad credentials | Config errors, critical failures |
| **Kubernetes impact** | Service degraded but available | Pod restart, potential downtime |
| **Testing scenario** | Secret rotation with bad values | ConfigMap propagation bugs |

**When to use:**
- Use `error-if-file` for authentication/authorization failures where the service should stay up
- Use `crash-if-file` for critical config errors where pod should restart

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

### Memory Spike

Rapidly allocate memory for sudden resource consumption testing.

```
memory=spike:<size>
memory=spike:<size>:<duration>
memory=spike:<percentage>
memory=spike:<percentage>:<duration>
```

**Size Examples:**
- `memory=spike:500Mi` - Allocate 500MB immediately, hold for 10m (default)
- `memory=spike:1Gi:30s` - Allocate 1GB, hold for 30 seconds
- `memory=spike:100Mi:2m` - Allocate 100MB, hold for 2 minutes

**Percentage Examples:**
- `memory=spike:80%` - Allocate 80% of container limit, hold for 10m
- `memory=spike:80%:60s` - Allocate 80% of container limit, hold for 60s

**Size Units:** `Mi` (mebibytes), `Gi` (gibibytes)

**Percentage Behavior:**
- Requires container memory limit detection
- Uses `GOMEMBALLAST` env var or cgroup limits
- Fails gracefully if limit cannot be determined

**Allocation Characteristics:**
- **Immediate**: Allocates memory as fast as possible (unlike leak patterns)
- **Sustained**: Holds allocation for specified duration
- **Clean Release**: Releases memory and triggers GC after duration

**Use Cases:**
- **OOMKilled testing**: Spike beyond container limit to trigger OOM
- **Noisy neighbor simulation**: Test shared node resource contention
- **Memory pressure testing**: Verify behavior under low memory conditions
- **Resource exhaustion**: Combine with CPU spike for full exhaustion

**Example Scenarios:**

Trigger OOM kill:
```
memory=spike:150%:30s
```

Simulate noisy neighbor on shared node:
```
memory=spike:2Gi:5m
```

Combined resource exhaustion:
```
cpu=spike:10s:90,memory=spike:80%:10s
```

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

**Pod crash testing:**
```
panic=0.3
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

**Crashing order service:**
```
order-api:panic=0.3
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

**Extreme chaos with crashes:**
```
order-api:panic=0.1,product-api:error=0.5,latency=100-300ms
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
    "error:503:0.30",
    "panic:0.50"
  ]
}
```

Format includes details:
- Latency: `latency:fixed:100ms` or `latency:range:50ms-200ms`
- Error: `error:503:0.30` (code:probability)
- Panic: `panic:0.50` (probability)
- CPU: `cpu:spike:5s:intensity=90`
- Memory (leak): `memory:leak-slow:10485760:10m0s`
- Memory (spike): `memory:spike:524288000:30s` or `memory:spike:80%:1m0s`

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

