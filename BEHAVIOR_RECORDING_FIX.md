# Behavior Recording Fix

## Problem

When error behaviors were injected (e.g., `order-api:error=0.5,code=500`), the upstream call chain wasn't properly recording:
1. The actual error code (500) returned by the service
2. Which behaviors were applied by each service in the chain

### Specific Issues

1. **gRPC calls**: The caller was hardcoding `result.Code = 200` instead of reading the actual code from the response
2. **Missing behavior info**: Neither HTTP nor gRPC callers were capturing the `behaviors_applied` field from upstream responses

## Solution

### 1. Added `BehaviorsApplied` field to Result struct

Updated `pkg/service/client/caller.go`:
```go
type Result struct {
    Name             string
    URI              string
    Protocol         string
    Duration         time.Duration
    Code             int
    Error            string
    BehaviorsApplied []string  // NEW FIELD
    UpstreamCalls    []Result
}
```

### 2. Fixed gRPC caller to use actual response code

Changed in `callGRPC()`:
```go
// OLD: result.Code = 200 // gRPC success maps to HTTP 200
// NEW:
result.Code = int(resp.Code)  // Use actual code from response
result.BehaviorsApplied = resp.BehaviorsApplied
```

### 3. Updated HTTP caller to capture behaviors

Changed in `callHTTP()`:
- Parse `behaviors_applied` field from HTTP response body
- Include it in the result

### 4. Updated protobuf definition

Added `behaviors_applied` field to `UpstreamCall` message in `proto/testservice/service.proto`:
```proto
message UpstreamCall {
  // ... existing fields ...
  repeated UpstreamCall upstream_calls = 7;
  repeated string behaviors_applied = 8;  // NEW FIELD
}
```

### 5. Updated type definitions

Added `BehaviorsApplied` field to `service.UpstreamCall` in `pkg/service/types.go`.

### 6. Updated conversion functions

- Updated `resultToUpstreamCall()` in both HTTP and gRPC servers to include `BehaviorsApplied`
- Added recursive conversion helpers in caller.go to properly handle nested upstream calls

## Expected Behavior After Fix

When calling:
```bash
curl -H "web.local" localhost:8080/product?behavior=order-api:error=0.5,code=500
```

### When order-api returns error (50% chance):
```json
{
  "service": {"name": "web", ...},
  "code": 200,
  "upstream_calls": [
    {
      "name": "order-api",
      "protocol": "grpc",
      "code": 500,  // <-- NOW SHOWS 500 CORRECTLY
      "behaviors_applied": ["error:500:0.50"],  // <-- NOW SHOWS APPLIED BEHAVIOR
      "upstream_calls": []  // Empty because error was returned before making calls
    },
    {
      "name": "product-api",
      "protocol": "http",
      "code": 200
    }
  ]
}
```

### When order-api succeeds (50% chance):
```json
{
  "service": {"name": "web", ...},
  "code": 200,
  "upstream_calls": [
    {
      "name": "order-api",
      "protocol": "grpc",
      "code": 200,  // <-- Shows success
      "upstream_calls": [
        {"name": "product-api", "code": 200},
        {"name": "payment-api", "code": 200},
        {"name": "order-db", "code": 200}
      ]
    },
    {
      "name": "product-api",
      "code": 200
    }
  ]
}
```

## Files Modified

1. `testapp/pkg/service/client/caller.go` - Fixed code recording and added behaviors tracking
2. `testapp/pkg/service/http/server.go` - Updated conversion to include behaviors
3. `testapp/pkg/service/grpc/server.go` - Updated conversion to include behaviors
4. `testapp/pkg/service/types.go` - Added BehaviorsApplied field
5. `testapp/proto/testservice/service.proto` - Added behaviors_applied to proto
6. `testapp/proto/testservice/service.pb.go` - Regenerated from proto
7. `testapp/proto/testservice/service_grpc.pb.go` - Regenerated from proto

## Testing

To test the fix:
1. Rebuild and redeploy the testservice
2. Make multiple calls with error injection behavior
3. Verify that when errors occur, the upstream_calls show the correct error code and applied behaviors

