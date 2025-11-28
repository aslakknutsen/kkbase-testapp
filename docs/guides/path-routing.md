# Path-Based Routing

HTTP services support path-based routing to selectively call upstream services based on the incoming request URL path, and to specify explicit forward paths when calling upstreams.

## Overview

Path-based routing uses several concepts:

- **`name`**: Unique ID for the upstream entry (used for behavior targeting)
- **`service`**: Target service name (defaults to `name` if not specified)
- **`match`**: Incoming paths that trigger routing to this upstream (HTTP callers only)
- **`path`**: Explicit forward path to call on the upstream (HTTP upstreams only)
- **`group`**: Weighted selection group - upstreams in same group are mutually exclusive

**Features:**
- Prefix matching for `match` - `/orders` matches `/orders`, `/orders/123`, `/orders/123/items`
- Explicit forward paths - call upstreams on specific paths regardless of incoming request
- Multiple matches - all matching upstreams are called (fan-out)
- Weighted groups - upstreams in same group: one selected based on weights
- 404 on no match - returns HTTP 404 if no upstream matches (when match rules exist)
- Protocol-aware - validation warnings for incompatible configs

## DSL Configuration

### Basic Upstream (No Path Routing)

```yaml
services:
  - name: web
    upstreams: [api, cache]  # Calls all upstreams on "/" for any request
```

### Match-Based Routing

Route to different upstreams based on incoming request path:

```yaml
services:
  - name: api-gateway
    namespace: frontend
    protocols: [http]
    upstreams:
      - name: order-api
        match: [/orders, /cart]
      - name: product-api
        match: [/products, /catalog]
      - name: user-api
        match: [/users, /auth]
      - name: health-check  # No match = called for all requests
```

### Explicit Forward Paths

Specify exact paths to call on upstreams:

```yaml
services:
  - name: payment
    protocols: [grpc]
    upstreams:
      - name: message-bus
        path: /events/PaymentProcessed
      - name: message-bus
        path: /events/PaymentFailed
```

### Combined Match and Path

Match incoming requests and forward to explicit paths:

```yaml
services:
  - name: api-gateway
    protocols: [http]
    upstreams:
      - name: order-api
        match: [/api/v1/orders]
        path: /v2/orders  # Forward to v2 endpoint
```

## Semantics

| `match` | `path` | Behavior |
|---------|--------|----------|
| set | omitted | Call when incoming matches, forward to "/" |
| omitted | set | Always call, forward to `path` |
| set | set | Call when incoming matches, forward to `path` |
| omitted | omitted | Always call, forward to "/" |

## Match Behavior

### Prefix Matching

Match uses prefix matching:

```yaml
upstreams:
  - name: order-api
    match: [/orders]
```

| Request Path | Matches? |
|--------------|----------|
| `/orders` | Yes |
| `/orders/123` | Yes |
| `/orders/123/items` | Yes |
| `/order` | No |
| `/products` | No |

### Multiple Paths for One Upstream

```yaml
upstreams:
  - name: order-api
    match: [/orders, /cart, /checkout]
```

All these paths route to `order-api`.

### Multiple Upstreams with Different Matches

```yaml
upstreams:
  - name: order-api
    match: [/orders, /cart]
  - name: product-api
    match: [/products]
  - name: user-api
    match: [/users]
```

| Request Path | Calls Upstream |
|--------------|----------------|
| `/orders/123` | `order-api` |
| `/cart/add` | `order-api` |
| `/products/search` | `product-api` |
| `/users/profile` | `user-api` |
| `/unknown` | (none - returns 404) |

### Catch-All Upstreams

Upstreams without `match` are called for all requests:

```yaml
upstreams:
  - name: order-api
    match: [/orders]
  - name: logger-api  # No match = called for all requests
```

Result:
- `/orders` → calls `order-api` AND `logger-api`
- `/products` → calls only `logger-api`

## Forward Path Behavior

The `path` field specifies the exact path to call on the upstream:

```yaml
upstreams:
  - name: message-bus
    path: /events/OrderCreated
```

When this upstream is called, the request goes to `message-bus` at `/events/OrderCreated`, regardless of what path the caller received.

### Default Forward Path

If `path` is not specified, upstreams are called at `/`.

## Weighted Groups

Upstreams can be grouped for weighted selection. This is useful for simulating mutually exclusive outcomes like payment success/failure.

### Basic Group Configuration

```yaml
upstreams:
  - name: payment-ok           # Unique ID for this entry
    service: message-bus       # Target service
    path: /events/PaymentProcessed
    group: payment-outcome     # Group name
  - name: payment-fail
    service: message-bus
    path: /events/PaymentFailed
    group: payment-outcome
```

Without explicit weights, upstreams in a group have equal probability (50% each in this case).

### Setting Weights via Behavior

Static weights in DSL:

```yaml
behavior:
  upstreamWeights:
    payment-ok: 85
    payment-fail: 15
```

Dynamic weights at runtime:

```bash
curl "http://service:8080/?behavior=upstreamWeights=payment-ok:85;payment-fail:15"
```

### Weight Distribution

- Weights are relative, not percentages
- Unspecified upstreams in a group share remaining weight equally
- Example: `payment-ok: 85` → `payment-fail` gets 15% automatically

### Mixed Grouped and Ungrouped

```yaml
upstreams:
  - name: logger            # No group = always called
  - name: payment-ok
    service: message-bus
    group: payment-outcome
  - name: payment-fail
    service: message-bus
    group: payment-outcome
```

Result per request:
- `logger` is always called
- One of `payment-ok` or `payment-fail` is selected based on weights

## Independent Probability

For events that should be called independently with a certain probability (not mutually exclusive), use `probability`:

```yaml
upstreams:
  - name: audit-log
    service: audit
    probability: 0.10         # 10% of requests
  - name: stock-alert
    service: message-bus
    path: /events/StockLevelLow
    probability: 0.01         # 1% of requests
```

### Probability vs Groups

| Feature | Groups + Weights | Probability |
|---------|-----------------|-------------|
| Semantics | Mutually exclusive (pick one) | Independent (might not be called) |
| Use case | Payment success/failure | Audit sampling, alerts |
| Per request | Exactly one from group | Zero or one |

### Combined Example

```yaml
upstreams:
  # Always called
  - name: notification
    service: notification-svc
    
  # Mutually exclusive group
  - name: stock-ok
    service: message-bus
    group: stock-outcome
  - name: stock-fail
    service: message-bus
    group: stock-outcome
    
  # Independent probability
  - name: stock-alert
    service: message-bus
    probability: 0.01

behavior:
  upstreamWeights:
    stock-ok: 90
    stock-fail: 10
```

Result per request:
- `notification`: always called
- One of `stock-ok`/`stock-fail`: based on 90:10 weights
- `stock-alert`: called 1% of requests (independent roll)

## Protocol Considerations

### HTTP Callers

- `match`: Used to filter which incoming paths trigger the upstream
- `path`: Used to specify forward path

### gRPC Callers

- `match`: **Ignored** (gRPC doesn't have incoming HTTP paths)
- `path`: Used to specify forward path to HTTP upstreams

### gRPC Upstreams

- `path`: **Ignored** (gRPC uses service/method, not paths)

Validation warnings are emitted at generation time for incompatible configurations.

## Environment Variable Format

### Format

```
id=url[:match=/a,/b][:path=/forward][:group=name][:prob=0.5]
```

- `id`: Unique identifier for this upstream entry (used for behavior targeting)
- `url`: Target service URL
- `match`: Incoming paths that trigger this upstream
- `path`: Forward path to call on upstream
- `group`: Weighted selection group name
- `prob`: Independent call probability (0.0-1.0)

Multiple upstreams are separated by `|`.

### Examples

```bash
# Simple upstream (no routing)
UPSTREAMS="api=http://api:8080"

# With match
UPSTREAMS="order-api=http://order-api:8080:match=/orders,/cart"

# With path
UPSTREAMS="message-bus=http://message-bus:8080:path=/events/OrderCreated"

# With both
UPSTREAMS="api=http://api:8080:match=/api/v1:path=/v2"

# With group for weighted selection
UPSTREAMS="payment-ok=http://bus:8080:path=/events/PaymentOK:group=outcome|payment-fail=http://bus:8080:path=/events/PaymentFail:group=outcome"

# Multiple upstreams
UPSTREAMS="order-api=http://order-api:8080:match=/orders|product-api=http://product-api:8080:match=/products"
```

## Use Cases

### API Gateway Simulation

Route incoming requests to different backends:

```yaml
services:
  - name: api-gateway
    protocols: [http]
    upstreams:
      - name: order-service
        match: [/api/v1/orders, /api/v1/checkout]
      - name: product-service
        match: [/api/v1/products, /api/v1/catalog]
      - name: user-service
        match: [/api/v1/users, /api/v1/auth]
```

### Event Publishing

gRPC services publishing events to an HTTP message bus:

```yaml
services:
  - name: payment
    protocols: [grpc]
    upstreams:
      - name: message-bus
        path: /events/PaymentProcessed
      - name: message-bus
        path: /events/PaymentFailed
      - name: message-bus
        path: /events/PaymentRefunded
```

### Weighted Event Selection

When publishing mutually exclusive events (e.g., payment outcomes), use groups with weights:

```yaml
services:
  - name: payment
    protocols: [grpc]
    upstreams:
      - name: payment-processed      # Unique ID for behavior targeting
        service: message-bus         # Target service
        path: /events/PaymentProcessed
        group: payment-outcome       # Group for weighted selection
      - name: payment-failed
        service: message-bus
        path: /events/PaymentFailed
        group: payment-outcome
      - name: payment-refunded
        service: message-bus
        path: /events/PaymentRefunded
        group: payment-outcome
    behavior:
      upstreamWeights:
        payment-processed: 85
        payment-failed: 5
        payment-refunded: 10
```

Upstreams in the same group are mutually exclusive - only one is called per request, selected based on weights.

### Event Routing

Message bus routing events to notification service:

```yaml
services:
  - name: message-bus
    protocols: [http]
    upstreams:
      - name: notification
        match: [/events/OrderCreated, /events/OrderUpdated]
      - name: notification
        match: [/events/PaymentProcessed, /events/PaymentFailed]
```

### Version Migration

Route old API paths to new endpoints:

```yaml
services:
  - name: api-proxy
    protocols: [http]
    upstreams:
      - name: api-v2
        match: [/api/v1/users]
        path: /v2/users
      - name: api-v2
        match: [/api/v1/orders]
        path: /v2/orders
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

### Test Path Routing at Runtime

```bash
# Should call order-api
curl http://api-gateway:8080/orders

# Should call product-api
curl http://api-gateway:8080/products

# Should return 404 (no matching upstream)
curl http://api-gateway:8080/unknown
```

## See Also

- [DSL Reference](../reference/dsl-spec.md) - Upstream configuration syntax
- [Architecture](../concepts/architecture.md) - HTTP server implementation
- [E-Commerce Example](../../examples/ecommerce-full/) - Path routing in action
