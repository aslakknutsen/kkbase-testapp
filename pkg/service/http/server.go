package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kagenti/kkbase/testapp/pkg/service"
	"github.com/kagenti/kkbase/testapp/pkg/service/behavior"
	"github.com/kagenti/kkbase/testapp/pkg/service/client"
	"github.com/kagenti/kkbase/testapp/pkg/service/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
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
		caller:    client.NewCaller(),
	}
}

// ServeHTTP handles HTTP requests
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()

	// Extract trace context from headers (handled automatically by OTEL propagator)
	// The shared caller will properly propagate context to upstreams

	// Start span
	ctx, span := s.telemetry.StartSpan(ctx, "http.request",
		attribute.String("http.method", r.Method),
		attribute.String("http.path", r.URL.Path),
	)
	defer span.End()

	// Track active requests
	s.telemetry.IncActiveRequests("http")
	defer s.telemetry.DecActiveRequests("http")

	// Create response
	resp := service.NewResponse(s.config, "http")

	// Capture the request URL (path + query string)
	resp.URL = r.URL.RequestURI()

	// Get trace IDs
	if spanCtx := span.SpanContext(); spanCtx.IsValid() {
		resp.TraceID = spanCtx.TraceID().String()
		resp.SpanID = spanCtx.SpanID().String()
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

		// Check for error injection
		if shouldErr, errCode := beh.ShouldError(); shouldErr {
			resp.Code = errCode
			resp.Body = fmt.Sprintf("Injected error: %d", errCode)
			resp.BehaviorsApplied = beh.GetAppliedBehaviors()
			resp.Finalize(start)

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

	// Match upstreams based on request path
	matchedUpstreams := s.matchUpstreamsForPath(r.URL.Path)

	// If no upstreams match, return 404
	if len(matchedUpstreams) == 0 {
		resp.Code = 404
		resp.Body = fmt.Sprintf("No upstream matches path: %s", r.URL.Path)
		resp.Finalize(start)

		s.telemetry.RecordBehavior("path_not_found")
		s.sendResponse(w, resp, 404, span, start)
		return
	}

	// Call matched upstreams
	resp.UpstreamCalls = s.callMatchedUpstreams(ctx, matchedUpstreams, r.URL.Path, behaviorStr)

	// Set success response
	resp.Code = 200
	resp.Body = fmt.Sprintf("Hello from %s (HTTP)", s.config.Name)
	resp.Finalize(start)

	s.sendResponse(w, resp, 200, span, start)
}

// resultToUpstreamCall converts a client.Result to service.Response (for upstream calls)
func (s *Server) resultToUpstreamCall(result client.Result) service.Response {
	call := service.Response{
		Service: &service.ServiceInfo{
			Name:     result.Name,
			Protocol: result.Protocol,
		},
		URL:              result.URL,
		Code:             result.Code,
		Duration:         result.Duration.String(),
		Error:            result.Error,
		BehaviorsApplied: result.BehaviorsApplied,
	}

	// Convert nested calls
	if len(result.UpstreamCalls) > 0 {
		call.UpstreamCalls = make([]service.Response, len(result.UpstreamCalls))
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
func (s *Server) callMatchedUpstreams(ctx context.Context, upstreams map[string]*service.UpstreamConfig, requestPath string, behaviorStr string) []service.Response {
	var calls []service.Response

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

		// Convert to service.Response (upstream call) and record metrics
		call := s.resultToUpstreamCall(result)
		s.telemetry.RecordUpstreamCall(name, call.Code, result.Duration)

		calls = append(calls, call)
	}

	return calls
}

// sendResponse sends the JSON response
func (s *Server) sendResponse(w http.ResponseWriter, resp *service.Response, statusCode int, span trace.Span, start time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.telemetry.Logger.Error("Failed to encode response", zap.Error(err))
		span.RecordError(err)
	}

	// Record metrics
	duration := time.Since(start)
	s.telemetry.RecordRequest("http", "GET", statusCode, duration)

	// Log request
	s.telemetry.Logger.Info("request_completed",
		zap.Int("status", statusCode),
		zap.Duration("duration", duration),
		zap.String("trace_id", resp.TraceID),
		zap.Int("upstream_calls", len(resp.UpstreamCalls)),
	)

	if statusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}
}
