# Path-Based Upstream Routing

## Overview

HTTP services now support path-based routing to selectively call upstream services based on the incoming request URL path. This enables realistic simulation of frontend services routing to different backends based on the request path.

## Features

- **Prefix Matching**: `/orders` matches both `/orders` and `/orders/123/items`
- **Path Stripping**: Matched prefix is stripped before forwarding to upstream
- **Multiple Matches**: All upstreams matching the path are called
- **404 on No Match**: Returns HTTP 404 if no upstream matches the path
- **Backward Compatible**: Services without path configuration call all upstreams

## DSL Configuration

### New Format (with path-based routing)

```yaml
services:
  - name: web
    namespace: frontend
    upstreams:
      - name: order-api
        paths: [/orders, /cart]
      - name: product-api
        paths: [/products, /catalog]
      - name: catch-all-api  # No paths = matches all paths
```

### Old Format (backward compatible)

```yaml
services:
  - name: web
    upstreams: [order-api, product-api]  # No path filtering, calls all upstreams
```

## Behavior

### Path Matching Example

**Configuration:**
```yaml
upstreams:
  - name: order-api
    paths: [/orders, /cart]
  - name: product-api
    paths: [/products]
```

**Request Behavior:**
- `GET /orders` → Calls `order-api` with path `/`
- `GET /orders/123` → Calls `order-api` with path `/123`
- `GET /cart/items` → Calls `order-api` with path `/items`
- `GET /products/search` → Calls `product-api` with path `/search`
- `GET /unknown` → Returns HTTP 404 (no upstream matches)

### Path Stripping

The matched prefix is stripped from the path before forwarding:
- Request: `/orders/123/items`
- Matched prefix: `/orders`
- Forwarded path: `/123/items`

## Environment Variable Format

The UPSTREAMS environment variable format has been extended:

### New Format (with paths)
```
order-api:grpc://order-api.orders.svc.cluster.local:9090:/orders,/cart|product-api:http://product-api.products.svc.cluster.local:8080:/products
```

Format: `name:url:path1,path2|name2:url2:path3`
- Use `|` as delimiter between upstreams
- Use `,` to separate multiple paths for the same upstream
- Omit paths section for catch-all behavior

### Old Format (backward compatible)
```
api:http://api.simple-web.svc.cluster.local:8080,cache:http://cache.simple-web.svc.cluster.local:8080
```

Format: `name:url,name2:url2`
- Use `,` as delimiter between upstreams
- No paths = catch-all behavior

## Testing

### Test New Format

```bash
# Generate manifests with path-based routing
./testgen generate examples/ecommerce/app.yaml -o output/ecommerce

# Check the generated UPSTREAMS env var
grep -A 2 "UPSTREAMS" output/ecommerce/ecommerce/10-services/web-deployment.yaml
```

Expected output:
```yaml
- name: UPSTREAMS
  value: "order-api:grpc://order-api.orders.svc.cluster.local:9090:/orders,/cart|product-api:http://product-api.products.svc.cluster.local:8080:/products,/catalog"
```

### Test Backward Compatibility

```bash
# Generate manifests with old format
./testgen generate examples/simple-web/app.yaml -o output/simple-web

# Check the generated UPSTREAMS env var
grep -A 2 "UPSTREAMS" output/simple-web/simple-web/10-services/frontend-deployment.yaml
```

Expected output:
```yaml
- name: UPSTREAMS
  value: "api:http://api.simple-web.svc.cluster.local:8080"
```

## Runtime Testing

Once deployed, you can test path-based routing:

```bash
# Assuming web service is exposed via gateway

# Should call order-api
curl http://shop.local/orders

# Should call product-api
curl http://shop.local/products

# Should return 404 (no matching upstream)
curl http://shop.local/unknown
```

## Implementation Details

### Files Modified

1. **pkg/dsl/types/types.go**: Added `UpstreamRoute` struct and custom YAML unmarshaling
2. **pkg/service/config.go**: Extended `UpstreamConfig` with `Paths` field
3. **pkg/service/http/server.go**: Added path matching and routing logic
4. **pkg/generator/k8s/generator.go**: Updated UPSTREAMS env var generation
5. **pkg/dsl/parser/parser.go**: Updated validation for new upstream format
6. **examples/ecommerce/app.yaml**: Demonstrates path-based routing

### Key Functions

- `matchUpstreamsForPath()`: Returns upstreams matching the request path
- `stripMatchedPrefix()`: Strips the matched prefix from the path
- `callMatchedUpstreams()`: Calls matched upstreams with stripped paths

## Use Cases

1. **API Gateway Simulation**: Frontend service routes to different backends based on path
2. **Microservices Testing**: Test complex routing scenarios in Kubernetes
3. **Gateway API Testing**: Simulate HTTPRoute path matching behavior
4. **Load Testing**: Generate realistic traffic patterns to different services

## Migration Guide

### Existing Services

No changes required - old format continues to work. Services without path configuration will call all upstreams for any request path.

### New Services

Use the new format to enable path-based routing:

```yaml
services:
  - name: frontend
    upstreams:
      - name: user-api
        paths: [/users, /auth]
      - name: product-api
        paths: [/products, /catalog, /inventory]
      - name: order-api
        paths: [/orders, /checkout]
```

### Mixed Configuration

You can mix both formats:

```yaml
services:
  - name: frontend
    upstreams:
      - name: order-api
        paths: [/orders]
      - name: logger-api  # No paths = called for all requests
```

