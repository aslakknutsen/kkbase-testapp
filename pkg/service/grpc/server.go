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

	var behaviorsApplied []string
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
			behaviorsApplied = beh.GetAppliedBehaviors()

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

		behaviorsApplied = beh.GetAppliedBehaviors()

		// Record applied behaviors
		for _, applied := range behaviorsApplied {
			s.telemetry.RecordBehavior(applied)
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
func (s *Server) buildResponse(ctx context.Context, start time.Time, code int, behaviorsApplied []string, upstreamCalls []*pb.UpstreamCall) *pb.ServiceResponse {
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
