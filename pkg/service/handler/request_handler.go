package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/behavior"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/client"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/telemetry"
	pb "github.com/aslakknutsen/kkbase/testapp/proto/testservice"
	"go.uber.org/zap"
)

// RequestContext holds all the data needed to process a request
type RequestContext struct {
	Ctx         context.Context
	StartTime   time.Time
	TraceID     string
	SpanID      string
	BehaviorStr string
}

// RequestHandler encapsulates common request handling logic for both HTTP and gRPC
type RequestHandler struct {
	config    *service.Config
	caller    *client.Caller
	telemetry *telemetry.Telemetry
}

// NewRequestHandler creates a new request handler
func NewRequestHandler(cfg *service.Config, caller *client.Caller, tel *telemetry.Telemetry) *RequestHandler {
	return &RequestHandler{
		config:    cfg,
		caller:    caller,
		telemetry: tel,
	}
}

// ProcessRequest handles the complete request lifecycle
// Returns a response and a boolean indicating if it's an early exit (before upstream calls)
func (h *RequestHandler) ProcessRequest(reqCtx *RequestContext, protocol string) (*pb.ServiceResponse, bool, error) {
	// Get default behavior if not provided
	behaviorStr := reqCtx.BehaviorStr
	if behaviorStr == "" {
		behaviorStr = h.config.DefaultBehavior
	}

	// Parse behavior chain
	behaviorChain, err := behavior.ParseChain(behaviorStr)
	if err != nil {
		h.telemetry.Logger.Warn("Failed to parse behavior chain",
			zap.Error(err))
		// Continue with empty behavior chain
		behaviorChain = &behavior.BehaviorChain{}
	}

	// Extract behavior for this service
	beh := behaviorChain.ForService(h.config.Name)

	// Execute behaviors with early exit on errors
	var behaviorsApplied string
	if beh != nil {
		executor := behavior.NewExecutor(beh, reqCtx.TraceID, h.config.Name, h.telemetry.Logger)
		result, err := executor.Execute(reqCtx.Ctx)
		if err != nil {
			return nil, false, fmt.Errorf("execute behavior: %w", err)
		}

		behaviorsApplied = executor.String()

		// Check for early exit
		if result != nil && result.ShouldReturn {
			// Record behavior metric
			h.telemetry.RecordBehavior(result.BehaviorType)

			// Build and return error response
			resp := h.buildResponse(reqCtx, protocol, result.StatusCode, result.ErrorMessage, behaviorsApplied, nil)
			return resp, true, nil
		}

		// Record applied behaviors
		if behaviorsApplied != "" {
			h.telemetry.RecordBehavior(behaviorsApplied)
		}
	}

	// No early exit - return success response info
	return nil, false, nil
}

// CallUpstreams calls upstream services and returns the calls
// This is called by the server after ProcessRequest if there's no early exit
func (h *RequestHandler) CallUpstreams(ctx context.Context, behaviorStr string, matchedUpstreams []*service.UpstreamConfig) ([]*pb.UpstreamCall, error) {
	var calls []*pb.UpstreamCall

	// If no upstreams configured, return empty
	if len(h.config.Upstreams) == 0 {
		return calls, nil
	}

	// Determine which upstreams to call
	upstreamsToCall := matchedUpstreams
	if upstreamsToCall == nil {
		// No matched upstreams provided (gRPC case) - call all configured upstreams
		upstreamsToCall = h.config.Upstreams
	}

	// Call each upstream (fail-fast: stop on first failure)
	for _, upstream := range upstreamsToCall {
		name := upstream.Name
		// Build upstream config with path appended to URL (for HTTP upstreams)
		upstreamWithPath := upstream
		if upstream.Protocol == "http" && upstream.Path != "" {
			upstreamWithPath = &service.UpstreamConfig{
				Name:     upstream.Name,
				URL:      upstream.URL + upstream.Path,
				Protocol: upstream.Protocol,
				Match:    upstream.Match,
				Path:     upstream.Path,
			}
		} else if upstream.Protocol == "http" && upstream.Path == "" {
			// Default to "/" for HTTP upstreams without explicit path
			upstreamWithPath = &service.UpstreamConfig{
				Name:     upstream.Name,
				URL:      upstream.URL + "/",
				Protocol: upstream.Protocol,
				Match:    upstream.Match,
				Path:     "/",
			}
		}

		// Use shared caller with behavior propagation
		result := h.caller.Call(ctx, name, upstreamWithPath, behaviorStr)

		// Convert to pb.UpstreamCall and record metrics
		call := h.ResultToUpstreamCall(result)

		// Determine method for metrics
		method := "Call"
		if result.Protocol == "http" {
			method = "GET"
		}
		h.telemetry.RecordUpstreamCall(method, name, int(call.Code), result.Duration)

		calls = append(calls, call)

		// Fail-fast: stop on first failure (non-2xx response or error)
		if call.Code >= 300 || call.Error != "" {
			break
		}
	}

	return calls, nil
}

// BuildSuccessResponse builds a successful response
func (h *RequestHandler) BuildSuccessResponse(reqCtx *RequestContext, protocol string, behaviorsApplied string, upstreamCalls []*pb.UpstreamCall) *pb.ServiceResponse {
	body := "All ok"
	return h.buildResponse(reqCtx, protocol, 200, body, behaviorsApplied, upstreamCalls)
}

// BuildUpstreamErrorResponse builds a response for upstream failures
func (h *RequestHandler) BuildUpstreamErrorResponse(reqCtx *RequestContext, protocol string, failedCall *pb.UpstreamCall, behaviorsApplied string, upstreamCalls []*pb.UpstreamCall) *pb.ServiceResponse {
	body := fmt.Sprintf("Upstream service failure: %s returned %d", failedCall.Name, failedCall.Code)
	return h.buildResponse(reqCtx, protocol, 502, body, behaviorsApplied, upstreamCalls)
}

// CheckUpstreamFailures checks if any upstream returned non-2xx (excluding connection errors where Code=0)
func (h *RequestHandler) CheckUpstreamFailures(upstreamCalls []*pb.UpstreamCall) *pb.UpstreamCall {
	for _, call := range upstreamCalls {
		if call.Code >= 300 {
			return call
		}
	}
	return nil
}

// buildResponse constructs a response
func (h *RequestHandler) buildResponse(reqCtx *RequestContext, protocol string, code int, body string, behaviorsApplied string, upstreamCalls []*pb.UpstreamCall) *pb.ServiceResponse {
	now := time.Now()

	return &pb.ServiceResponse{
		Service: &pb.ServiceInfo{
			Name:      h.config.Name,
			Version:   h.config.Version,
			Namespace: h.config.Namespace,
			Pod:       h.config.PodName,
			Node:      h.config.NodeName,
			Protocol:  protocol,
		},
		StartTime:        reqCtx.StartTime.Format(time.RFC3339Nano),
		EndTime:          now.Format(time.RFC3339Nano),
		Duration:         now.Sub(reqCtx.StartTime).String(),
		Code:             int32(code),
		Body:             body,
		BehaviorsApplied: behaviorsApplied,
		TraceId:          reqCtx.TraceID,
		SpanId:           reqCtx.SpanID,
		UpstreamCalls:    upstreamCalls,
	}
}

// ResultToUpstreamCall converts a client.Result to pb.UpstreamCall
func (h *RequestHandler) ResultToUpstreamCall(result client.Result) *pb.UpstreamCall {
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
			call.UpstreamCalls[i] = h.ResultToUpstreamCall(uc)
		}
	}

	return call
}
