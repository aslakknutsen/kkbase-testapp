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
}

// NewServer creates a new HTTP server
func NewServer(cfg *service.Config, tel *telemetry.Telemetry) *Server {
	return &Server{
		config:    cfg,
		telemetry: tel,
		caller:    client.NewCaller(tel),
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
	s.telemetry.IncActiveRequests("http")
	defer s.telemetry.DecActiveRequests("http")

	// Create response
	resp := &pb.ServiceResponse{
		Service: &pb.ServiceInfo{
			Name:      s.config.Name,
			Version:   s.config.Version,
			Namespace: s.config.Namespace,
			Pod:       s.config.PodName,
			Node:      s.config.NodeName,
			Protocol:  "http",
		},
		StartTime: start.Format(time.RFC3339Nano),
		Url:       r.URL.RequestURI(),
	}

	// Get trace IDs
	if spanCtx := span.SpanContext(); spanCtx.IsValid() {
		resp.TraceId = spanCtx.TraceID().String()
		resp.SpanId = spanCtx.SpanID().String()
	}

	// Parse behavior from query parameters or headers
	behaviorStr := r.URL.Query().Get("behavior")
	if behaviorStr == "" {
		behaviorStr = r.Header.Get("X-Behavior")
	}
	if behaviorStr == "" {
		behaviorStr = s.config.DefaultBehavior
	}

	// Parse behavior chain (supports targeted behaviors like "service:latency=100ms")
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
			now := time.Now()
			resp.Code = int32(errCode)
			resp.Body = fmt.Sprintf("Injected error: %d", errCode)
			resp.BehaviorsApplied = beh.GetAppliedBehaviors()
			resp.EndTime = now.Format(time.RFC3339Nano)
			resp.Duration = now.Sub(start).String()

			s.telemetry.RecordBehavior("error")
			s.sendResponse(w, resp, errCode, span, start)
			return
		}

		resp.BehaviorsApplied = beh.GetAppliedBehaviors()

		// Record applied behaviors
		for _, applied := range resp.BehaviorsApplied {
			s.telemetry.RecordBehavior(applied)
		}
	}

	// Check if upstreams are configured
	// If no upstreams at all, this is a leaf service - skip upstream calls
	if len(s.config.Upstreams) > 0 {
		// Match upstreams based on request path
		matchedUpstreams := s.matchUpstreamsForPath(r.URL.Path)

		// If upstreams are configured but none match, return 404
		if len(matchedUpstreams) == 0 {
			now := time.Now()
			resp.Code = 404
			resp.Body = fmt.Sprintf("No upstream matches path: %s", r.URL.Path)
			resp.EndTime = now.Format(time.RFC3339Nano)
			resp.Duration = now.Sub(start).String()

			s.telemetry.RecordBehavior("path_not_found")
			s.sendResponse(w, resp, 404, span, start)
			return
		}

		// Call matched upstreams
		resp.UpstreamCalls = s.callMatchedUpstreams(ctx, matchedUpstreams, r.URL.Path, behaviorStr)
	}

	// Set success response
	now := time.Now()
	resp.Code = 200
	resp.Body = fmt.Sprintf("Hello from %s (HTTP)", s.config.Name)
	resp.EndTime = now.Format(time.RFC3339Nano)
	resp.Duration = now.Sub(start).String()

	s.sendResponse(w, resp, 200, span, start)
}

// resultToUpstreamCall converts a client.Result to pb.UpstreamCall (for upstream calls)
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

	// Convert nested calls
	if len(result.UpstreamCalls) > 0 {
		call.UpstreamCalls = make([]*pb.UpstreamCall, len(result.UpstreamCalls))
		for i, uc := range result.UpstreamCalls {
			call.UpstreamCalls[i] = s.resultToUpstreamCall(uc)
		}
	}

	return call
}

// matchUpstreamsForPath returns upstreams that match the given path
func (s *Server) matchUpstreamsForPath(path string) map[string]*service.UpstreamConfig {
	// If no upstreams configured, return empty
	if len(s.config.Upstreams) == 0 {
		return nil
	}

	matched := make(map[string]*service.UpstreamConfig)
	hasAnyPathConfig := false

	for name, upstream := range s.config.Upstreams {
		if len(upstream.Paths) == 0 {
			// No paths configured = catch-all
			matched[name] = upstream
		} else {
			hasAnyPathConfig = true
			// Check if path matches any prefix
			for _, prefix := range upstream.Paths {
				if strings.HasPrefix(path, prefix) {
					matched[name] = upstream
					break
				}
			}
		}
	}

	// If some upstreams have path config but none matched, return empty
	if hasAnyPathConfig && len(matched) == 0 {
		return nil
	}

	return matched
}

// stripMatchedPrefix strips the matched path prefix from the request path
func (s *Server) stripMatchedPrefix(path string, upstream *service.UpstreamConfig) string {
	if len(upstream.Paths) == 0 {
		return path // No paths configured, don't strip
	}

	// Find longest matching prefix
	longestMatch := ""
	for _, prefix := range upstream.Paths {
		if strings.HasPrefix(path, prefix) && len(prefix) > len(longestMatch) {
			longestMatch = prefix
		}
	}

	if longestMatch != "" {
		stripped := strings.TrimPrefix(path, longestMatch)
		if stripped == "" {
			return "/"
		}
		return stripped
	}
	return path
}

// callMatchedUpstreams calls the matched upstreams with path stripping
func (s *Server) callMatchedUpstreams(ctx context.Context, upstreams map[string]*service.UpstreamConfig, requestPath string, behaviorStr string) []*pb.UpstreamCall {
	var calls []*pb.UpstreamCall

	for name, upstream := range upstreams {
		// Strip matched prefix from path
		forwardPath := s.stripMatchedPrefix(requestPath, upstream)

		// Update upstream URL to include the path
		upstreamWithPath := &service.UpstreamConfig{
			Name:     upstream.Name,
			URL:      upstream.URL + forwardPath,
			Protocol: upstream.Protocol,
			Paths:    upstream.Paths,
		}

		// Use shared caller with behavior propagation
		result := s.caller.Call(ctx, name, upstreamWithPath, behaviorStr)

		// Convert to pb.UpstreamCall and record metrics
		call := s.resultToUpstreamCall(result)
		s.telemetry.RecordUpstreamCall(name, int(call.Code), result.Duration)

		calls = append(calls, call)
	}

	return calls
}

// sendResponse sends the JSON response using protojson
func (s *Server) sendResponse(w http.ResponseWriter, resp *pb.ServiceResponse, statusCode int, span trace.Span, start time.Time) {
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
	s.telemetry.RecordRequest("http", "GET", statusCode, duration)

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
