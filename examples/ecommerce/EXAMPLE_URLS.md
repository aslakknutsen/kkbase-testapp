# E-Commerce TestApp - Example URLs & Use Cases

This guide provides example URLs to test all features of the TestService using the e-commerce application.

## Application Topology

```
web (HTTP)
  ├─→ order-api (gRPC)
  │    ├─→ product-api (HTTP)
  │    │    ├─→ product-db (HTTP)
  │    │    └─→ product-cache (HTTP)
  │    ├─→ payment-api (gRPC)
  │    │    └─→ payment-db (HTTP)
  │    └─→ order-db (HTTP)
  └─→ product-api (HTTP)
       ├─→ product-db (HTTP)
       └─→ product-cache (HTTP)
```

## Prerequisites

Deploy the ecommerce example:
```bash
cd testapp
./testgen generate examples/ecommerce/app.yaml -o output
kubectl apply -f output/ecommerce/

# Port-forward for local testing (or use ingress)
kubectl port-forward -n frontend svc/web 8080:8080
```

## Basic Examples

### 1. Simple Request - No Behavior
```bash
curl http://localhost:8080/

# Via Gateway (if configured)
curl https://shop.local/ --insecure
```

**Returns:**
- Service info (name, pod, namespace, node)
- Trace ID and Span ID
- Full call chain showing all upstream services
- Response times for each hop

### 2. Health Check
```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

**Returns:** `OK`

## Behavior Injection Examples

### 3. Add Latency
```bash
# Fixed 500ms latency (applies to ALL services in call chain)
curl "http://localhost:8080/?behavior=latency=500ms"

# Random latency between 100-1000ms
curl "http://localhost:8080/?behavior=latency=100-1000ms"
```

**Observe:**
- Response takes longer
- `behaviors_applied: ["latency"]` in response
- Call chain shows increased duration
- Traces show latency span

### 3a. Targeted Latency - Specific Service Only
```bash
# Add latency ONLY to product-api
curl "http://localhost:8080/?behavior=product-api:latency=500ms"

# Add latency ONLY to order-api
curl "http://localhost:8080/?behavior=order-api:latency=1000ms"

# Multiple targets with different behaviors
curl "http://localhost:8080/?behavior=product-api:latency=300ms,order-api:latency=100ms"
```

**Observe:**
- Only the targeted service shows increased duration
- Other services in the call chain are unaffected
- Response shows which service applied the behavior

### 4. Inject Errors
```bash
# 50% error rate (returns 500)
curl "http://localhost:8080/?behavior=error=0.5"

# 100% error rate with specific code (503)
curl "http://localhost:8080/?behavior=error=1.0,code=503"

# 10% error rate
curl "http://localhost:8080/?behavior=error=0.1"
```

**Observe:**
- Random HTTP 500/503 responses
- Error appears in metrics
- Trace shows error status

### 5. CPU Spike
```bash
# Burn CPU for a short period
curl "http://localhost:8080/?behavior=cpu=200m"

# Longer CPU spike
curl "http://localhost:8080/?behavior=cpu=500m,duration=5s"
```

**Observe:**
- Response is slower
- CPU metrics spike in Prometheus
- Container resource usage increases

### 6. Memory Pressure
```bash
# Allocate 100MB
curl "http://localhost:8080/?behavior=memory=100Mi"

# Gradual memory leak
curl "http://localhost:8080/?behavior=memory=leak"
```

**Observe:**
- Memory metrics increase
- Container memory usage grows

## Combined Behaviors

### 7. Realistic Production Simulation
```bash
# 5% error rate + 50-200ms latency (applies to ALL services)
curl "http://localhost:8080/?behavior=latency=50-200ms,error=0.05"
```

### 8. Degraded Service
```bash
# High latency + errors + resource pressure (ALL services)
curl "http://localhost:8080/?behavior=latency=500-2000ms,error=0.2,cpu=300m"
```

### 9. Cascading Failure Scenario
```bash
# Frontend triggers, which propagates to upstreams
curl "http://localhost:8080/?behavior=error=0.3,latency=1000ms"
```

**Observe:**
- Multiple services slow down
- Error rates increase across the call chain
- Traces show propagation

### 9a. Targeted Cascading Failure
```bash
# Simulate database being slow, affecting everything upstream
curl "http://localhost:8080/?behavior=product-db:latency=2000ms,order-db:latency=2000ms"

# Simulate payment service failing
curl "http://localhost:8080/?behavior=payment-api:error=1.0"

# Simulate product-api being slow AND returning errors
curl "http://localhost:8080/?behavior=product-api:latency=1000ms,product-api:error=0.5"
```

**Observe:**
- Only the targeted services exhibit the behavior
- Upstream services wait for slow downstream services
- Call chain shows exactly where the bottleneck is

### 9b. Mixed Global and Targeted Behaviors
```bash
# Global 5% error + product-api gets extra 500ms latency
curl "http://localhost:8080/?behavior=error=0.05,product-api:latency=500ms"

# All services get slight latency, but databases get much more
curl "http://localhost:8080/?behavior=latency=10ms,product-db:latency=500ms,order-db:latency=500ms"

# Simulate realistic scenario: baseline latency + specific service degradation
curl "http://localhost:8080/?behavior=\
latency=20-50ms,\
product-api:latency=200-800ms,\
product-api:error=0.1,\
payment-api:latency=100-300ms"
```

**Observe:**
- Global behaviors apply to all services
- Targeted behaviors override or add to global behaviors
- Can simulate realistic production scenarios

## Cross-Protocol Testing

### 10. HTTP → gRPC → HTTP Call Chain
```bash
# Default behavior calls order-api (gRPC) which calls product-api (HTTP)
curl http://localhost:8080/

# Check the response JSON:
# - web.protocol = "http"
# - upstream_calls[0].name = "order-api", protocol = "grpc"
# - upstream_calls[0].upstream_calls[0].name = "product-api", protocol = "http"
```

### 11. Direct Service Testing

Test individual services directly:

```bash
# Test order-api (gRPC) - requires grpcurl
grpcurl -plaintext -d '{"behavior": "latency=100ms"}' \
  order-api.orders.svc.cluster.local:9090 \
  testservice.TestService/Call

# Test product-api (HTTP)
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://product-api.products.svc.cluster.local:8080/?behavior=latency=50ms

# Test payment-api (gRPC)
grpcurl -plaintext \
  payment-api.payments.svc.cluster.local:9090 \
  testservice.TestService/Call
```

## Trace Correlation

### 12. Get Trace ID and View in Jaeger
```bash
# Make request and extract trace ID
TRACE_ID=$(curl -s http://localhost:8080/ | jq -r '.trace_id')
echo "Trace ID: $TRACE_ID"

# Open in Jaeger (if deployed)
echo "View trace: http://localhost:16686/trace/$TRACE_ID"
```

### 13. Request with Custom Header
```bash
# Pass behavior via header instead of query param
curl -H "X-Behavior: latency=100ms,error=0.1" http://localhost:8080/
```

## Load Testing Scenarios

### 14. Steady Load
```bash
# Generate 100 requests per second for 1 minute
for i in {1..6000}; do
  curl -s http://localhost:8080/ > /dev/null &
  sleep 0.01
done
```

### 15. Burst Pattern
```bash
# Send bursts of 50 requests
for burst in {1..10}; do
  echo "Burst $burst"
  for i in {1..50}; do
    curl -s http://localhost:8080/ > /dev/null &
  done
  sleep 5
done
```

### 16. Progressive Load with Failures
```bash
# Start with low error rate, increase over time
for error_rate in 0.01 0.05 0.10 0.20 0.50; do
  echo "Testing with error rate: $error_rate"
  for i in {1..100}; do
    curl -s "http://localhost:8080/?behavior=error=$error_rate" > /dev/null &
  done
  sleep 10
done
```

## Advanced Scenarios

### 17. Simulate Timeout
```bash
# Request with extreme latency (will timeout in many systems)
curl --max-time 5 "http://localhost:8080/?behavior=latency=10s"

# Target specific service to timeout
curl --max-time 5 "http://localhost:8080/?behavior=order-api:latency=10s"
```

### 18. Chaos Testing - Random Behaviors
```bash
# Script to randomize behaviors across different services
BEHAVIORS=(
  "latency=10ms"
  "latency=100-500ms"
  "error=0.05"
  "error=0.5"
  "latency=200ms,error=0.1"
  "cpu=300m"
  "product-api:latency=1000ms"
  "order-api:error=0.5"
  "payment-api:latency=500ms,payment-api:error=0.2"
  "product-db:latency=200-1000ms"
  ""
)

for i in {1..100}; do
  BEHAVIOR=${BEHAVIORS[$RANDOM % ${#BEHAVIORS[@]}]}
  curl -s "http://localhost:8080/?behavior=$BEHAVIOR" | jq '.code, .duration'
  sleep 0.5
done
```

### 18a. Targeted Chaos - Test Each Service Independently
```bash
# Test each service's behavior independently
SERVICES=(
  "web"
  "order-api"
  "product-api"
  "payment-api"
  "product-db"
  "order-db"
  "payment-db"
)

for service in "${SERVICES[@]}"; do
  echo "Testing $service with latency..."
  curl -s "http://localhost:8080/?behavior=$service:latency=500ms" | \
    jq ".upstream_calls[] | select(.name==\"$service\") | {name, duration, behaviors_applied}"
  
  echo "Testing $service with errors..."
  curl -s "http://localhost:8080/?behavior=$service:error=1.0" | \
    jq ".upstream_calls[] | select(.name==\"$service\") | {name, code, error}"
  
  sleep 1
done
```

### 19. Test Specific Service Behavior Impact
```bash
# Test how product-api latency affects overall response
curl -s "http://localhost:8080/?behavior=product-api:latency=1000ms" | \
  jq '{
    total_duration: .duration,
    product_api_duration: (.upstream_calls[] | select(.name=="product-api") | .duration)
  }'

# Test how database latency propagates
curl -s "http://localhost:8080/?behavior=product-db:latency=500ms" | \
  jq '{
    total: .duration,
    product_api: (.upstream_calls[] | select(.name=="product-api") | {
      duration,
      db_duration: (.upstream_calls[] | select(.name=="product-db") | .duration)
    })
  }'

# Observe the call chain with targeted behavior
curl http://localhost:8080/?behavior=order-api:latency=300ms | \
  jq '.upstream_calls[] | {name, duration, behaviors_applied}'
```

## Observability Examples

### 20. Monitor Metrics
```bash
# View metrics endpoint
curl http://localhost:8080:9091/metrics | grep testservice_requests_total

# Filter by status code
curl http://localhost:8080:9091/metrics | grep -E 'testservice_requests_total.*status="[45]'
```

### 21. View Call Chain Structure
```bash
# Pretty print the full call chain
curl -s http://localhost:8080/ | jq '{
  service: .service.name,
  duration: .duration,
  trace_id: .trace_id,
  upstreams: [.upstream_calls[] | {
    name: .name,
    protocol: .protocol,
    duration: .duration,
    code: .code,
    nested: [.upstream_calls[]? | .name]
  }]
}'
```

### 22. Extract All Services in Call Chain
```bash
# Recursively extract all service names
curl -s http://localhost:8080/ | jq '
  def services: .service.name, (.upstream_calls[]? | services);
  [services] | unique
'
```

## Performance Testing

### 23. Measure Service Response Time
```bash
# Use curl timing
curl -w "@-" -o /dev/null -s "http://localhost:8080/?behavior=latency=100ms" <<'EOF'
    time_namelookup:  %{time_namelookup}\n
       time_connect:  %{time_connect}\n
    time_appconnect:  %{time_appconnect}\n
   time_pretransfer:  %{time_pretransfer}\n
      time_redirect:  %{time_redirect}\n
 time_starttransfer:  %{time_starttransfer}\n
                    ----------\n
         time_total:  %{time_total}\n
EOF
```

### 24. Benchmark Different Behaviors
```bash
# Compare response times
echo "=== Baseline ==="
time curl -s http://localhost:8080/ > /dev/null

echo "=== With 100ms latency ==="
time curl -s "http://localhost:8080/?behavior=latency=100ms" > /dev/null

echo "=== With CPU load ==="
time curl -s "http://localhost:8080/?behavior=cpu=200m" > /dev/null
```

## Debugging & Troubleshooting

### 25. Verify Upstream Connectivity
```bash
# Check if all upstreams are reachable in the response
curl -s http://localhost:8080/ | jq '.upstream_calls[] | {
  name: .name,
  code: .code,
  error: .error,
  duration: .duration
}'
```

### 26. Test Error Propagation
```bash
# Force error at different levels
curl -s "http://localhost:8080/?behavior=error=1.0" | jq '{
  my_code: .code,
  my_error: .body,
  upstream_codes: [.upstream_calls[]?.code]
}'
```

### 27. Validate Trace Propagation
```bash
# Check trace ID consistency
curl -s http://localhost:8080/ | jq '{
  root_trace: .trace_id,
  upstream_traces: [.upstream_calls[]? | .uri, .protocol]
}'
```

## Sample Expected Responses

### Successful Request
```json
{
  "service": {
    "name": "web",
    "version": "1.0.0",
    "namespace": "frontend",
    "pod": "web-abc123",
    "node": "node-1",
    "protocol": "http"
  },
  "start_time": "2025-10-24T12:34:56.789Z",
  "end_time": "2025-10-24T12:34:56.912Z",
  "duration": "123.456789ms",
  "code": 200,
  "body": "Hello from web (HTTP)",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "upstream_calls": [
    {
      "name": "order-api",
      "uri": "grpc://order-api.orders.svc.cluster.local:9090",
      "protocol": "grpc",
      "duration": "89.123456ms",
      "code": 200,
      "upstream_calls": [...]
    },
    {
      "name": "product-api",
      "uri": "http://product-api.products.svc.cluster.local:8080",
      "protocol": "http",
      "duration": "45.678901ms",
      "code": 200,
      "upstream_calls": [...]
    }
  ],
  "behaviors_applied": ["latency"]
}
```

### Error Response
```json
{
  "service": {...},
  "code": 500,
  "body": "Injected error: 500",
  "behaviors_applied": ["error"]
}
```

## Targeted Behavior Syntax Reference

### Service-Targeted Format
```
service-name:behavior=value
```

### Examples
```bash
# Single service, single behavior
product-api:latency=500ms

# Single service, multiple behaviors
order-api:latency=100ms,order-api:error=0.5

# Multiple services
product-api:latency=300ms,order-api:error=0.5

# Mix targeted and global
product-api:latency=500ms,error=0.05

# Complex scenario
behavior=\
  product-api:latency=200-800ms,\
  product-api:error=0.1,\
  order-api:latency=100ms,\
  payment-api:latency=50-150ms,\
  latency=10ms,\
  error=0.01
```

### Precedence Rules
- **Specific overrides global**: If both `product-api:latency=500ms` and `latency=100ms` are present, product-api gets 500ms
- **Last wins for same service**: `order-api:latency=100ms,order-api:latency=200ms` → order-api gets 200ms
- **Global applies to all**: `latency=50ms` applies to every service unless overridden

## Tips for Effective Testing

1. **Start Simple**: Begin with basic requests, then add behaviors
2. **Use jq**: Parse JSON responses for easier analysis
3. **Check Traces**: Always verify trace propagation in Jaeger
4. **Monitor Metrics**: Watch Prometheus for real-time behavior effects
5. **Test Protocols**: Verify both HTTP and gRPC call chains work
6. **Error Scenarios**: Test how errors propagate through the chain
7. **Load Test**: Generate realistic load to test observability under pressure
8. **Target Specific Services**: Use `service:behavior` syntax to isolate testing
9. **Test Cascading Effects**: See how one slow service affects the entire chain
10. **Mix Global and Targeted**: Simulate realistic scenarios with baseline + specific degradation

## Integration with Monitoring

```bash
# Query Prometheus for request rates
curl 'http://prometheus:9090/api/v1/query?query=rate(testservice_requests_total[5m])'

# Query for error rate by service
curl 'http://prometheus:9090/api/v1/query?query=\
  sum(rate(testservice_requests_total{status=~"5.."}[5m])) by (service)\
  /sum(rate(testservice_requests_total[5m])) by (service)'

# Query for P95 latency by service
curl 'http://prometheus:9090/api/v1/query?query=\
  histogram_quantile(0.95,\
    rate(testservice_request_duration_seconds_bucket[5m])\
  )'

# View traces in Jaeger
open http://localhost:16686
# Search for service: "web", operation: "http.request"
# Filter by tags: behavior.applied="latency"
```

## Real-World Scenario Examples

### Scenario 1: Database Bottleneck Investigation
```bash
# Simulate slow database
curl "http://localhost:8080/?behavior=product-db:latency=1000ms"

# Observe impact on all services that depend on it
# Watch how product-api response time increases
# See how web service total time is affected
```

### Scenario 2: Payment Service Outage
```bash
# Simulate payment service down
curl "http://localhost:8080/?behavior=payment-api:error=1.0"

# Observe:
# - Order processing fails
# - Error propagates to frontend
# - Other services (product-api) still work
```

### Scenario 3: Gradual Degradation
```bash
# Start with normal operation
curl "http://localhost:8080/"

# Add slight latency to database
curl "http://localhost:8080/?behavior=product-db:latency=100ms"

# Increase database latency
curl "http://localhost:8080/?behavior=product-db:latency=500ms"

# Add errors to overloaded service
curl "http://localhost:8080/?behavior=product-db:latency=1000ms,product-db:error=0.2"
```

### Scenario 4: Multi-Service Degradation
```bash
# Realistic scenario: multiple services affected differently
curl "http://localhost:8080/?behavior=\
  product-api:latency=200-400ms,\
  order-api:latency=100-300ms,\
  payment-api:latency=50-200ms,\
  product-db:latency=300-600ms,\
  error=0.02"

# All services get 2% baseline error
# Each service has realistic latency variance
# Databases are slowest (simulating disk I/O)
```

### Scenario 5: Canary Testing Simulation
```bash
# Simulate canary version with higher error rate
# (In real deployment, this would target specific pods)
curl "http://localhost:8080/?behavior=order-api:error=0.15,latency=10ms"

# Compare to stable version
curl "http://localhost:8080/?behavior=order-api:error=0.01,latency=10ms"
```

---

**Pro Tip**: Combine these examples with your kkbase monitoring system to validate end-to-end observability!

**Advanced Tip**: Use targeted behaviors to test how your monitoring system detects and alerts on specific service degradation patterns!

