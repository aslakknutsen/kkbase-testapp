# Testing Strategies

Comprehensive testing approaches for TestApp, from unit tests to end-to-end scenarios.

## Testing Levels

### Unit Testing

Test individual components in isolation.

**Behavior Engine**

Location: `pkg/service/behavior/engine_test.go`

Run tests:
```bash
go test -v ./pkg/service/behavior/
```

With coverage:
```bash
go test -cover ./pkg/service/behavior/
```

**Test Categories:**
- Behavior parsing (latency, error, CPU, memory)
- Service-targeted behavior chains
- Error probability logic
- Behavior serialization
- Behavior application

**Key Test Scenarios:**
- Parse basic behaviors: `latency=100ms`, `error=0.5`
- Parse targeted behaviors: `service:latency=500ms`
- Precedence rules: specific vs global
- Round-trip serialization
- Statistical error rates (1000 iteration sampling)

### Integration Testing

Test components working together in a real cluster.

**Generate and Deploy:**

```bash
# Generate manifests
./testgen generate examples/simple-web/app.yaml -o output

# Validate manifests
kubectl apply --dry-run=client -f output/simple-web/

# Deploy
kubectl apply -f output/simple-web/

# Wait for ready
kubectl wait --for=condition=ready pod -l part-of=simple-web --timeout=120s
```

**Test Service Communication:**

```bash
# Port forward
kubectl port-forward svc/frontend 8080:8080

# Test basic request
curl http://localhost:8080/

# Verify call chain
curl -s http://localhost:8080/ | jq '.upstream_calls[].name'
```

**Test Protocol Translation:**

Create services with mixed protocols:

```yaml
services:
  - name: web
    protocols: [http]
    upstreams: [api]
  - name: api
    protocols: [grpc]
    upstreams: [db]
  - name: db
    protocols: [http]
```

Verify HTTP → gRPC → HTTP works:

```bash
curl -s http://localhost:8080/ | \
  jq '.upstream_calls[] | {name, protocol}'
```

Expected:
```json
{"name":"api","protocol":"grpc"}
```

**Test Trace Propagation:**

Deploy with Jaeger, make requests, verify traces in UI:

```bash
TRACE_ID=$(curl -s http://localhost:8080/ | jq -r '.trace_id')
echo "http://localhost:16686/trace/$TRACE_ID"
```

Verify all services appear in the trace.

### End-to-End Testing

Test complete scenarios including observability.

**Scenario 1: Basic Application Flow**

1. Deploy application
2. Generate traffic
3. Verify responses
4. Check metrics in Prometheus
5. View traces in Jaeger
6. Review logs

**Scenario 2: Behavior Injection**

Test behavior engine end-to-end:

```bash
# No behavior (baseline)
hey -n 100 -c 5 http://localhost:8080/

# With latency
hey -n 100 -c 5 'http://localhost:8080/?behavior=latency=100ms'

# With errors
hey -n 100 -c 5 'http://localhost:8080/?behavior=error=0.2'
```

Verify in Prometheus:
```promql
rate(testservice_requests_total{status=~"5.."}[1m])
```

**Scenario 3: Multi-Namespace Communication**

Deploy e-commerce example:

```bash
./testgen generate examples/ecommerce/app.yaml -o output
kubectl apply -f output/ecommerce/
```

Test cross-namespace calls:

```bash
kubectl port-forward -n frontend svc/web 8080:8080
curl http://localhost:8080/
```

Verify upstreams in different namespaces are called:

```bash
curl -s http://localhost:8080/ | \
  jq '.upstream_calls[] | {name, uri}'
```

## Testing Patterns

### Test Targeted Behaviors

```bash
# Test single service
curl '/?behavior=product-api:latency=500ms' | \
  jq '.upstream_calls[] | select(.name=="product-api") | .duration'

# Verify only product-api is affected
curl '/?behavior=product-api:latency=500ms' | \
  jq '.upstream_calls[] | select(.name=="order-api") | .duration'
```

### Test Error Handling

```bash
# Inject high error rate
for i in {1..20}; do
  curl -s '/?behavior=order-api:error=0.8' | jq '.upstream_calls[] | select(.name=="order-api") | .code'
done | sort | uniq -c
```

Expected: ~80% showing error codes, ~20% showing 200.

### Test Load Scenarios

```bash
# Baseline
hey -n 1000 -c 10 -q 100 http://localhost:8080/

# With realistic latency
hey -n 1000 -c 10 -q 100 'http://localhost:8080/?behavior=latency=10-50ms'

# With service degradation
hey -n 1000 -c 10 -q 100 'http://localhost:8080/?behavior=product-api:latency=200-500ms,error=0.1'
```

Monitor metrics during load test:

```bash
watch -n 1 "curl -s http://localhost:9091/metrics | grep testservice_requests_total"
```

## Test Scenarios

### Scenario: Database Slowdown

Simulate slow database affecting application:

```bash
curl '/?behavior=product-db:latency=1s,order-db:latency=1s'
```

**Verify:**
- All services calling databases are slow
- Services not calling databases unaffected
- Timeouts trigger correctly
- Traces show slowdown at database layer

### Scenario: Cascading Failure

Simulate cascading failure from one service:

```bash
curl '/?behavior=order-api:error=0.8'
```

**Verify:**
- Order API fails frequently
- Downstream services also affected
- Error tracking works
- Circuit breakers trigger (if configured)

### Scenario: Mixed Protocol Chain

Test HTTP → gRPC → HTTP:

```bash
curl -s http://localhost:8080/ | jq '.'
```

**Verify:**
- All protocols in response
- Trace IDs propagate
- Metrics collected for all protocols
- Behaviors apply across protocols

### Scenario: Path-Based Routing

Test path routing works:

```bash
# Different paths route to different services
curl http://localhost:8080/orders | jq '.upstream_calls[].name'
curl http://localhost:8080/products | jq '.upstream_calls[].name'
```

**Verify:**
- Correct upstream called for each path
- Path stripping works
- 404 returned for unknown paths

## Observability Testing

### Test Metrics Collection

```bash
# Generate traffic
for i in {1..100}; do curl -s http://localhost:8080/ > /dev/null; done

# Check metrics
curl http://localhost:9091/metrics | grep testservice_requests_total
```

**Verify:**
- Request counters increase
- Duration histograms populated
- Labels correct (service, method, status)

### Test Trace Propagation

```bash
# Make request, get trace ID
TRACE_ID=$(curl -s http://localhost:8080/ | jq -r '.trace_id')

# Find trace in Jaeger
curl "http://localhost:16686/api/traces/$TRACE_ID" | jq '.data[0].spans | length'
```

**Verify:**
- Trace has spans for all services
- Parent-child relationships correct
- Timing makes sense
- Tags present (service.name, etc.)

### Test Log Correlation

```bash
# Make request with trace ID
TRACE_ID=$(curl -s http://localhost:8080/ | jq -r '.trace_id')

# Find logs with same trace ID
kubectl logs -l part-of=simple-web | grep "$TRACE_ID"
```

**Verify:**
- Logs from multiple services have same trace_id
- Log entries correlate to trace spans
- Error logs match error spans

## Performance Testing

### Baseline Performance

Measure without behaviors:

```bash
hey -n 10000 -c 50 http://localhost:8080/
```

Record:
- Requests/sec
- Mean latency
- P95/P99 latency
- Error rate

### With Behavior Overhead

Measure with behaviors:

```bash
hey -n 10000 -c 50 'http://localhost:8080/?behavior=latency=10ms'
```

Compare to baseline to understand behavior overhead (should be minimal).

### Resource Usage

Monitor resource usage during load:

```bash
kubectl top pods -l part-of=simple-web
```

Verify:
- CPU usage reasonable
- Memory stable
- No resource leaks

## Continuous Testing

### Pre-Deployment Validation

```bash
# Generate and validate
./testgen generate examples/simple-web/app.yaml -o /tmp/test
kubectl apply --dry-run=client -f /tmp/test/

# Run unit tests
go test ./...

# Build service
make build
```

### Post-Deployment Smoke Tests

```bash
# Deploy
kubectl apply -f output/simple-web/

# Wait for ready
kubectl wait --for=condition=ready pod -l part-of=simple-web --timeout=120s

# Smoke test
curl -f http://localhost:8080/ || exit 1

# Check metrics endpoint
curl -f http://localhost:9091/metrics || exit 1
```

### Regression Testing

Maintain a suite of test requests:

```bash
#!/bin/bash
# test-suite.sh

# Test 1: Basic request
curl -f http://localhost:8080/ || exit 1

# Test 2: With latency
curl -f 'http://localhost:8080/?behavior=latency=100ms' || exit 1

# Test 3: With targeted behavior
curl -f 'http://localhost:8080/?behavior=api:latency=200ms' || exit 1

# Test 4: Verify response format
curl -s http://localhost:8080/ | jq -e '.trace_id' || exit 1

echo "All tests passed"
```

## Best Practices

**Do:**
- Test at multiple levels (unit, integration, E2E)
- Use behaviors to simulate real-world scenarios
- Verify observability signals (metrics, traces, logs)
- Test protocol translation
- Test cross-namespace communication
- Monitor resource usage
- Automate regression tests

**Don't:**
- Test only with perfect conditions
- Ignore error paths
- Skip observability verification
- Test in isolation without realistic load
- Forget to test cleanup (service deletion)

## See Also

- [Behavior Testing Guide](behavior-testing.md) - Detailed behavior syntax
- [Observability](../concepts/observability.md) - Metrics, traces, logs
- [Jaeger Setup](jaeger-setup.md) - Distributed tracing
- [Behavior Engine Tests](../../pkg/service/behavior/TEST_SCENARIOS.md) - Unit test details

