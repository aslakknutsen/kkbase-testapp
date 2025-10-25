# Behavior Engine Test Scenarios

This document describes the comprehensive test scenarios for the behavior chain parsing and behavior engine.

## Test Coverage

### 1. TestParseBehavior - Basic Behavior Parsing

Tests parsing of individual behavior specifications without service targeting.

#### Scenarios:

**Empty String**
- Input: `""`
- Expected: Empty behavior with no effects
- Purpose: Ensure empty strings are handled gracefully

**Fixed Latency**
- Input: `"latency=100ms"`
- Expected: Fixed latency of 100ms
- Purpose: Test simple latency specification

**Range Latency**
- Input: `"latency=50ms-200ms"` or `"latency=50-200ms"`
- Expected: Random latency between 50ms and 200ms
- Purpose: Test range-based latency with different unit formats

**Error with Probability Only**
- Input: `"error=0.5"`
- Expected: 50% chance of error with default code 500
- Purpose: Test error injection with just probability

**Error with Code Only**
- Input: `"error=503"`
- Expected: 100% error rate with code 503
- Purpose: Test guaranteed error with specific code

**Error with Probability and Code**
- Input: `"error=503:0.3"`
- Expected: 30% chance of error with code 503
- Purpose: Test complete error specification (colon syntax)

**CPU Spike**
- Input: `"cpu=spike"`
- Expected: CPU spike pattern
- Purpose: Test CPU behavior parsing

**Memory Leak**
- Input: `"memory=leak-slow"`
- Expected: Slow memory leak pattern
- Purpose: Test memory behavior parsing

**Combined Behaviors**
- Input: `"latency=100ms,error=500:0.5"`
- Expected: Both latency AND error behaviors
- Purpose: Test multiple behaviors in one specification

---

### 2. TestParseChain - Behavior Chain Parsing with Service Targeting

Tests parsing of behavior chains that can target specific services.

#### Syntax Rules:
- `service:behavior` - Targets specific service
- `behavior` - Applies globally to all services (no prefix)
- Comma-separated values continue the current service context
- New `service:` prefix starts a new service context

#### Scenarios:

**Empty Chain**
- Input: `""`
- Expected: No behaviors
- Purpose: Handle empty chains

**Single Global Behavior**
- Input: `"latency=100ms"`
- Expected: One global behavior affecting all services
- Purpose: Test simplest global case

**Single Service-Targeted Behavior**
- Input: `"order-api:error=500:0.5"`
- Expected: Error behavior only for order-api
- Purpose: Test basic service targeting

**Multiple Service-Targeted Behaviors**
- Input: `"order-api:error=500:0.5,product-api:latency=200ms"`
- Expected: Two separate behaviors, each for different service
- Purpose: Test multiple independent service targets

**Service with Chained Behaviors**
- Input: `"order-api:error=0.5,latency=50ms"`
- Expected: ONE behavior for order-api with BOTH error and latency
- Purpose: Demonstrates that comma-separated values after a service prefix continue to apply to that service

**Service with Multiple Behaviors Combined**
- Input: `"order-api:error=500:0.5,latency=100ms"`
- Expected: One behavior for order-api with both error and latency
- Purpose: Test combining multiple behaviors for one service

**Complex Chain**
- Input: `"order-api:error=500:0.3,product-api:latency=100-200ms,payment-api:error=0.1,latency=50ms"`
- Expected: Three behaviors:
  1. order-api: error only
  2. product-api: latency only  
  3. payment-api: error + latency (latency continues from payment-api prefix)
- Purpose: Test complex real-world scenario

---

### 3. TestBehaviorChainForService - Service-Specific Behavior Resolution

Tests the `ForService()` method that extracts applicable behaviors for a specific service.

#### Scenarios:

**Specific Behavior for Service**
- Chain: `"order-api:error=500:0.5,product-api:latency=100ms"`
- Query: `ForService("order-api")`
- Expected: Returns error behavior only
- Purpose: Verify service-specific behaviors are extracted correctly

**Global Behavior Applied to Service**
- Chain: `"latency=50ms"`
- Query: `ForService("any-service")`
- Expected: Returns latency behavior
- Purpose: Verify global behaviors apply to all services

**Service with Multiple Behaviors in Sequence**
- Chain: `"order-api:error=0.5,latency=100ms"`
- Query: `ForService("order-api")`
- Expected: Returns behavior with BOTH error and latency
- Purpose: Verify chained behaviors are combined correctly

**No Behavior for Unmatched Service**
- Chain: `"order-api:error=0.5"`
- Query: `ForService("product-api")`
- Expected: Returns nil (no behavior)
- Purpose: Verify unmatched services get no behavior when no global exists

**Specific Overrides Global Latency**
- Chain: `"latency=100ms,order-api:latency=50ms"`
- Query: `ForService("order-api")`
- Expected: Returns 50ms latency (specific overrides global)
- Purpose: Verify service-specific behaviors take precedence

**Global Applied When No Specific Match**
- Chain: `"latency=50ms,order-api:error=0.5"`
- Query: `ForService("product-api")`
- Expected: Returns global latency behavior
- Purpose: Verify unmatched services fall back to global behavior

---

### 4. TestBehaviorString - Behavior Serialization

Tests the `String()` method for converting behaviors back to string format.

#### Scenarios:

**Latency Only**
- Input: `"latency=100ms"`
- Output: `"latency=100ms"`
- Purpose: Verify round-trip conversion

**Error with Default Code**
- Input: `"error=0.5"`
- Output: `"error=0.5"` (code 500 is default, not shown)
- Purpose: Verify default code is omitted in output

**Error with Custom Code**
- Input: `"error=503:0.5"`
- Output: `"error=0.5,code=503"`
- Purpose: Verify non-default codes are included

**Combined Behaviors**
- Input: `"latency=100ms,error=500:0.3"`
- Output: `"latency=100ms,error=0.3"` (default code omitted)
- Purpose: Verify multiple behaviors serialize correctly

---

### 5. TestBehaviorChainString - Chain Serialization

Tests the chain `String()` method for converting behavior chains back to string format.

#### Scenarios:

**Single Global Behavior**
- Input: `"latency=100ms"`
- Output: `"latency=100ms"`

**Single Targeted Behavior**
- Input: `"order-api:error=500:0.5"`
- Output: `"order-api:error=0.5"`

**Multiple Targeted Behaviors**
- Input: `"order-api:error=0.5,product-api:latency=100ms"`
- Output: `"order-api:error=0.5,product-api:latency=100ms"`

Purpose: Verify chains can be serialized for propagation to upstream services

---

### 6. TestShouldError - Error Probability Logic

Tests the error injection probability logic using statistical sampling.

#### Scenarios:

**50% Error Rate**
- Behavior: `"error=0.5,code=500"`
- Iterations: 1000
- Expected: ~50% errors (within 10% tolerance)
- Purpose: Verify probability is respected

**10% Error Rate**
- Behavior: `"error=0.1,code=503"`
- Iterations: 1000
- Expected: ~10% errors (within 5% tolerance)
- Purpose: Test low probability edge case

**100% Error Rate**
- Behavior: `"error=503"`
- Iterations: 100
- Expected: 100% errors (no variance)
- Purpose: Verify guaranteed errors work

---

### 7. TestApplyBehavior - Behavior Execution

Tests that behaviors are actually applied correctly.

#### Scenarios:

**Latency Applied**
- Behavior: `"latency=100ms"`
- Expected: Request delayed by ~100ms
- Purpose: Verify latency is actually applied

**Range Latency**
- Behavior: `"latency=50-100ms"`
- Expected: Delay between 50-100ms
- Purpose: Verify random latency within range

---

### 8. TestGetAppliedBehaviors - Behavior Reporting

Tests the reporting of which behaviors were applied (for observability).

#### Scenarios:

**Latency Only**
- Input: `"latency=100ms"`
- Expected: `["latency:fixed"]`

**Error Only**
- Input: `"error=500:0.5"`
- Expected: `["error:500:0.50"]`

**Combined Behaviors**
- Input: `"latency=100ms,error=503:0.3"`
- Expected: `["latency:fixed", "error:503:0.30"]`

**CPU Behavior**
- Input: `"cpu=spike"`
- Expected: `["cpu:spike"]`

**Memory Behavior**
- Input: `"memory=leak-slow"`
- Expected: `["memory:leak-slow"]`

Purpose: Verify applied behaviors are correctly reported for observability and debugging

---

## Key Learnings from Tests

### 1. Comma Behavior in Chains

When parsing chains, commas have special meaning:
- After a `service:` prefix, commas continue that service's behavior
- A new `service:` prefix starts a new behavior context

Example:
```
"order-api:error=0.5,latency=100ms"
```
Creates ONE behavior for order-api with both error and latency.

### 2. Error Syntax

Two formats are supported:
- `error=0.5` - Probability only (default code 500)
- `error=503:0.3` - Code and probability (colon-separated)

### 3. Specific vs Global Behaviors

- Service-specific behaviors COMPLETELY override global behaviors
- If no service-specific behavior exists, global behavior is used
- There is no merging between specific and global

Example:
```
"latency=100ms,order-api:error=0.5"
```
- order-api gets: ONLY error (no latency)
- other services get: latency only (no error)

### 4. Behavior Propagation

The `String()` methods enable behavior propagation:
1. Web service receives behavior string from client
2. Parses it and applies behaviors for "web"
3. Propagates behavior string to upstream services
4. Each upstream service extracts its own behaviors

---

## Running the Tests

```bash
# Run all behavior tests
go test -v ./pkg/service/behavior/

# Run specific test
go test -v ./pkg/service/behavior/ -run TestParseChain

# Run with race detection
go test -race ./pkg/service/behavior/
```

---

## Test Statistics

- **Total Test Functions**: 8
- **Total Test Cases**: 46
- **Execution Time**: ~190ms
- **Coverage**: Parsing, serialization, execution, probability, and service targeting

