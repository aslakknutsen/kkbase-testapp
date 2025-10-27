# ShopFlow E-Commerce Platform

A comprehensive e-commerce microservices example demonstrating event-driven architecture, saga patterns, and real-world service interactions using the TestApp DSL.

## Architecture

ShopFlow is a modern e-commerce platform built on 12 microservices organized across 7 namespaces. The architecture uses an **API Gateway** as the single entry point and a **Message Bus** to simulate asynchronous event-driven communication between services.

### Namespace Organization

```
sf-gateway      → API Gateway (entry point)
sf-products     → Product Catalog, Search, Inventory, Reviews
sf-users        → User Management
sf-shopping     → Shopping Cart, Checkout
sf-orders       → Order Management, Shipping
sf-payments     → Payment Processing
sf-infra        → Message Bus, Notifications
```

### Service Interaction Diagram

```
                    ┌─────────────────────────────────────┐
                    │  Client (Browser/Mobile App)        │
                    └──────────────┬──────────────────────┘
                                   │
                    ┌──────────────▼──────────────────────┐
                    │    api-gateway (sf-gateway)         │
                    │    Routes: /api/v1/*                │
                    └──────────────┬──────────────────────┘
                                   │
        ┌──────────────────────────┼──────────────────────────┐
        │                          │                          │
┌───────▼──────────┐   ┌──────────▼──────────┐   ┌─────────▼─────────┐
│ product-catalog  │   │ shopping-cart       │   │ user-management   │
│ (sf-products)    │   │ (sf-shopping)       │   │ (sf-users)        │
└───────┬──────────┘   └──────────┬──────────┘   └───────────────────┘
        │                          │
    ┌───┴────┐                     │
    │        │                     │
┌───▼────┐ ┌─▼────────┐      ┌────▼─────┐
│inventory│ │ search   │      │ checkout │
└───┬────┘ └──────────┘      └────┬─────┘
    │                              │
    │                         ┌────▼────────────┐
    │                         │ order-management│
    │                         │ (sf-orders)     │
    │                         └────┬────────────┘
    │                              │
    │                     ┌────────▼────────────────────────┐
    │                     │   message-bus (sf-infra)        │
    │                     │   Event Router & Orchestrator   │
    │                     └────────┬────────────────────────┘
    │                              │
    │         ┌────────────────────┼────────────────────┐
    │         │                    │                    │
┌───▼─────┐ ┌▼────────┐    ┌─────▼──────┐    ┌───────▼──────┐
│ payment │ │ shipping│    │notification│    │ (back to     │
│(sf-pays)│ │(sf-orders)   │ (sf-infra) │    │  services)   │
└─────────┘ └─────────┘    └────────────┘    └──────────────┘
```

## Service Descriptions

### Gateway Layer

**api-gateway** (`sf-gateway`)
- **Purpose**: Single entry point for all client requests
- **Endpoints**: Routes all `/api/v1/*` paths to appropriate backend services
- **Path Routing**:
  - `/api/v1/products`, `/api/v1/catalog` → product-catalog
  - `/api/v1/search` → search
  - `/api/v1/cart` → shopping-cart
  - `/api/v1/checkout` → checkout
  - `/api/v1/orders` → order-management
  - `/api/v1/account`, `/api/v1/users` → user-management
  - `/api/v1/reviews` → reviews
- **Protocol**: HTTP
- **Ingress**: `https://shopflow.local/api/v1`

### Product & Discovery Services (`sf-products`)

**product-catalog**
- **Purpose**: Source of truth for product information (descriptions, pricing, images)
- **Upstreams**: inventory, message-bus
- **Events Published**: ProductUpdated, PriceChanged
- **Protocols**: HTTP, gRPC
- **Latency**: 15-50ms

**search**
- **Purpose**: Powerful search and filtering (by brand, price, category, etc.)
- **Upstreams**: message-bus
- **Events Consumed**: ProductUpdated, PriceChanged
- **Protocol**: HTTP
- **Latency**: 20-80ms

**inventory**
- **Purpose**: Manages stock levels across warehouses
- **Upstreams**: message-bus
- **Events Published**: StockReserved, StockReservationFailed, StockLevelLow
- **Events Consumed**: PaymentProcessed
- **Protocol**: gRPC
- **Latency**: 10-40ms

**reviews**
- **Purpose**: Customer reviews and ratings for products
- **Upstreams**: message-bus
- **Events Consumed**: OrderDelivered (triggers review requests)
- **Protocol**: HTTP
- **Latency**: 15-60ms

### User Services (`sf-users`)

**user-management**
- **Purpose**: Authentication, user profiles, saved addresses, payment methods
- **Upstreams**: None (standalone)
- **Protocol**: HTTP, gRPC
- **Latency**: 10-35ms
- **Security**: High (handles PII)
- **Labels**: `pii: enabled`

### Transaction Services (`sf-shopping`)

**shopping-cart**
- **Purpose**: Manages user shopping cart state
- **Upstreams**: inventory (for availability checks)
- **Protocol**: HTTP
- **Latency**: 12-45ms

**checkout**
- **Purpose**: Orchestrates the checkout flow
- **Upstreams**: order-management (creates order and starts saga)
- **Protocol**: HTTP, gRPC
- **Latency**: 25-100ms
- **Critical**: True

### Order & Fulfillment Services (`sf-orders`)

**order-management**
- **Purpose**: Creates and tracks order lifecycle, orchestrates the order saga
- **Upstreams**: payment, inventory, shipping (direct calls for saga orchestration)
- **Events Published**: OrderCreated, OrderUpdated, OrderCancelled (via payment/inventory/shipping to message-bus)
- **Protocol**: gRPC
- **Latency**: 20-70ms
- **Critical**: True

**shipping**
- **Purpose**: Manages fulfillment and shipping
- **Upstreams**: message-bus
- **Events Published**: OrderShipped, OrderDelivered
- **Events Consumed**: StockReserved
- **Protocol**: HTTP
- **Latency**: 30-120ms

### Payment Services (`sf-payments`)

**payment**
- **Purpose**: Processes payments through external gateways
- **Upstreams**: message-bus
- **Events Published**: PaymentProcessed, PaymentFailed, PaymentRefunded
- **Events Consumed**: OrderCreated, StockReservationFailed
- **Protocol**: gRPC
- **Latency**: 50-200ms (includes external gateway calls)
- **Security**: High, PCI-compliant
- **Critical**: True

### Infrastructure Services (`sf-infra`)

**message-bus**
- **Purpose**: Event router for fan-out to notification service
- **Architecture**: Acts as a pub/sub broker - services publish events which are fanned out to notification
- **Path-Based Event Routing** (all routes to notification):
  - `/events/OrderCreated`, `/events/OrderUpdated`, `/events/OrderCancelled`
  - `/events/PaymentProcessed`, `/events/PaymentFailed`, `/events/PaymentRefunded`
  - `/events/StockReserved`, `/events/StockReservationFailed`, `/events/StockLevelLow`
  - `/events/OrderShipped`, `/events/OrderDelivered`
  - `/events/ProductUpdated`, `/events/PriceChanged`
- **Protocol**: HTTP
- **Latency**: 2-10ms
- **Critical**: True
- **Note**: Direct service-to-service calls handle the saga orchestration; message-bus handles cross-cutting notification concerns

**notification**
- **Purpose**: Sends emails, SMS, push notifications
- **Upstreams**: None (leaf service)
- **Events Consumed**: All order/payment events
- **Protocol**: HTTP
- **Latency**: 80-250ms (includes external service calls)

## Getting Started

### Generate Kubernetes Manifests

```bash
# From repository root
make build
./testgen examples/ecommerce-full/app.yaml

# Output will be in output/shopflow-ecommerce/
ls -la output/shopflow-ecommerce/
```

### Deploy to Kubernetes

```bash
# Deploy namespaces
kubectl apply -f output/shopflow-ecommerce/00-namespaces.yaml

# Deploy services
kubectl apply -f output/shopflow-ecommerce/10-services/

# Deploy gateway and routing
kubectl apply -f output/shopflow-ecommerce/20-gateway/
```

### Access the Application

```bash
# Add to /etc/hosts
echo "127.0.0.1 shopflow.local" | sudo tee -a /etc/hosts

# Port forward the gateway
kubectl port-forward -n sf-gateway svc/api-gateway 8080:8080

# Test the API
curl https://shopflow.local/api/v1/products
# OR via port-forward
curl http://localhost:8080/api/v1/products
```

## Example API Calls

### Browse Products

```bash
# Get all products
curl "http://localhost:8080/api/v1/products"

# Get product details (path continues to product-catalog)
curl "http://localhost:8080/api/v1/products/laptop-123"
```

### Search

```bash
# Search for laptops
curl "http://localhost:8080/api/v1/search?q=laptop"

# Filter by price range
curl "http://localhost:8080/api/v1/search?q=laptop&price_max=1000"
```

### Shopping Cart

```bash
# Add item to cart
curl -X POST "http://localhost:8080/api/v1/cart/add" \
  -H "Content-Type: application/json" \
  -d '{"product_id": "laptop-123", "quantity": 1}'

# View cart
curl "http://localhost:8080/api/v1/cart"

# Remove item
curl -X DELETE "http://localhost:8080/api/v1/cart/items/laptop-123"
```

### Checkout & Orders

```bash
# Initiate checkout (starts the saga!)
curl -X POST "http://localhost:8080/api/v1/checkout" \
  -H "Content-Type: application/json" \
  -d '{
    "cart_id": "cart-123",
    "shipping_address": "123 Main St",
    "payment_method": "card-456"
  }'

# Track order status
curl "http://localhost:8080/api/v1/orders/order-789"

# Get order history
curl "http://localhost:8080/api/v1/orders?user_id=user-123"
```

### User Account

```bash
# Get user profile
curl "http://localhost:8080/api/v1/account/profile"

# Update address
curl -X PUT "http://localhost:8080/api/v1/account/addresses/addr-1" \
  -H "Content-Type: application/json" \
  -d '{"street": "456 Oak Ave", "city": "San Francisco"}'
```

### Product Reviews

```bash
# Get product reviews
curl "http://localhost:8080/api/v1/reviews?product_id=laptop-123"

# Submit review
curl -X POST "http://localhost:8080/api/v1/reviews" \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "laptop-123",
    "rating": 5,
    "comment": "Great laptop!"
  }'
```

## Event-Driven Saga Pattern

### The Order Placement Saga

This is the heart of ShopFlow - a distributed transaction spanning multiple services that must either all succeed or rollback gracefully.

#### Normal Flow (Happy Path)

```
1. User clicks "Place Order"
   ↓
2. checkout service calls order-management (synchronous)
   ↓
3. order-management creates order with status="Pending"
   Returns order ID to customer immediately
   ↓
4. order-management calls payment service (orchestration)
   Publishes OrderCreated event via payment → message-bus → notification
   ↓
5. payment processes payment with external gateway
   ↓ (SUCCESS)
6. payment returns success to order-management
   Publishes PaymentProcessed event → message-bus → notification
   ↓
7. order-management calls inventory service
   ↓
8. inventory reserves stock
   ↓ (SUCCESS)
9. inventory returns success to order-management
   Publishes StockReserved event → message-bus → notification
   ↓
10. order-management calls shipping service
    ↓
11. shipping prepares shipment
    ↓ (SHIPPED)
12. shipping returns success, publishes OrderShipped event
    → message-bus → notification
    ↓
13. order-management updates order status to "Shipped"
    ↓
14. Customer receives notifications at each step via notification service
```

#### Failure Flow: Payment Fails

```
1-4. [Same as above]
   ↓
5. payment processes payment with external gateway
   ↓ (FAILURE - card declined)
6. payment returns error to order-management
   Publishes PaymentFailed event → message-bus → notification
   ↓
7. order-management updates order status to "Failed - Payment Error"
   ↓
8. Saga stops. Customer notified via notification service to update payment method.
```

#### Failure Flow: Stock Reservation Fails (Compensating Transaction)

```
1-7. [Payment succeeds, order-management proceeds to inventory]
   ↓
8. order-management calls inventory service
   ↓
9. inventory attempts to reserve stock
   ↓ (FAILURE - out of stock)
10. inventory returns error to order-management
    Publishes StockReservationFailed event → message-bus → notification
    ↓
11. order-management initiates COMPENSATING TRANSACTION
    Calls payment service to process refund
    ↓
12. payment processes refund
    Publishes PaymentRefunded event → message-bus → notification
    ↓
13. order-management updates order status to "Failed - Out of Stock"
    ↓
14. Customer receives notification: "Order cancelled, refund processed, item out of stock"
```

### Key Saga Concepts

1. **Orchestration Pattern**: order-management acts as the saga orchestrator, directly calling payment → inventory → shipping in sequence
2. **Event Publishing**: Each service publishes domain events (success/failure) to message-bus for cross-cutting concerns (notifications)
3. **Compensating Transactions**: When a step fails after earlier steps succeeded, the orchestrator executes compensating actions (e.g., order-management calls payment to refund if inventory unavailable)
4. **Eventual Consistency**: Order status updates as the saga progresses through its steps
5. **Separation of Concerns**: Direct calls handle transactional flow; message-bus handles notifications and auditing
6. **Testability**: The behavior system lets you inject failures at any step to test error handling and rollback logic

## Behavior Testing Scenarios

All behavior tests are performed by adding the `?behavior=` query parameter to your requests. For detailed behavior syntax, see the [Behavior Testing Guide](../../docs/guides/behavior-testing.md).

### Scenario 1: Normal Checkout Flow (Baseline)

**Purpose**: Establish baseline performance and verify the complete saga works end-to-end.

**Command**:
```bash
curl -X POST "http://localhost:8080/api/v1/checkout" \
  -H "Content-Type: application/json" \
  -d '{"cart_id": "cart-123"}' | jq
```

**Expected Outcome**:
- Order created with status "Pending"
- All events flow through successfully
- Response time: ~200-400ms (sum of all service latencies)
- Final order status: "Shipped"

**How to Observe**:
```bash
# Check the full call chain
curl "http://localhost:8080/api/v1/checkout?behavior=" | jq '.upstream_calls[] | {name, code, duration}'

# Verify all services were called
curl "http://localhost:8080/api/v1/checkout" | jq 'def services: .service.name, (.upstream_calls[]? | services); [services] | unique'
```

### Scenario 2: Payment Gateway Failure

**Purpose**: Test the saga handles payment failures gracefully and stops the flow without proceeding to inventory reservation.

**Behavior String**: `payment:error=503:0.5`

**Command**:
```bash
curl -X POST "http://localhost:8080/api/v1/checkout?behavior=payment:error=503:0.5" \
  -H "Content-Type: application/json" \
  -d '{"cart_id": "cart-123"}' | jq
```

**Expected Outcome**:
- 50% of requests: payment fails with 503
- Saga stops at payment step
- No inventory reservation attempted
- Order status: "Failed - Payment Error"
- Notification sent to customer

**How to Observe**:
```bash
# Check payment service status
curl "http://localhost:8080/api/v1/checkout?behavior=payment:error=503:0.5" | \
  jq '.upstream_calls[] | select(.name=="payment") | {code, duration}'

# Verify inventory was NOT called (call chain should stop at payment)
curl "http://localhost:8080/api/v1/checkout?behavior=payment:error=503:0.5" | \
  jq '.upstream_calls[] | select(.name=="inventory")'
# Should return nothing on failed payment
```

### Scenario 3: Inventory Service Slow (Timeout Testing)

**Purpose**: Test timeout handling when inventory service is slow. Verify system doesn't hang indefinitely.

**Behavior String**: `inventory:latency=500-1000ms`

**Command**:
```bash
curl -X POST "http://localhost:8080/api/v1/checkout?behavior=inventory:latency=500-1000ms" \
  -H "Content-Type: application/json" \
  -d '{"cart_id": "cart-123"}' | jq
```

**Expected Outcome**:
- Payment succeeds quickly
- Long wait at inventory step (500-1000ms)
- Request may timeout if client timeout < 1000ms
- Monitors/traces show slow inventory service

**How to Observe**:
```bash
# Check timing breakdown
curl "http://localhost:8080/api/v1/checkout?behavior=inventory:latency=500-1000ms" | \
  jq '.upstream_calls[] | {name, duration}'

# Look for inventory taking 500-1000ms
```

### Scenario 4: Cascading Failures

**Purpose**: Test system resilience when multiple services are degraded simultaneously.

**Behavior String**: `payment:error=0.2,latency=100-300ms,inventory:error=0.15,shipping:latency=200-500ms`

**Command**:
```bash
curl -X POST "http://localhost:8080/api/v1/checkout?behavior=payment:error=0.2,latency=100-300ms,inventory:error=0.15,shipping:latency=200-500ms" \
  -H "Content-Type: application/json" \
  -d '{"cart_id": "cart-123"}' | jq
```

**Expected Outcome**:
- ~20% payment failures
- ~15% inventory failures (triggers refunds)
- Increased overall latency
- Some requests succeed but take longer
- Circuit breakers may trigger if error rates sustained

**How to Observe**:
```bash
# Run multiple times and observe error distribution
for i in {1..20}; do
  curl -s "http://localhost:8080/api/v1/checkout?behavior=payment:error=0.2,inventory:error=0.15" | \
    jq -c '{payment: (.upstream_calls[] | select(.name=="payment").code), inventory: (.upstream_calls[] | select(.name=="inventory").code)}'
done
```

### Scenario 5: High Load Simulation

**Purpose**: Simulate database load by adding latency to all critical paths.

**Behavior String**: `order-management:latency=100-200ms,payment:latency=150-300ms,inventory:latency=80-150ms`

**Command**:
```bash
curl -X POST "http://localhost:8080/api/v1/checkout?behavior=order-management:latency=100-200ms,payment:latency=150-300ms,inventory:latency=80-150ms" \
  -H "Content-Type: application/json" \
  -d '{"cart_id": "cart-123"}' | jq
```

**Expected Outcome**:
- All services slower but still functional
- Total latency: 330-650ms (accumulated)
- No errors, just slower responses
- Good test for client timeout configuration

**How to Observe**:
```bash
# Measure total duration
curl "http://localhost:8080/api/v1/checkout?behavior=order-management:latency=100-200ms,payment:latency=150-300ms,inventory:latency=80-150ms" | \
  jq '{total_duration: .duration, breakdown: [.upstream_calls[] | {name, duration}]}'
```

### Scenario 6: Event Bus Failure

**Purpose**: Test resilience when the event bus (message broker) is degraded or failing.

**Behavior String**: `message-bus:error=0.3,latency=50-200ms`

**Command**:
```bash
curl -X POST "http://localhost:8080/api/v1/checkout?behavior=message-bus:error=0.3,latency=50-200ms" \
  -H "Content-Type: application/json" \
  -d '{"cart_id": "cart-123"}' | jq
```

**Expected Outcome**:
- ~30% of event deliveries fail
- Events may not reach downstream services
- Orders stuck in intermediate states
- Demonstrates need for retry logic and dead letter queues

**How to Observe**:
```bash
# Check message-bus health
curl "http://localhost:8080/api/v1/checkout?behavior=message-bus:error=0.3" | \
  jq '.upstream_calls[] | select(.name=="message-bus")'

# Run multiple times to see failure rate
for i in {1..10}; do
  curl -s "http://localhost:8080/api/v1/checkout?behavior=message-bus:error=0.3" | \
    jq -c '.upstream_calls[] | select(.name=="message-bus") | .code'
done | sort | uniq -c
```

### Scenario 7: Product Search Degradation

**Purpose**: Test frontend resilience when search service is slow or failing.

**Behavior String**: `search:latency=1s-3s,error=0.1`

**Command**:
```bash
curl "http://localhost:8080/api/v1/search?q=laptop&behavior=search:latency=1s-3s,error=0.1" | jq
```

**Expected Outcome**:
- Search is very slow (1-3 seconds)
- 10% of searches fail
- Other site functionality (cart, checkout) unaffected
- Demonstrates service isolation

### Scenario 8: Complete System Chaos

**Purpose**: Stress test with realistic chaos across all services.

**Behavior String**: `latency=20-100ms,error=0.05,payment:error=0.1,inventory:latency=100-500ms,message-bus:error=0.15`

**Command**:
```bash
curl -X POST "http://localhost:8080/api/v1/checkout?behavior=latency=20-100ms,error=0.05,payment:error=0.1,inventory:latency=100-500ms,message-bus:error=0.15" \
  -H "Content-Type: application/json" \
  -d '{"cart_id": "cart-123"}' | jq
```

**Expected Outcome**:
- System stressed but functional
- Mix of successes, failures, and slow responses
- Good test for monitoring/alerting thresholds
- Demonstrates real-world degraded performance

## Advanced Testing

### Load Testing with hey

```bash
# Install hey
go install github.com/rakyll/hey@latest

# Baseline load test
hey -n 1000 -c 10 http://localhost:8080/api/v1/products

# Load test with payment failures
hey -n 1000 -c 10 "http://localhost:8080/api/v1/checkout?behavior=payment:error=0.2"

# Sustained load with latency
hey -n 5000 -c 50 -q 100 "http://localhost:8080/api/v1/checkout?behavior=latency=50-150ms"
```

### Observing Event Flows

```bash
# Watch logs for event routing
kubectl logs -n sf-infra -l app=message-bus -f | grep "events/"

# Monitor notification service for event deliveries
kubectl logs -n sf-infra -l app=notification -f

# Check order status updates
kubectl logs -n sf-orders -l app=order-management -f | grep "status"
```

### Trace Analysis with Jaeger

```bash
# Get trace ID from response
TRACE_ID=$(curl -s http://localhost:8080/api/v1/checkout | jq -r '.trace_id')

# View in Jaeger UI
echo "Open: http://localhost:16686/trace/$TRACE_ID"

# Or use Jaeger API
curl "http://localhost:16686/api/traces/$TRACE_ID" | jq
```

## Key Metrics to Monitor

### Request Metrics
- `testservice_requests_total{service="api-gateway"}` - Gateway throughput
- `testservice_requests_total{service="payment", code="200"}` - Successful payments
- `testservice_requests_total{service="payment", code=~"5.."}` - Payment failures

### Latency Metrics
- `histogram_quantile(0.95, testservice_request_duration_seconds_bucket{service="checkout"})` - P95 checkout latency
- `histogram_quantile(0.99, testservice_request_duration_seconds_bucket{service="payment"})` - P99 payment latency

### Event Metrics
- `testservice_upstream_calls_total{service="message-bus"}` - Event routing volume
- `testservice_upstream_calls_total{service="message-bus", code!="200"}` - Event delivery failures

### Business Metrics
- Count of OrderCreated events
- Payment success rate
- Order completion rate (OrderShipped events)
- Average time from OrderCreated to OrderShipped

## Troubleshooting

### Orders Stuck in Pending

```bash
# Check if events are flowing
kubectl logs -n sf-infra -l app=message-bus | grep "OrderCreated"

# Check payment service
kubectl logs -n sf-payments -l app=payment | tail -50

# Verify message-bus routing
curl http://localhost:8080/api/v1/orders | jq '.upstream_calls[] | select(.name=="message-bus")'
```

### Payment Failures

```bash
# Check payment service error rate
curl http://localhost:9091/metrics | grep 'testservice_requests_total{service="payment"'

# Test payment service directly
kubectl port-forward -n sf-payments svc/payment 9090:9090
grpcurl -plaintext localhost:9090 testservice.TestService/Call
```

### Slow Responses

```bash
# Identify slow services
curl http://localhost:8080/api/v1/checkout | jq '.upstream_calls[] | {name, duration}' | sort -k2 -n

# Check for cascading latency
curl http://localhost:8080/api/v1/checkout | jq '.duration'
```

## Architecture Decisions

### Why Orchestration Pattern for the Saga?

ShopFlow uses the **orchestration pattern** where order-management coordinates the saga flow by directly calling services in sequence. This differs from **choreography** where services react to events independently.

**Benefits of Orchestration**:
1. **Clear flow**: Easy to understand the order of operations
2. **Centralized control**: order-management owns the saga state
3. **Simpler compensation**: Orchestrator knows what to undo on failure
4. **Better for DSL**: Avoids circular dependencies in the service graph
5. **Debuggability**: Single service to trace for saga execution

**Trade-offs**:
- More coupling: Services are explicitly called by orchestrator
- Single point of control: order-management must be highly available

In production, you might use choreography for truly event-driven systems, or orchestration frameworks like Temporal or Camunda.

### Why Path-Based Routing on Message Bus?

The message-bus uses path-based routing (e.g., `/events/OrderCreated`) to fan-out notifications. This allows:

1. **Event visibility** via HTTP traces (you can see event flows in Jaeger)
2. **Behavior testing** of event delivery (latency, failures, message-bus crashes)
3. **Simple demonstration** of pub/sub patterns
4. **Decoupling**: Cross-cutting concerns (notifications) separated from transactional flow

In production, you'd replace this with Kafka, RabbitMQ, or cloud-native event buses.

### Why Multiple Namespaces?

Namespace separation provides:

1. **Security boundaries** - Different teams, different permissions
2. **Resource isolation** - Payment namespace separate from others
3. **Network policies** - Fine-grained traffic control
4. **Realistic architecture** - Mirrors real enterprise deployments

### Why Mix HTTP and gRPC?

Different protocols for different needs:

- **HTTP**: External-facing (API Gateway), simple CRUD (user-management, reviews)
- **gRPC**: Internal high-performance calls (payment, order-management, inventory)

## Next Steps

1. **Add Resilience Patterns**: Implement retries, circuit breakers, timeouts
2. **Add Caching**: Deploy Redis for product-catalog and search
3. **Add Databases**: Replace simulated DBs with real PostgreSQL/MySQL
4. **Add Observability**: Full Jaeger, Prometheus, Grafana stack
5. **Add Security**: mTLS between services, authentication, authorization
6. **Scale Testing**: Test with 10k+ orders/minute

## See Also

- [Behavior Testing Guide](../../docs/guides/behavior-testing.md)
- [Path-Based Routing](../../docs/guides/path-routing.md)
- [Architecture Concepts](../../docs/concepts/architecture.md)
- [Jaeger Setup Guide](../../docs/guides/jaeger-setup.md)

