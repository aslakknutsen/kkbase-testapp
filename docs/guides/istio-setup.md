# Istio Setup Guide

Configure TestApp to generate Istio service mesh manifests for advanced traffic management, resilience, and security.

## Overview

TestApp includes native support for Istio service mesh, automatically generating:
- **VirtualService** - Traffic routing and retries
- **DestinationRule** - Circuit breakers, load balancing, mTLS, traffic splitting
- **Gateway** - Ingress routing (alternative to Gateway API)

All mesh features are configured via the YAML DSL and generated alongside Kubernetes resources.

## Prerequisites

### Install Istio

Install Istio in your cluster:

```bash
# Download Istio
curl -L https://istio.io/downloadIstio | sh -
cd istio-*

# Install with demo profile
bin/istioctl install --set profile=demo -y

# Enable automatic sidecar injection
kubectl label namespace default istio-injection=enabled
```

Verify installation:

```bash
kubectl get pods -n istio-system
kubectl get svc -n istio-system
```

## Enabling Istio in TestApp

### Basic Configuration

Enable Istio mesh provider in your DSL:

```yaml
app:
  name: my-app
  namespaces:
    - default
  providers:
    ingress: gateway-api    # or istio-gateway
    mesh: istio             # Enable Istio mesh

services:
  - name: frontend
    namespace: default
    type: Deployment
    replicas: 2
    protocols: [http]
    upstreams: [backend]

  - name: backend
    namespace: default
    type: Deployment
    replicas: 2
    protocols: [http]
```

Generate manifests:

```bash
./testgen generate app.yaml -o output
```

This creates a `40-mesh/` directory with Istio resources:

```
output/my-app/
├── 10-services/
│   ├── frontend-deployment.yaml
│   └── backend-deployment.yaml
└── 40-mesh/
    ├── frontend-virtualservice.yaml
    ├── frontend-destinationrule.yaml
    ├── backend-virtualservice.yaml
    └── backend-destinationrule.yaml
```

Deploy:

```bash
kubectl apply -f output/my-app/
```

## Circuit Breakers

Protect services from cascading failures with circuit breakers.

### App-Level Defaults

Configure circuit breakers for all services:

```yaml
app:
  name: my-app
  providers:
    mesh: istio
  meshDefaults:
    circuitBreaker:
      consecutiveErrors: 5
      interval: 30s
      baseEjectionTime: 30s
      maxEjectionPercent: 50

services:
  - name: frontend
    upstreams: [backend]
  - name: backend
    # Inherits circuit breaker from meshDefaults
```

**Generated DestinationRule:**

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: backend
spec:
  host: backend.default.svc.cluster.local
  trafficPolicy:
    outlierDetection:
      consecutiveGatewayErrors: 5
      consecutive5xxErrors: 5
      interval: 30s
      baseEjectionTime: 30s
      maxEjectionPercent: 50
```

### Service-Level Overrides

Override circuit breaker settings per service:

```yaml
services:
  - name: backend
    mesh:
      circuitBreaker:
        consecutiveErrors: 3      # More sensitive
        interval: 15s
        baseEjectionTime: 60s     # Longer ejection
```

### Testing Circuit Breakers

Test circuit breaker behavior with error injection:

```bash
# Trigger high error rate
curl 'http://frontend:8080/?behavior=backend:error=0.8'

# Watch circuit breaker kick in
kubectl logs -l app=frontend -f
```

After 3-5 consecutive errors, backend pods are temporarily ejected from the load balancer.

## Load Balancing Strategies

Control how traffic is distributed across service instances.

### Available Strategies

```yaml
meshDefaults:
  loadBalancing: ROUND_ROBIN  # or LEAST_REQUEST, RANDOM, PASSTHROUGH
```

**Strategies:**
- `ROUND_ROBIN` - Distribute requests evenly (default)
- `LEAST_REQUEST` - Route to instance with fewest active requests
- `RANDOM` - Random selection
- `PASSTHROUGH` - Use original destination

### Example: Least Request

Best for uneven request durations:

```yaml
app:
  meshDefaults:
    loadBalancing: LEAST_REQUEST

services:
  - name: api
    replicas: 3
    # Requests routed to least busy pod
```

### Per-Service Load Balancing

```yaml
services:
  - name: api
    mesh:
      loadBalancing: LEAST_REQUEST
  
  - name: cache
    mesh:
      loadBalancing: RANDOM
```

## Retry Policies

Automatically retry failed requests for improved reliability.

### Basic Retries

```yaml
meshDefaults:
  retries:
    attempts: 3
    perTryTimeout: 1s
    retryOn: "5xx,reset,connect-failure,refused-stream"
```

**Generated VirtualService:**

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: backend
spec:
  http:
  - route:
    - destination:
        host: backend.default.svc.cluster.local
    retries:
      attempts: 3
      perTryTimeout: 1s
      retryOn: "5xx,reset,connect-failure,refused-stream"
```

### Retry Conditions

Customize when to retry:

```yaml
meshDefaults:
  retries:
    attempts: 5
    perTryTimeout: 2s
    retryOn: "5xx,gateway-error,reset"  # More specific
```

**Common retryOn values:**
- `5xx` - Any 5xx error
- `gateway-error` - 502, 503, 504
- `reset` - Connection reset
- `connect-failure` - Connection failed
- `refused-stream` - HTTP/2 REFUSED_STREAM
- `retriable-4xx` - 409 Conflict

### Testing Retries

```bash
# Inject 50% error rate
curl 'http://frontend:8080/?behavior=backend:error=0.5'

# Check VirtualService logs - should see automatic retries
istioctl proxy-config log deploy/frontend --level debug
```

## Request Timeouts

Set maximum request duration:

```yaml
meshDefaults:
  timeout: 5s  # All requests time out after 5 seconds

services:
  - name: slow-service
    mesh:
      timeout: 30s  # Override: allow 30 seconds
```

Combine with retries:

```yaml
meshDefaults:
  timeout: 10s
  retries:
    attempts: 3
    perTryTimeout: 3s  # Each attempt gets 3s, total max 10s
```

## Mutual TLS (mTLS)

Control encryption between services.

### Istio Mutual TLS (Default)

```yaml
meshDefaults:
  mtls: ISTIO_MUTUAL  # Use Istio-managed certs (default)
```

### Strict mTLS

Require mTLS for all connections:

```yaml
meshDefaults:
  mtls: STRICT  # Reject non-mTLS traffic
```

### Permissive Mode

Accept both mTLS and plaintext (for migration):

```yaml
meshDefaults:
  mtls: PERMISSIVE  # Allow both encrypted and plaintext
```

### Disable mTLS

```yaml
services:
  - name: legacy-service
    mesh:
      mtls: DISABLE  # No encryption
```

## Traffic Splitting (Canary Deployments)

Gradually roll out new versions with traffic splitting.

### Basic Traffic Split

```yaml
services:
  - name: api
    replicas: 3
    mesh:
      trafficSplit:
        - version: v1
          subset: v1
          weight: 90
        - version: v2
          subset: v2
          weight: 10
```

**Generated Manifests:**

**VirtualService:**

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: api
spec:
  http:
  - route:
    - destination:
        host: api.default.svc.cluster.local
        subset: v1
      weight: 90
    - destination:
        host: api.default.svc.cluster.local
        subset: v2
      weight: 10
```

**DestinationRule:**

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: api
spec:
  host: api.default.svc.cluster.local
  subsets:
  - name: v1
    labels:
      version: v1
  - name: v2
    labels:
      version: v2
```

### Deployment Strategy

1. **Deploy v1 (100%):**

```yaml
mesh:
  trafficSplit:
    - version: v1
      subset: v1
      weight: 100
```

2. **Add v2 with 10% traffic:**

Update pods with `version: v2` label, then:

```yaml
mesh:
  trafficSplit:
    - version: v1
      subset: v1
      weight: 90
    - version: v2
      subset: v2
      weight: 10
```

3. **Gradually increase v2:**

```yaml
mesh:
  trafficSplit:
    - version: v1
      subset: v1
      weight: 50
    - version: v2
      subset: v2
      weight: 50
```

4. **Complete rollout:**

```yaml
mesh:
  trafficSplit:
    - version: v2
      subset: v2
      weight: 100
```

### Pod Labels Required

Ensure pods have version labels:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-v2
spec:
  template:
    metadata:
      labels:
        app: api
        version: v2  # Required for subset matching
```

## Service-Level vs App-Level Configuration

### Precedence

Service-level mesh config overrides app-level defaults:

```yaml
app:
  meshDefaults:
    timeout: 5s
    retries:
      attempts: 3
    loadBalancing: ROUND_ROBIN

services:
  - name: fast-api
    # Uses all defaults
    
  - name: slow-api
    mesh:
      timeout: 30s  # Override timeout only
      # Inherits retries and loadBalancing
```

### Per-Service Mesh Control

Disable mesh for specific services:

```yaml
services:
  - name: external-service
    mesh:
      enabled: false  # No Istio resources generated
```

## Istio Gateway vs Gateway API

TestApp supports both ingress options.

### Gateway API (Recommended)

```yaml
app:
  providers:
    ingress: gateway-api
    mesh: istio

services:
  - name: frontend
    ingress:
      enabled: true
      host: app.example.com
      tls: true
```

Generates Gateway API resources (cross-platform).

### Istio Gateway

```yaml
app:
  providers:
    ingress: istio-gateway
    mesh: istio

services:
  - name: frontend
    ingress:
      enabled: true
      host: app.example.com
      tls: true
```

Generates Istio Gateway + VirtualService (Istio-specific).

**Output:**

```
20-gateway/
├── istio-gateway.yaml           # Gateway
└── frontend-ingress-virtualservice.yaml  # VirtualService for ingress routing
```

## Complete Example

Full-featured Istio configuration:

```yaml
app:
  name: ecommerce
  namespaces:
    - frontend
    - backend
  providers:
    ingress: gateway-api
    mesh: istio
  meshDefaults:
    timeout: 5s
    retries:
      attempts: 3
      perTryTimeout: 1s
      retryOn: "5xx,reset,connect-failure"
    circuitBreaker:
      consecutiveErrors: 5
      interval: 30s
      baseEjectionTime: 30s
      maxEjectionPercent: 50
    loadBalancing: LEAST_REQUEST
    mtls: ISTIO_MUTUAL

services:
  - name: web
    namespace: frontend
    replicas: 3
    protocols: [http]
    upstreams: [api]
    ingress:
      enabled: true
      host: shop.example.com
      tls: true

  - name: api
    namespace: backend
    replicas: 5
    protocols: [http]
    upstreams: [product-service, order-service]
    mesh:
      loadBalancing: ROUND_ROBIN  # Override
      trafficSplit:
        - version: v1
          subset: v1
          weight: 90
        - version: v2
          subset: v2
          weight: 10

  - name: product-service
    namespace: backend
    replicas: 3
    protocols: [http]
    mesh:
      timeout: 10s  # Longer timeout for complex queries

  - name: order-service
    namespace: backend
    replicas: 3
    protocols: [http]
    mesh:
      circuitBreaker:
        consecutiveErrors: 3  # More sensitive
```

Generate and deploy:

```bash
./testgen generate ecommerce.yaml -o output
kubectl apply -f output/ecommerce/
```

## Generated Manifest Structure

```
output/ecommerce/
├── 00-namespaces.yaml
├── 10-services/
│   ├── web-deployment.yaml
│   ├── web-service.yaml
│   ├── api-deployment.yaml
│   └── ...
├── 20-gateway/
│   ├── gateway.yaml
│   ├── web-httproute.yaml
│   └── certificates.yaml
└── 40-mesh/
    ├── api-virtualservice.yaml
    ├── api-destinationrule.yaml
    ├── product-service-virtualservice.yaml
    ├── product-service-destinationrule.yaml
    ├── order-service-virtualservice.yaml
    └── order-service-destinationrule.yaml
```

## Troubleshooting

### Verify Istio Injection

Check if pods have sidecars:

```bash
kubectl get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[*].name}{"\n"}{end}'
```

Should show `istio-proxy` container.

### Check VirtualService Status

```bash
kubectl get virtualservice
kubectl describe virtualservice api
```

### Verify DestinationRule

```bash
kubectl get destinationrule
kubectl describe destinationrule api
```

### Test Traffic Splitting

```bash
# Make multiple requests
for i in {1..100}; do
  curl -s http://api:8080/ | jq -r '.service.version'
done | sort | uniq -c

# Should show ~90 v1, ~10 v2
```

### Debug Circuit Breaker

Enable debug logging:

```bash
istioctl proxy-config log deploy/api --level debug
kubectl logs -l app=api -c istio-proxy -f
```

### Check mTLS Status

```bash
istioctl authn tls-check api.default.svc.cluster.local
```

### Common Issues

**Issue: VirtualService not applying**

Check namespace label injection:

```bash
kubectl get namespace default --show-labels
```

Should have `istio-injection=enabled`.

**Issue: Circuit breaker not triggering**

Verify outlier detection settings:

```bash
istioctl proxy-config cluster deploy/api -o json | jq '.[] | select(.name | contains("backend"))'
```

**Issue: Traffic split not working**

Ensure pods have correct version labels:

```bash
kubectl get pods --show-labels | grep version
```

## Performance Considerations

**VirtualService overhead:**
- Minimal latency impact (~1-2ms)
- Retries increase total request time on failures

**Circuit breaker overhead:**
- Negligible CPU/memory impact
- Reduces cascade failures

**mTLS overhead:**
- ~1-2ms additional latency
- Minor CPU increase for encryption

## See Also

- [Architecture](../concepts/architecture.md) - How TestApp generates resources
- [DSL Reference](../reference/dsl-spec.md) - Complete mesh configuration options
- [Observability](../concepts/observability.md) - Metrics and tracing with Istio
- [Istio Documentation](https://istio.io/latest/docs/) - Official Istio docs

