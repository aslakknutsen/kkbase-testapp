# TestApp Fixes Summary

This document summarizes all the fixes applied to make TestApp fully functional with mixed HTTP/gRPC protocols.

## Issues Fixed

### 1. Health Probe Separation ✅

**Issue**: Both liveness and readiness probes used the same `/health` endpoint.

**Fix**: 
- Added separate `/ready` endpoint to TestService
- Liveness probe: `/health` (checks if process is alive)
- Readiness probe: `/ready` (checks if ready for traffic)

**Files Modified**:
- `cmd/testservice/main.go`: Added `/ready` handler
- `pkg/generator/k8s/generator.go`: Updated probe generation

**Benefit**: Better probe semantics following Kubernetes best practices.

---

### 2. Health Probes for All Service Types ✅

**Issue**: gRPC-only services had HTTP port set to 0, causing invalid health probe configurations.

**Error**:
```
port: Invalid value: 0: must be between 1 and 65535
```

**Fix**:
- TestService always runs HTTP server on port 8080 for health checks
- All services get HTTP port 8080 set (even gRPC-only)
- HTTP port used internally for health but not exposed in Service for gRPC-only

**Files Modified**:
- `pkg/dsl/types/types.go`: Always set HTTP port to 8080
- `pkg/generator/k8s/generator.go`: Always generate health probes

**Benefit**: All services have working health checks regardless of protocol.

---

### 3. HTTP→gRPC Upstream Calls ✅

**Issue**: HTTP server couldn't call gRPC upstreams.

**Error**:
```
unsupported protocol scheme "grpc"
```

**Fix**:
- Added protocol detection in HTTP server's `callUpstream()`
- Implemented `callUpstreamGRPC()` method to handle gRPC connections
- Proper trace propagation via gRPC metadata
- Response conversion from gRPC to standard format

**Files Modified**:
- `pkg/service/http/server.go`: Added gRPC client support
- `pkg/service/http/metadata.go`: Added metadata carrier for trace propagation

**Benefit**: HTTP services can now call gRPC upstreams seamlessly.

---

### 4. gRPC→gRPC Upstream Calls ✅

**Issue**: gRPC server wasn't stripping `grpc://` prefix when calling gRPC upstreams.

**Error**:
```
rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing: dial tcp: address grpc://payment-api.payments.svc.cluster.local:9090: too many colons in address
```

**Fix**:
- Strip `grpc://` prefix before passing to `grpc.Dial()`
- Implemented full HTTP upstream call support (was a stub)
- Proper trace propagation for both protocols

**Files Modified**:
- `pkg/service/grpc/server.go`: 
  - Fixed `callUpstreamGRPC()` to strip protocol prefix
  - Implemented `callUpstreamHTTP()` for HTTP upstream calls

**Benefit**: All protocol combinations now work:
- HTTP → HTTP ✅
- HTTP → gRPC ✅
- gRPC → HTTP ✅
- gRPC → gRPC ✅

---

## Protocol Support Matrix

| Caller Protocol | Upstream Protocol | Status | Notes |
|-----------------|-------------------|--------|-------|
| HTTP | HTTP | ✅ Working | Standard HTTP client |
| HTTP | gRPC | ✅ Working | Uses gRPC client with trace propagation |
| gRPC | HTTP | ✅ Working | Uses HTTP client with trace propagation |
| gRPC | gRPC | ✅ Working | Uses gRPC client with metadata propagation |

---

## Testing

### Build & Generate

```bash
cd testapp
make build
./testgen generate examples/ecommerce/app.yaml -o output
```

### Validate Manifests

```bash
kubectl apply --dry-run=client -f output/ecommerce/
```

### Deploy

```bash
kubectl apply -f output/ecommerce/
```

### Test Mixed Protocol Call Chain

Example e-commerce app call chain:
```
web (HTTP) → order-api (gRPC) → product-api (HTTP) → product-db (HTTP)
                               → payment-api (gRPC)
                               → order-db (HTTP)
```

All combinations work with full trace propagation!

---

## Trace Propagation

### HTTP → HTTP
- W3C trace-context headers propagated automatically

### HTTP → gRPC  
- Extract OpenTelemetry context from HTTP request
- Inject into gRPC metadata
- gRPC server extracts from metadata

### gRPC → HTTP
- Extract context from gRPC metadata
- Inject into HTTP headers
- HTTP server extracts from headers

### gRPC → gRPC
- Propagate via gRPC metadata carrier
- Maintains span relationships

---

## Architecture Summary

```
┌─────────────────────────────────────────────────────┐
│                  TestService Binary                  │
├─────────────────────────────────────────────────────┤
│                                                      │
│  HTTP Server (Port 8080)                            │
│  ├─ /health (liveness)                              │
│  ├─ /ready (readiness)                              │
│  ├─ / (main handler)                                │
│  └─ HTTP & gRPC upstream clients                    │
│                                                      │
│  gRPC Server (Port 9090)                            │
│  ├─ Call() RPC                                      │
│  └─ HTTP & gRPC upstream clients                    │
│                                                      │
│  Metrics Server (Port 9091)                         │
│  └─ /metrics (Prometheus)                           │
│                                                      │
└─────────────────────────────────────────────────────┘
```

---

## Key Design Decisions

1. **Always Run HTTP Server**: Even for gRPC-only services, HTTP server runs for health checks
2. **Separate Endpoints**: `/health` for liveness, `/ready` for readiness
3. **Protocol Detection**: Upstream protocol determined by URL scheme (`http://` vs `grpc://`)
4. **Trace Propagation**: OpenTelemetry context flows through all protocol combinations
5. **Call Chain Reconstruction**: Nested upstream calls preserved across protocol boundaries

---

## Future Enhancements

- [ ] Connection pooling for upstream calls
- [ ] Circuit breaker support
- [ ] Retry policies
- [ ] Timeout configuration per upstream
- [ ] TLS support for gRPC connections
- [ ] Health check depth (check upstream health)
- [ ] Graceful shutdown with connection draining

