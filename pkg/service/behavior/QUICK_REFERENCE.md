# Behavior Engine Quick Reference

## Common Behavior Patterns

### Latency

```bash
# Fixed latency
latency=100ms

# Random latency (range)
latency=50-200ms
latency=50ms-200ms    # Same as above

# Real-world examples
latency=5ms           # Fast local cache
latency=50-100ms      # Database query
latency=200-500ms     # External API call
latency=1s-3s         # Slow 3rd party service
```

### Error Injection

```bash
# Probability only (default 500 error)
error=0.5             # 50% error rate
error=0.1             # 10% error rate

# Specific error code (100% rate)
error=503             # Always return 503
error=429             # Always return 429

# Probability + custom code
error=503:0.3         # 30% chance of 503
error=429:0.05        # 5% chance of 429 (rate limiting)
```

### CPU Behaviors

```bash
# CPU spike
cpu=spike

# With duration and intensity
cpu=spike:5s          # 5 second spike
cpu=spike:10s:90      # 10 second spike at 90% CPU
```

### Memory Behaviors

```bash
# Memory leak
memory=leak-slow
memory=leak-fast
```

### Combined Behaviors

```bash
# Multiple behaviors on one service
latency=100ms,error=503:0.1    # Latency + occasional errors

# Complex combinations
latency=50-100ms,error=0.05,cpu=spike
```

---

## Service-Targeted Behaviors

### Basic Targeting

```bash
# Target single service
order-api:error=500:0.5

# Target multiple services
order-api:error=0.5,product-api:latency=200ms

# Mix of targeted and global
latency=50ms,order-api:error=0.5
# Result:
#   - order-api: error ONLY (specific overrides global)
#   - other services: latency only
```

### Chaining Behaviors on One Service

```bash
# Multiple behaviors for same service
order-api:error=0.5,latency=100ms
# Result: order-api gets BOTH error and latency

# This is different from:
order-api:error=0.5,other-api:latency=100ms
# Result: each service gets its own behavior
```

### Important: Comma Rules

After a `service:` prefix, comma-separated values continue for that service until you specify a new `service:` prefix.

```bash
# ONE behavior for order-api (error + latency)
order-api:error=0.5,latency=100ms

# TWO behaviors (order-api gets error, product-api gets latency)
order-api:error=0.5,product-api:latency=100ms

# THREE behaviors 
order-api:error=0.5,product-api:latency=100ms,payment-api:error=0.1
```

---

## Real-World Scenarios

### Scenario 1: Simulate Flaky External API

```bash
# product-api has 20% error rate and high latency
curl "http://web:8080/api?behavior=product-api:error=503:0.2,latency=500-1000ms"
```

### Scenario 2: Database Slowdown

```bash
# All *-db services slow
curl "http://web:8080/api?behavior=order-db:latency=500-1500ms,product-db:latency=500-1500ms"
```

### Scenario 3: Cascading Failures

```bash
# order-api has high error rate, others slow down
curl "http://web:8080/api?behavior=order-api:error=0.7,product-api:latency=300ms,payment-api:latency=400ms"
```

### Scenario 4: Rate Limiting

```bash
# Simulate rate limiting on payment service
curl "http://web:8080/api?behavior=payment-api:error=429:0.3"
```

### Scenario 5: Testing Timeout Handling

```bash
# Make service extremely slow to test timeouts
curl "http://web:8080/api?behavior=order-api:latency=5s"
```

### Scenario 6: Global Latency with Specific Overrides

```bash
# Everyone slow, but order-api even slower
curl "http://web:8080/api?behavior=latency=100ms,order-api:latency=500ms"
# Result:
#   - order-api: 500ms latency (specific wins)
#   - others: 100ms latency (global)
```

### Scenario 7: Chaos Testing

```bash
# Random chaos across services
curl "http://web:8080/api?behavior=order-api:error=0.3,latency=100-500ms,product-api:error=0.2,payment-api:latency=200-800ms,error=0.1"
```

---

## Testing Error Handling

### Test Circuit Breakers

```bash
# High error rate to trigger circuit breaker
curl "http://web:8080/api?behavior=order-api:error=0.8"
```

### Test Retry Logic

```bash
# 50% error rate should trigger retries
curl "http://web:8080/api?behavior=order-api:error=0.5"

# Watch logs to see retries in action
```

### Test Fallback Mechanisms

```bash
# Make primary service fail, verify fallback works
curl "http://web:8080/api?behavior=order-api:error=503"
```

---

## Performance Testing

### Baseline Performance

```bash
# No behaviors - measure baseline
curl "http://web:8080/api"
```

### Realistic Load

```bash
# Simulate realistic network latency
curl "http://web:8080/api?behavior=latency=10-50ms"
```

### Stress Test

```bash
# Add latency to simulate load
curl "http://web:8080/api?behavior=latency=100-300ms"
```

### Worst Case

```bash
# Combine high latency and errors
curl "http://web:8080/api?behavior=latency=500ms,error=0.3"
```

---

## Behavior Propagation

When you send a behavior string to a service, it propagates to all downstream services:

```bash
curl "http://web:8080?behavior=order-api:error=0.5"
```

**What happens:**
1. Web service receives behavior string
2. Web parses it, finds no behavior for "web"
3. Web calls order-api and passes behavior string
4. Order-api parses it, finds "order-api:error=0.5", applies error
5. Order-api calls its upstreams, passes behavior string
6. Each upstream extracts its own behavior (if any)

**This enables:**
- Testing deep in the call chain
- Simulating failures anywhere in your architecture
- End-to-end chaos testing

---

## Observability

All applied behaviors are reported in the response:

```json
{
  "service": {"name": "order-api"},
  "code": 500,
  "behaviors_applied": ["error:500:0.50", "latency:fixed"],
  "upstream_calls": [...]
}
```

This helps you:
- Verify behaviors were applied correctly
- Debug test scenarios
- Understand which behaviors affected each service in the chain

---

## Tips and Best Practices

### 1. Start Simple
```bash
# Start with one service, one behavior
order-api:latency=100ms
```

### 2. Build Up Complexity
```bash
# Add more services
order-api:latency=100ms,product-api:latency=50ms

# Add more behaviors
order-api:latency=100ms,error=0.1,product-api:latency=50ms
```

### 3. Use Ranges for Realism
```bash
# Fixed latency is unrealistic
latency=100ms                    # ❌ Too artificial

# Range latency is more realistic  
latency=80-120ms                 # ✅ Better
```

### 4. Test Edge Cases
```bash
# 100% errors
error=500

# No errors (baseline)
# (no error behavior)

# Very rare errors
error=0.01                       # 1% error rate
```

### 5. Combine with Load Testing
```bash
# Use with tools like hey, wrk, k6
hey -n 1000 -c 10 "http://web:8080?behavior=order-api:error=0.2"
```

### 6. Document Your Test Scenarios
```bash
# Create a script with named scenarios
./test-scenarios.sh cascading-failure
./test-scenarios.sh database-slowdown
./test-scenarios.sh rate-limiting
```

---

## Common Mistakes

### ❌ Wrong: Using comma for error code
```bash
error=0.5,code=503               # This won't work!
```

### ✅ Correct: Use colon for error code
```bash
error=503:0.5                    # Correct!
```

---

### ❌ Wrong: Expecting global to merge with specific
```bash
latency=100ms,order-api:error=0.5
```
**Wrong assumption:** order-api gets both latency and error  
**Reality:** order-api gets ONLY error (specific overrides global)

### ✅ Correct: Chain behaviors for same service
```bash
order-api:error=0.5,latency=100ms
```
**Result:** order-api gets BOTH error and latency

---

### ❌ Wrong: Multiple unit specifications
```bash
latency=100ms-200        # Mixed units don't parse
```

### ✅ Correct: Consistent units or smart parsing
```bash
latency=100ms-200ms      # Both have units
latency=100-200ms        # Unit on max, applied to both
```

---

## Quick Test Commands

```bash
# Test your ecommerce app
BASE="http://localhost:8080"

# Baseline
curl -s "$BASE/product" | jq

# Slow order service
curl -s "$BASE/product?behavior=order-api:latency=500ms" | jq

# Failing order service
curl -s "$BASE/product?behavior=order-api:error=500:0.5" | jq '.upstream_calls[] | select(.name=="order-api")'

# Complex scenario
curl -s "$BASE/product?behavior=order-api:error=0.3,latency=100-300ms,product-api:latency=50-100ms" | jq
```

---

## Integration with Kubernetes

Deploy with environment variables:

```yaml
env:
- name: DEFAULT_BEHAVIOR
  value: "latency=10-30ms"  # Simulate network latency for all requests
```

Or use ConfigMaps/Secrets for different environments:

```yaml
# development.yaml
DEFAULT_BEHAVIOR: "latency=50-100ms,error=0.01"

# staging.yaml  
DEFAULT_BEHAVIOR: "latency=20-50ms"

# production.yaml
DEFAULT_BEHAVIOR: ""  # No artificial behaviors
```

