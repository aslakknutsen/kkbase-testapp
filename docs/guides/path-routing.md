# Path-Based Routing

HTTP services support path-based routing to selectively call upstream services based on the incoming request URL path. This enables realistic simulation of frontend services routing to different backends.

## Overview

Path-based routing allows a service to route requests to different upstream services based on URL paths, similar to API gateways and reverse proxies.

**Features:**
- Prefix matching - `/orders` matches `/orders`, `/orders/123`, `/orders/123/items`
- Path stripping - Matched prefix removed before forwarding
- Multiple matches - All matching upstreams are called
- 404 on no match - Returns HTTP 404 if no upstream matches
- Backward compatible - Services without path configuration call all upstreams

## DSL Configuration

### With Path-Based Routing

```yaml
services:
  - name: api-gateway
    namespace: frontend
    protocols: [http]
    upstreams:
      - name: order-api
        paths: [/orders, /cart]
      - name: product-api
        paths: [/products, /catalog]
      - name: user-api
        paths: [/users, /auth]
      - name: health-check  # No paths = matches all paths
```

### Without Path-Based Routing (Backward Compatible)

```yaml
services:
  - name: web
    upstreams: [api, cache]  # Calls all upstreams for any path
```

## Path Matching

### Prefix Matching

Paths use prefix matching:

```yaml
upstreams:
  - name: order-api
    paths: [/orders]
```

| Request Path | Matches? | Forwarded Path |
|--------------|----------|----------------|
| `/orders` | Yes | `/` |
| `/orders/123` | Yes | `/123` |
| `/orders/123/items` | Yes | `/123/items` |
| `/order` | No | - |
| `/products` | No | - |

### Multiple Paths for One Upstream

```yaml
upstreams:
  - name: order-api
    paths: [/orders, /cart, /checkout]
```

All these paths route to `order-api`:
- `/orders` → `order-api`
- `/cart` → `order-api`
- `/checkout` → `order-api`

### Multiple Upstreams with Different Paths

```yaml
upstreams:
  - name: order-api
    paths: [/orders, /cart]
  - name: product-api
    paths: [/products]
  - name: user-api
    paths: [/users]
```

| Request Path | Calls Upstream | Forwarded Path |
|--------------|----------------|----------------|
| `/orders/123` | `order-api` | `/123` |
| `/cart/add` | `order-api` | `/add` |
| `/products/search` | `product-api` | `/search` |
| `/users/profile` | `user-api` | `/profile` |
| `/unknown` | (none) | Returns 404 |

### Catch-All Upstreams

Upstreams without paths match all requests:

```yaml
upstreams:
  - name: order-api
    paths: [/orders]
  - name: logger-api  # No paths = called for all requests
```

Result:
- `/orders` → calls `order-api` AND `logger-api`
- `/products` → calls only `logger-api`

## Path Stripping

The matched prefix is stripped before forwarding to the upstream:

**Configuration:**
```yaml
upstreams:
  - name: order-api
    paths: [/orders]
```

**Examples:**

| Incoming Request | Matched Prefix | Path to Upstream |
|------------------|----------------|------------------|
| `/orders` | `/orders` | `/` |
| `/orders/123` | `/orders` | `/123` |
| `/orders/123/items` | `/orders` | `/123/items` |

This allows upstream services to not know about the routing prefix.

## Environment Variable Format

### New Format (with paths)

```
order-api:grpc://order-api:9090:/orders,/cart|product-api:http://product-api:8080:/products
```

Format: `name:url:path1,path2|name2:url2:path3`
- `|` delimiter between upstreams
- `,` separates multiple paths for same upstream
- Omit paths section for catch-all

### Old Format (backward compatible)

```
api:http://api:8080,cache:http://cache:8080
```

Format: `name:url,name2:url2`
- `,` delimiter between upstreams
- No paths = catch-all behavior

## Use Cases

### API Gateway Simulation

Frontend service routes to different backends based on path:

```yaml
services:
  - name: api-gateway
    upstreams:
      - name: order-service
        paths: [/api/v1/orders, /api/v1/checkout]
      - name: product-service
        paths: [/api/v1/products, /api/v1/catalog]
      - name: user-service
        paths: [/api/v1/users, /api/v1/auth]
```

### Microservices Testing

Test complex routing scenarios:

```yaml
services:
  - name: web-frontend
    upstreams:
      - name: orders-backend
        paths: [/orders]
      - name: products-backend
        paths: [/products]
      - name: search-backend
        paths: [/search]
```

### Gateway API Simulation

Simulate HTTPRoute path matching behavior:

```yaml
services:
  - name: ingress-proxy
    upstreams:
      - name: api-v1
        paths: [/api/v1]
      - name: api-v2
        paths: [/api/v2]
      - name: static-assets
        paths: [/static, /assets]
```

## Testing

### Generate Manifests

```bash
./testgen generate examples/ecommerce/app.yaml -o output
```

### Verify UPSTREAMS Environment Variable

```bash
kubectl get deployment api-gateway -o yaml | grep -A 2 "UPSTREAMS"
```

Expected output:
```yaml
- name: UPSTREAMS
  value: "order-api:grpc://order-api:9090:/orders,/cart|product-api:http://product-api:8080:/products"
```

### Test Path Routing at Runtime

```bash
# Should call order-api
curl http://api-gateway:8080/orders

# Should call product-api
curl http://api-gateway:8080/products

# Should return 404 (no matching upstream)
curl http://api-gateway:8080/unknown
```

### Verify Routing in Response

```bash
curl http://api-gateway:8080/orders | jq '.upstream_calls[].name'
# Output: "order-api"

curl http://api-gateway:8080/products | jq '.upstream_calls[].name'
# Output: "product-api"
```

## Example: E-Commerce Gateway

```yaml
services:
  - name: api-gateway
    namespace: frontend
    protocols: [http]
    replicas: 2
    upstreams:
      - name: order-api
        url: grpc://order-api.orders.svc.cluster.local:9090
        paths: [/orders, /cart, /checkout]
      - name: product-api
        url: http://product-api.products.svc.cluster.local:8080
        paths: [/products, /catalog, /search]
      - name: user-api
        url: http://user-api.users.svc.cluster.local:8080
        paths: [/users, /auth, /profile]
      - name: review-api
        url: http://review-api.reviews.svc.cluster.local:8080
        paths: [/reviews, /ratings]
```

Requests:
- `GET /orders/123` → routes to `order-api` with path `/123`
- `GET /products/search?q=laptop` → routes to `product-api` with path `/search?q=laptop`
- `GET /users/profile` → routes to `user-api` with path `/profile`
- `GET /reviews?product=123` → routes to `review-api` with path `/?product=123`

## Migration Guide

### Existing Services

No changes required. Services without path configuration continue to call all upstreams.

### New Services with Path Routing

Use the new format:

```yaml
services:
  - name: frontend
    upstreams:
      - name: user-api
        paths: [/users, /auth]
      - name: product-api
        paths: [/products, /catalog]
```

### Mixed Configuration

You can mix path-based and catch-all upstreams:

```yaml
services:
  - name: frontend
    upstreams:
      - name: order-api
        paths: [/orders]      # Only for /orders paths
      - name: logger-api      # Called for all requests
```

## Limitations

- HTTP only (gRPC services ignore path routing)
- Prefix matching only (no regex or exact match)
- Path rewriting not supported (only stripping)
- Query parameters are preserved

## See Also

- [DSL Reference](../reference/dsl-spec.md) - Upstream configuration syntax
- [Architecture](../concepts/architecture.md) - HTTP server implementation
- [E-Commerce Example](../../examples/ecommerce/) - Path routing in action

