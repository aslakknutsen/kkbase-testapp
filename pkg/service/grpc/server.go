package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/behavior"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/client"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/telemetry"
	pb "github.com/aslakknutsen/kkbase/testapp/proto/testservice"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	grpc_codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
)

// Server implements the TestService gRPC server
type Server struct {
	pb.UnimplementedTestServiceServer
	config    *service.Config
	telemetry *telemetry.Telemetry
	caller    *client.Caller
}

// NewServer creates a new gRPC server
func NewServer(cfg *service.Config, tel *telemetry.Telemetry) *Server {
	return &Server{
		config:    cfg,
		telemetry: tel,
		caller:    client.NewCaller(tel),
	}
}

// Call handles a gRPC call with configurable behavior
func (s *Server) Call(ctx context.Context, req *pb.CallRequest) (*pb.ServiceResponse, error) {
	start := time.Now()

	// Extract trace context from metadata
	ctx = ExtractTraceContext(ctx)

	// Start span with gRPC semantic naming: $package.$service/$method
	ctx, span := s.telemetry.StartServerSpan(ctx, "testservice.TestService/Call",
		semconv.RPCSystemGRPC,
		semconv.RPCService("testservice.TestService"),
		semconv.RPCMethod("Call"),
		semconv.NetworkProtocolName("grpc"),
		semconv.NetworkTransportTCP,
		semconv.ServerAddress("localhost"),
		semconv.ServerPort(s.config.GRPCPort),
		semconv.ClientAddress(extractClientAddr(ctx)),
	)
	defer span.End()

	// Note: gRPC active request tracking is not part of standard gRPC metrics
	// The interceptors track started_total and handled_total instead
	// s.telemetry.IncActiveRequests("grpc")
	// defer s.telemetry.DecActiveRequests("grpc")

	// Parse behavior chain (supports targeted behaviors like "service:latency=100ms")
	behaviorStr := req.Behavior
	if behaviorStr == "" {
		behaviorStr = s.config.DefaultBehavior
	}

	var behaviorsApplied string
	statusCode := 0 // 0 = OK in gRPC

	behaviorChain, err := behavior.ParseChain(behaviorStr)
	if err != nil {
		s.telemetry.Logger.Warn("Failed to parse behavior chain", zap.Error(err))
		span.RecordError(err)
	}

	// Extract behavior for THIS service
	var beh *behavior.Behavior
	if behaviorChain != nil {
		beh = behaviorChain.ForService(s.config.Name)
	}

	// Apply behavior (only if it targets this service)
	if beh != nil {
		// Apply behavior
		if err := beh.Apply(ctx); err != nil {
			s.telemetry.Logger.Warn("Failed to apply behavior", zap.Error(err))
		}

		// Apply disk behavior separately (needs trace ID)
		if beh.Disk != nil {
			// Get trace ID from span context
			spanCtx := span.SpanContext()
			traceID := spanCtx.TraceID().String()
			if err := beh.ApplyDisk(ctx, traceID); err != nil {
				// Disk fill failed (e.g., disk full) - return ResourceExhausted
				s.telemetry.Logger.Warn("Disk fill failed",
					zap.Error(err),
					zap.String("path", beh.Disk.Path),
					zap.Int64("size", beh.Disk.Size),
				)
				
				statusCode := 507 // HTTP 507 Insufficient Storage
				behaviorsApplied = beh.String()
				
				resp := s.buildResponse(ctx, start, statusCode, behaviorsApplied, nil)
				resp.Body = fmt.Sprintf("Disk fill failed: %v", err)

				s.telemetry.RecordBehavior("disk-fill-failed")
				
				span.SetAttributes(
					semconv.RPCGRPCStatusCodeKey.Int(int(grpc_codes.ResourceExhausted)),
					semconv.ErrorTypeKey.String("disk_full"),
				)
				span.SetStatus(codes.Error, fmt.Sprintf("Disk fill failed: %v", err))
				return resp, nil
			}
		}

		// Check for crash-if-file (do this BEFORE panic and error checks)
		if shouldCrash, matched, msg := beh.ShouldCrashOnFile(); shouldCrash {
			s.telemetry.Logger.Fatal("Config file contains invalid content - crashing as configured",
				zap.String("service", s.config.Name),
				zap.String("file", beh.CrashIfFile.FilePath),
				zap.String("matched_content", matched),
				zap.String("message", msg),
			)
			panic(fmt.Sprintf("Config file crash: %s", msg))
		} else if msg != "" {
			// Log file read errors without crashing
			s.telemetry.Logger.Warn("Failed to check config file for invalid content",
				zap.String("file", beh.CrashIfFile.FilePath),
				zap.String("error", msg),
			)
		}

		// Check for error-if-file (do this BEFORE panic and error checks)
		if shouldErr, errCode, matched, msg := beh.ShouldErrorOnFile(); shouldErr {
			s.telemetry.Logger.Warn("File contains invalid content - returning error as configured",
				zap.String("service", s.config.Name),
				zap.String("file", beh.ErrorIfFile.FilePath),
				zap.String("matched_content", matched),
				zap.Int("error_code", errCode),
				zap.String("message", msg),
			)
			
			grpcCode := httpToGRPCCode(errCode)
			behaviorsApplied = beh.String()
			
			resp := s.buildResponse(ctx, start, errCode, behaviorsApplied, nil)
			resp.Body = fmt.Sprintf("File validation failed: %s", msg)

			s.telemetry.RecordBehavior("error-if-file")
			
			span.SetAttributes(
				semconv.RPCGRPCStatusCodeKey.Int(int(grpcCode)),
				semconv.ErrorTypeKey.String(fmt.Sprintf("grpc_%d", errCode)),
			)
			span.SetStatus(codes.Error, msg)
			return resp, nil
		} else if msg != "" {
			// Log file read errors without returning error
			s.telemetry.Logger.Warn("Failed to check file for invalid content",
				zap.String("file", beh.ErrorIfFile.FilePath),
				zap.String("error", msg),
			)
		}

		// Check for panic injection (do this BEFORE error check)
		if beh.ShouldPanic() {
			s.telemetry.Logger.Fatal("Panic behavior triggered - crashing pod",
				zap.String("service", s.config.Name),
				zap.Float64("panic_prob", beh.Panic.Prob),
			)
			panic(fmt.Sprintf("Panic behavior triggered in service %s", s.config.Name))
		}

		// Check for error injection
		if shouldErr, errCode := beh.ShouldError(); shouldErr {
			statusCode = errCode
			behaviorsApplied = beh.String()

			resp := s.buildResponse(ctx, start, statusCode, behaviorsApplied, nil)
			resp.Body = fmt.Sprintf("Injected error: %d", errCode)

			s.telemetry.RecordBehavior("error")
			// Note: gRPC metrics are now handled by interceptors
			// s.recordMetrics(statusCode, start)

			span.SetAttributes(
				semconv.RPCGRPCStatusCodeKey.Int(int(grpc_codes.Internal)),
				semconv.ErrorTypeKey.String(fmt.Sprintf("grpc_%d", errCode)),
			)
			span.SetStatus(codes.Error, fmt.Sprintf("Injected error: %d", errCode))
			return resp, nil
		}

		behaviorsApplied = beh.String()

		// Record applied behaviors
		if behaviorsApplied != "" {
			s.telemetry.RecordBehavior(behaviorsApplied)
		}
	}

	// Make upstream calls with behavior chain propagated
	upstreamCalls := s.callAllUpstreams(ctx, behaviorStr)

	// Build response
	resp := s.buildResponse(ctx, start, 200, behaviorsApplied, upstreamCalls)
	resp.Body = fmt.Sprintf("Hello from %s (gRPC)", s.config.Name)

	// Note: gRPC metrics are now handled by interceptors, so we don't need to record them here
	// s.recordMetrics(200, start)
	
	span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(grpc_codes.OK)))
	span.SetStatus(codes.Ok, "")

	return resp, nil
}

// callAllUpstreams calls all configured upstream services with behavior propagation
func (s *Server) callAllUpstreams(ctx context.Context, behaviorStr string) []*pb.UpstreamCall {
	var calls []*pb.UpstreamCall

	for name, upstream := range s.config.Upstreams {
		upstreamToCall := upstream

		// If upstream has paths configured and is HTTP, append the first path
		// (gRPC doesn't use URL paths, so only do this for non-gRPC upstreams)
		if len(upstream.Paths) > 0 && upstream.Protocol != "grpc" {
			upstreamToCall = &service.UpstreamConfig{
				Name:     upstream.Name,
				URL:      upstream.URL + upstream.Paths[0], // Use first configured path
				Protocol: upstream.Protocol,
				Paths:    upstream.Paths,
			}
		}

		// Use shared caller with behavior propagation
		result := s.caller.Call(ctx, name, upstreamToCall, behaviorStr)

		// Convert to pb.UpstreamCall and record metrics
		call := s.resultToUpstreamCall(result)
		// For gRPC upstreams, method is "Call", for HTTP upstreams it's "GET"
		method := "Call"
		if result.Protocol == "http" {
			method = "GET"
		}
		s.telemetry.RecordUpstreamCall(method, name, int(call.Code), result.Duration)

		calls = append(calls, call)
	}

	return calls
}

// resultToUpstreamCall converts a client.Result to pb.UpstreamCall
func (s *Server) resultToUpstreamCall(result client.Result) *pb.UpstreamCall {
	call := &pb.UpstreamCall{
		Name:             result.Name,
		Uri:              result.URL,
		Protocol:         result.Protocol,
		Code:             int32(result.Code),
		Duration:         result.Duration.String(),
		Error:            result.Error,
		BehaviorsApplied: result.BehaviorsApplied,
	}

	// Convert nested calls recursively
	if len(result.UpstreamCalls) > 0 {
		call.UpstreamCalls = make([]*pb.UpstreamCall, len(result.UpstreamCalls))
		for i, uc := range result.UpstreamCalls {
			call.UpstreamCalls[i] = s.resultToUpstreamCall(uc)
		}
	}

	return call
}

// buildResponse constructs the gRPC response
func (s *Server) buildResponse(ctx context.Context, start time.Time, code int, behaviorsApplied string, upstreamCalls []*pb.UpstreamCall) *pb.ServiceResponse {
	now := time.Now()

	resp := &pb.ServiceResponse{
		Service: &pb.ServiceInfo{
			Name:      s.config.Name,
			Version:   s.config.Version,
			Namespace: s.config.Namespace,
			Pod:       s.config.PodName,
			Node:      s.config.NodeName,
			Protocol:  "grpc",
		},
		StartTime:        start.Format(time.RFC3339Nano),
		EndTime:          now.Format(time.RFC3339Nano),
		Duration:         now.Sub(start).String(),
		Code:             int32(code),
		BehaviorsApplied: behaviorsApplied,
		UpstreamCalls:    upstreamCalls,
	}

	// Extract trace IDs from span context
	span := trace.SpanFromContext(ctx)
	spanCtx := span.SpanContext()
	if spanCtx.IsValid() {
		resp.TraceId = spanCtx.TraceID().String()
		resp.SpanId = spanCtx.SpanID().String()
	}

	return resp
}

// recordMetrics records request metrics
func (s *Server) recordMetrics(statusCode int, start time.Time) {
	duration := time.Since(start)
	s.telemetry.RecordRequest("grpc", "Call", statusCode, duration)

	s.telemetry.Logger.Info("grpc_request_completed",
		zap.Int("status", statusCode),
		zap.Duration("duration", duration),
	)
}

// Helper function for extracting client address
func extractClientAddr(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok {
		return p.Addr.String()
	}
	return ""
}

// httpToGRPCCode maps HTTP status codes to gRPC status codes
func httpToGRPCCode(httpCode int) grpc_codes.Code {
	switch httpCode {
	case 400:
		return grpc_codes.InvalidArgument
	case 401:
		return grpc_codes.Unauthenticated
	case 403:
		return grpc_codes.PermissionDenied
	case 404:
		return grpc_codes.NotFound
	case 409:
		return grpc_codes.AlreadyExists
	case 429:
		return grpc_codes.ResourceExhausted
	case 499:
		return grpc_codes.Canceled
	case 500:
		return grpc_codes.Internal
	case 501:
		return grpc_codes.Unimplemented
	case 503:
		return grpc_codes.Unavailable
	case 504:
		return grpc_codes.DeadlineExceeded
	default:
		return grpc_codes.Unknown
	}
}
