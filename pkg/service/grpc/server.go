package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/client"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/handler"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/telemetry"
	pb "github.com/aslakknutsen/kkbase/testapp/proto/testservice"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.uber.org/zap"
	grpc_codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Server implements the TestService gRPC server
type Server struct {
	pb.UnimplementedTestServiceServer
	config    *service.Config
	telemetry *telemetry.Telemetry
	caller    *client.Caller
	handler   *handler.RequestHandler
}

// NewServer creates a new gRPC server
func NewServer(cfg *service.Config, tel *telemetry.Telemetry) *Server {
	caller := client.NewCaller(tel)
	return &Server{
		config:    cfg,
		telemetry: tel,
		caller:    caller,
		handler:   handler.NewRequestHandler(cfg, caller, tel),
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

	// Get trace IDs
	var traceID, spanID string
	if spanCtx := span.SpanContext(); spanCtx.IsValid() {
		traceID = spanCtx.TraceID().String()
		spanID = spanCtx.SpanID().String()
	}

	// Build request context
	reqCtx := &handler.RequestContext{
		Ctx:         ctx,
		StartTime:   start,
		TraceID:     traceID,
		SpanID:      spanID,
		BehaviorStr: req.Behavior,
	}

	// Process request with handler (behavior execution)
	processResult, err := s.handler.ProcessRequest(reqCtx, "grpc")
	if err != nil {
		s.telemetry.Logger.Error("Failed to process request", zap.Error(err))
		span.RecordError(err)
		span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(grpc_codes.Internal)))
		span.SetStatus(codes.Error, err.Error())
		return nil, status.Errorf(grpc_codes.Internal, "Internal error: %v", err)
	}

	// If early exit (behavior triggered error), return response
	if processResult.EarlyExit {
		statusCode := int(processResult.Response.Code)
		grpcCode := httpToGRPCCode(statusCode)

		span.SetAttributes(
			semconv.RPCGRPCStatusCodeKey.Int(int(grpcCode)),
			semconv.ErrorTypeKey.String(fmt.Sprintf("grpc_%d", statusCode)),
		)
		span.SetStatus(codes.Error, processResult.Response.Body)
		return processResult.Response, nil
	}

	// Use effective behaviors applied (includes defaults like upstreamWeights)
	behaviorsApplied := processResult.BehaviorsApplied

	// Call upstreams (all configured upstreams for gRPC)
	// - behaviorsApplied: used for routing decisions (includes defaults)
	// - req.Behavior: propagated to downstream (external behavior only)
	upstreamCalls, err := s.handler.CallUpstreams(ctx, behaviorsApplied, req.Behavior, nil)
	if err != nil {
		s.telemetry.Logger.Error("Failed to call upstreams", zap.Error(err))
		span.RecordError(err)
		span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(grpc_codes.Internal)))
		span.SetStatus(codes.Error, err.Error())
		return nil, status.Errorf(grpc_codes.Internal, "Upstream call failed: %v", err)
	}

	// Check if any upstream returned non-2xx (excluding connection errors where Code=0)
	var resp *pb.ServiceResponse
	if failedCall := s.handler.CheckUpstreamFailures(upstreamCalls); failedCall != nil {
		resp = s.handler.BuildUpstreamErrorResponse(reqCtx, "grpc", failedCall, behaviorsApplied, upstreamCalls)

		span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(grpc_codes.Unavailable)))
		span.SetStatus(codes.Error, resp.Body)

		// Record application-level metrics (since we're not returning gRPC error)
		s.telemetry.RecordGRPCRequest("Call", int(resp.Code), time.Since(start))

		// Return response without gRPC error so upstream_calls are preserved
		// The error info is in resp.Code and resp.Body
		return resp, nil
	}

	// Build success response
	resp = s.handler.BuildSuccessResponse(reqCtx, "grpc", behaviorsApplied, upstreamCalls)

	span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(grpc_codes.OK)))
	span.SetStatus(codes.Ok, "")

	// Record application-level metrics
	s.telemetry.RecordGRPCRequest("Call", int(resp.Code), time.Since(start))

	return resp, nil
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
