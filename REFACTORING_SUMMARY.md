# Refactoring: Shared Upstream Caller

## Motivation

The HTTP and gRPC servers had significant code duplication for making upstream calls:

**Before**: 
- HTTP server had its own `callUpstreamHTTP()` and `callUpstreamGRPC()` methods (~120 lines)
- gRPC server had its own `callUpstreamHTTP()` and `callUpstreamGRPC()` methods (~120 lines)
- Metadata carrier duplicated between packages
- Total duplication: ~250 lines of similar code

**Problem**:
- Any bug fix or feature needed to be applied in two places
- Protocol handling logic was duplicated
- Trace propagation code was duplicated
- Response conversion code was duplicated

## Solution

Created a shared `pkg/service/client` package with a unified `Caller` component.

### Architecture

```
┌─────────────────────────────────────────────────┐
│           pkg/service/client/caller.go          │
│                                                  │
│  ┌────────────────────────────────────────────┐ │
│  │           Caller (Unified)                 │ │
│  │  - Call(ctx, name, upstream) → Result     │ │
│  │  - Protocol detection                      │ │
│  │  - HTTP client (with trace propagation)   │ │
│  │  - gRPC client (with metadata propagation)│ │
│  │  - Response conversion                     │ │
│  └────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
                        ↑
           ┌────────────┴────────────┐
           │                         │
┌──────────┴──────────┐   ┌─────────┴─────────┐
│   HTTP Server       │   │   gRPC Server     │
│  - Uses Caller      │   │  - Uses Caller    │
│  - Converts Result  │   │  - Converts Result│
└─────────────────────┘   └───────────────────┘
```

### Key Components

#### 1. `client.Caller`

Central component that handles all upstream calls:

```go
type Caller struct {
    httpClient *http.Client
    tracer     trace.Tracer
}

func (c *Caller) Call(ctx, name, upstream) Result {
    // 1. Start span for tracing
    // 2. Detect protocol (http vs grpc)
    // 3. Route to appropriate client
    // 4. Return standardized Result
}
```

#### 2. `client.Result`

Standardized response format:

```go
type Result struct {
    Name          string
    URI           string
    Protocol      string
    Duration      time.Duration
    Code          int
    Error         string
    UpstreamCalls []Result  // Recursive for call chains
}
```

#### 3. Conversion Methods

Each server converts `client.Result` to its own format:

- HTTP Server: `resultToUpstreamCall() → service.UpstreamCall`
- gRPC Server: `resultToUpstreamCall() → pb.UpstreamCall`

### Benefits

✅ **Code Reuse**: Single implementation of HTTP and gRPC clients  
✅ **Maintainability**: Bug fixes in one place  
✅ **Consistency**: Same behavior across protocols  
✅ **Testability**: Test client logic independently  
✅ **Extensibility**: Easy to add new protocols (e.g., WebSocket)  
✅ **Size Reduction**: Eliminated ~250 lines of duplication  

### Changes Summary

#### Files Created
- `pkg/service/client/caller.go` - Unified upstream caller

#### Files Modified
- `pkg/service/http/server.go`:
  - Removed `callUpstream()`, `callUpstreamHTTP()`, `callUpstreamGRPC()`
  - Added `caller *client.Caller` field
  - Added `resultToUpstreamCall()` converter
  - Simplified `callAllUpstreams()` to use shared caller

- `pkg/service/grpc/server.go`:
  - Removed `callUpstream()`, `callUpstreamHTTP()`, `callUpstreamGRPC()`
  - Added `caller *client.Caller` field
  - Added `resultToUpstreamCall()` converter
  - Simplified `callAllUpstreams()` to use shared caller

#### Files Deleted
- `pkg/service/http/metadata.go` - Moved to client package
- `pkg/service/http/trace.go` - No longer needed

### Code Comparison

**Before** (HTTP Server):
```go
// ~120 lines of HTTP/gRPC client code
func (s *Server) callUpstreamHTTP(...) { ... }
func (s *Server) callUpstreamGRPC(...) { ... }
```

**Before** (gRPC Server):
```go
// ~120 lines of HTTP/gRPC client code (duplicated)
func (s *Server) callUpstreamHTTP(...) { ... }
func (s *Server) callUpstreamGRPC(...) { ... }
```

**After** (Both Servers):
```go
// Simple delegation to shared caller
func (s *Server) callAllUpstreams(ctx context.Context) {
    for name, upstream := range s.config.Upstreams {
        result := s.caller.Call(ctx, name, upstream)
        call := s.resultToUpstreamCall(result)
        s.telemetry.RecordUpstreamCall(name, call.Code, result.Duration)
        calls = append(calls, call)
    }
}
```

### Testing Impact

- ✅ No functional changes - same behavior
- ✅ Build passes without errors
- ✅ All protocol combinations still work:
  - HTTP → HTTP ✅
  - HTTP → gRPC ✅
  - gRPC → HTTP ✅
  - gRPC → gRPC ✅

### Future Enhancements

With this refactoring, it's now much easier to add:

1. **Connection Pooling**: Add to `Caller` once, benefits all
2. **Circuit Breaker**: Centralized failure handling
3. **Retry Logic**: Unified retry policies
4. **New Protocols**: Add WebSocket, NATS, etc. in one place
5. **Advanced Tracing**: Baggage propagation, span events
6. **Metrics**: Centralized client-side metrics

### Migration Notes

No configuration or deployment changes needed. This is purely an internal refactoring that maintains the same external API and behavior.

---

**Result**: Cleaner, more maintainable codebase with ~30% less code for upstream calling logic.

