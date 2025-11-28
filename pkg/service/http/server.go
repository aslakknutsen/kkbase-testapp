package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/behavior"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/client"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/handler"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/router"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/telemetry"
	pb "github.com/aslakknutsen/kkbase/testapp/proto/testservice"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
)

// Server handles HTTP requests
type Server struct {
	config    *service.Config
	telemetry *telemetry.Telemetry
	caller    *client.Caller
	handler   *handler.RequestHandler
	router    router.Router
}

// NewServer creates a new HTTP server
func NewServer(cfg *service.Config, tel *telemetry.Telemetry) *Server {
	caller := client.NewCaller(tel)
	return &Server{
		config:    cfg,
		telemetry: tel,
		caller:    caller,
		handler:   handler.NewRequestHandler(cfg, caller, tel),
		router:    router.NewPathRouter(cfg.Upstreams),
	}
}

// ServeHTTP handles HTTP requests
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()

	// Extract trace context from HTTP headers
	propagator := otel.GetTextMapPropagator()
	ctx = propagator.Extract(ctx, propagation.HeaderCarrier(r.Header))

	// Start span with HTTP semantic naming: {method} {route}
	spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	ctx, span := s.telemetry.StartServerSpan(ctx, spanName,
		semconv.HTTPRequestMethodOriginal(r.Method),
		semconv.URLScheme(getScheme(r)),
		semconv.URLPath(r.URL.Path),
		semconv.ServerAddress(r.Host),
		semconv.ServerPort(extractPort(r.Host, s.config.HTTPPort)),
		semconv.NetworkProtocolName("http"),
		semconv.NetworkProtocolVersion(extractHTTPVersion(r.Proto)),
		semconv.NetworkTransportTCP,
		semconv.ClientAddress(extractClientIP(r)),
		semconv.UserAgentOriginal(r.UserAgent()),
	)
	defer span.End()

	// Track active requests
	s.telemetry.IncActiveRequests(r.Method, r.URL.Path)
	defer s.telemetry.DecActiveRequests(r.Method, r.URL.Path)

	// Get trace IDs
	var traceID, spanID string
	if spanCtx := span.SpanContext(); spanCtx.IsValid() {
		traceID = spanCtx.TraceID().String()
		spanID = spanCtx.SpanID().String()
	}

	// Parse behavior from query parameters or headers
	behaviorStr := r.URL.Query().Get("behavior")
	if behaviorStr == "" {
		behaviorStr = r.Header.Get("X-Behavior")
	}

	// Build request context
	reqCtx := &handler.RequestContext{
		Ctx:         ctx,
		StartTime:   start,
		TraceID:     traceID,
		SpanID:      spanID,
		BehaviorStr: behaviorStr,
	}

	// Process request with handler (behavior execution)
	processResult, err := s.handler.ProcessRequest(reqCtx, "http")
	if err != nil {
		s.telemetry.Logger.Error("Failed to process request", zap.Error(err))
		span.RecordError(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// If early exit (behavior triggered error), send response
	if processResult.EarlyExit {
		statusCode := int(processResult.Response.Code)
		processResult.Response.Url = r.URL.RequestURI()
		s.sendResponse(w, r, processResult.Response, statusCode, span, start)
		return
	}

	// Use behaviors applied from ProcessRequest (includes defaults like upstreamWeights)
	behaviorsApplied := processResult.BehaviorsApplied

	// Route and call upstreams
	var resp *pb.ServiceResponse
	var upstreamCalls []*pb.UpstreamCall
	if s.router.HasUpstreams() {
		// Extract upstream weights from effective behavior (includes defaults)
		var upstreamWeights map[string]int
		if behaviorsApplied != "" {
			if b, err := behavior.Parse(behaviorsApplied); err == nil && b.UpstreamWeights != nil {
				upstreamWeights = b.UpstreamWeights.Weights
			}
		}

		// Match upstreams based on request path with weighted selection for groups
		matchedUpstreams := s.router.MatchWithWeights(r.URL.Path, upstreamWeights)

		// If upstreams are configured but none match, return 404
		if matchedUpstreams == nil {
			resp = s.handler.BuildSuccessResponse(reqCtx, "http", behaviorsApplied, nil)
			resp.Code = 404
			resp.Body = fmt.Sprintf("No upstream matches path: %s", r.URL.Path)
			resp.Url = r.URL.RequestURI()

			s.telemetry.RecordBehavior("path_not_found")
			s.sendResponse(w, r, resp, 404, span, start)
			return
		}

		// Call matched upstreams - propagate original external behavior only (not defaults)
		// Each downstream service will apply its own defaults if no behavior targets it
		upstreamCalls = s.callMatchedUpstreams(ctx, matchedUpstreams, r.URL.Path, behaviorStr)

		// Check if any upstream returned non-2xx (excluding connection errors where Code=0)
		if failedCall := s.handler.CheckUpstreamFailures(upstreamCalls); failedCall != nil {
			resp = s.handler.BuildUpstreamErrorResponse(reqCtx, "http", failedCall, behaviorsApplied, upstreamCalls)
			resp.Url = r.URL.RequestURI()
			s.sendResponse(w, r, resp, 502, span, start)
			return
		}
	}

	// Build success response
	resp = s.handler.BuildSuccessResponse(reqCtx, "http", behaviorsApplied, upstreamCalls)
	resp.Url = r.URL.RequestURI()
	s.sendResponse(w, r, resp, 200, span, start)
}

// callMatchedUpstreams calls the matched upstreams with explicit forward paths (fail-fast)
func (s *Server) callMatchedUpstreams(ctx context.Context, upstreams []*service.UpstreamConfig, requestPath string, behaviorStr string) []*pb.UpstreamCall {
	var calls []*pb.UpstreamCall

	for _, upstream := range upstreams {
		// Get the explicit forward path (or "/" if not set)
		forwardPath := s.router.GetForwardPath(upstream)

		// Update upstream URL to include the path
		upstreamWithPath := &service.UpstreamConfig{
			Name:     upstream.Name,
			URL:      upstream.URL + forwardPath,
			Protocol: upstream.Protocol,
			Match:    upstream.Match,
			Path:     upstream.Path,
		}

		// Use shared caller with behavior propagation
		result := s.caller.Call(ctx, upstream.Name, upstreamWithPath, behaviorStr)

		// Convert to pb.UpstreamCall using handler's method
		call := s.handler.ResultToUpstreamCall(result)
		s.telemetry.RecordUpstreamCall("GET", upstream.Name, int(call.Code), result.Duration)

		calls = append(calls, call)

		// Fail-fast: stop on first failure (non-2xx response or error)
		if call.Code >= 300 || call.Error != "" {
			break
		}
	}

	return calls
}

// sendResponse sends the JSON response using protojson
func (s *Server) sendResponse(w http.ResponseWriter, r *http.Request, resp *pb.ServiceResponse, statusCode int, span trace.Span, start time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Use protojson for marshaling with proper options
	marshaler := protojson.MarshalOptions{
		UseProtoNames:   true,  // Use snake_case field names from proto
		EmitUnpopulated: false, // Skip zero values (like omitempty)
	}

	jsonBytes, err := marshaler.Marshal(resp)
	if err != nil {
		s.telemetry.Logger.Error("Failed to encode response", zap.Error(err))
		span.RecordError(err)
		return
	}

	if _, err := w.Write(jsonBytes); err != nil {
		s.telemetry.Logger.Error("Failed to write response", zap.Error(err))
		span.RecordError(err)
	}

	// Record metrics
	duration := time.Since(start)
	s.telemetry.RecordRequest(r.Method, r.URL.Path, statusCode, duration)

	// Log request
	s.telemetry.Logger.Info("request_completed",
		zap.Int("status", statusCode),
		zap.Duration("duration", duration),
		zap.String("trace_id", resp.TraceId),
		zap.Int("upstream_calls", len(resp.UpstreamCalls)),
	)

	// Set status code and error attributes
	span.SetAttributes(semconv.HTTPResponseStatusCode(statusCode))
	if statusCode >= 400 {
		span.SetAttributes(semconv.ErrorTypeKey.String(fmt.Sprintf("%d", statusCode)))
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

// Helper functions for extracting HTTP attributes

func getScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}

func extractPort(host string, defaultPort int) int {
	_, portStr, err := net.SplitHostPort(host)
	if err != nil {
		return defaultPort
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return defaultPort
	}
	return port
}

func extractHTTPVersion(proto string) string {
	return strings.TrimPrefix(proto, "HTTP/")
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}
