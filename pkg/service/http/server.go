package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	// Make upstream calls with behavior chain propagated
	upstreamParam := r.URL.Query().Get("upstream")
	if upstreamParam != "" {
		upstreamCalls := s.makeUpstreamCalls(ctx, upstreamParam, behaviorStr)
		resp.UpstreamCalls = upstreamCalls
	} else {
		// Call all configured upstreams with behavior chain
		resp.UpstreamCalls = s.callAllUpstreams(ctx, behaviorStr)
	}

	// Set success response
	resp.Code = 200
	resp.Body = fmt.Sprintf("Hello from %s (HTTP)", s.config.Name)
	resp.Finalize(start)

	s.sendResponse(w, resp, 200, span, start)
}

// makeUpstreamCalls parses and executes upstream call specifications
// Format: "service1,service2:behavior=latency=100ms"
func (s *Server) makeUpstreamCalls(ctx context.Context, upstreamSpec string, behaviorStr string) []service.UpstreamCall {
	var calls []service.UpstreamCall

	// For simplicity, just split by comma for now
	// TODO: Implement more sophisticated parsing
	return calls
}

// callAllUpstreams calls all configured upstream services with behavior propagation
func (s *Server) callAllUpstreams(ctx context.Context, behaviorStr string) []service.UpstreamCall {
	var calls []service.UpstreamCall

	for name, upstream := range s.config.Upstreams {
		// Use shared caller with behavior propagation
		result := s.caller.Call(ctx, name, upstream, behaviorStr)

		// Convert to service.UpstreamCall and record metrics
		call := s.resultToUpstreamCall(result)
		s.telemetry.RecordUpstreamCall(name, call.Code, result.Duration)

		calls = append(calls, call)
	}

	return calls
}

// resultToUpstreamCall converts a client.Result to service.UpstreamCall
func (s *Server) resultToUpstreamCall(result client.Result) service.UpstreamCall {
	call := service.UpstreamCall{
		Name:             result.Name,
		URI:              result.URI,
		Protocol:         result.Protocol,
		Code:             result.Code,
		Duration:         result.Duration.String(),
		Error:            result.Error,
		BehaviorsApplied: result.BehaviorsApplied,
	}

	// Convert nested calls
	if len(result.UpstreamCalls) > 0 {
		call.UpstreamCalls = make([]service.UpstreamCall, len(result.UpstreamCalls))
		for i, uc := range result.UpstreamCalls {
			call.UpstreamCalls[i] = s.resultToUpstreamCall(uc)
		}
	}

	return call
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
