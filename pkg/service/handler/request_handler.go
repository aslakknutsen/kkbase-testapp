package handler

import (
	"context"
	"fmt"
	"math/rand"
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

// ProcessResult contains the result of processing a request
type ProcessResult struct {
	Response         *pb.ServiceResponse // Non-nil on early exit
	BehaviorsApplied string              // Effective behaviors applied (includes defaults)
	EarlyExit        bool                // True if should return immediately
}

// ProcessRequest handles the complete request lifecycle
// Returns ProcessResult with response on early exit, otherwise just BehaviorsApplied
func (h *RequestHandler) ProcessRequest(reqCtx *RequestContext, protocol string) (*ProcessResult, error) {
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
			return nil, fmt.Errorf("execute behavior: %w", err)
		}

		behaviorsApplied = executor.String()

		// Check for early exit
		if result != nil && result.ShouldReturn {
			// Record behavior metric
			h.telemetry.RecordBehavior(result.BehaviorType)

			// Build and return error response
			resp := h.buildResponse(reqCtx, protocol, result.StatusCode, result.ErrorMessage, behaviorsApplied, nil)
			return &ProcessResult{
				Response:         resp,
				BehaviorsApplied: behaviorsApplied,
				EarlyExit:        true,
			}, nil
		}

		// Record applied behaviors
		if behaviorsApplied != "" {
			h.telemetry.RecordBehavior(behaviorsApplied)
		}
	}

	// No early exit - return behaviors applied for use in success response
	return &ProcessResult{
		BehaviorsApplied: behaviorsApplied,
		EarlyExit:        false,
	}, nil
}

// CallUpstreams calls upstream services and returns the calls
// This is called by the server after ProcessRequest if there's no early exit
// For gRPC (matchedUpstreams == nil), applies weighted selection if groups are configured
// Parameters:
//   - effectiveBehaviorStr: used for routing decisions (includes defaults like upstreamWeights)
//   - propagateBehaviorStr: passed to downstream services (external behavior only, not defaults)
func (h *RequestHandler) CallUpstreams(ctx context.Context, effectiveBehaviorStr string, propagateBehaviorStr string, matchedUpstreams []*service.UpstreamConfig) ([]*pb.UpstreamCall, error) {
	var calls []*pb.UpstreamCall

	// If no upstreams configured, return empty
	if len(h.config.Upstreams) == 0 {
		return calls, nil
	}

	// Determine which upstreams to call
	upstreamsToCall := matchedUpstreams
	if upstreamsToCall == nil {
		// No matched upstreams provided (gRPC case) - apply weighted selection to all upstreams
		// Uses effective behavior (includes defaults) for routing decisions
		upstreamsToCall = h.applyWeightedSelectionForGRPC(effectiveBehaviorStr)
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

		// Use shared caller - propagate external behavior only (not defaults)
		// Each downstream service will apply its own defaults if no behavior targets it
		result := h.caller.Call(ctx, name, upstreamWithPath, propagateBehaviorStr)

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

// applyWeightedSelectionForGRPC applies weighted selection and probability filtering for gRPC
// - Groups: select one per group based on weights
// - Ungrouped with Probability: include based on probability roll
// - Ungrouped without Probability: always include
func (h *RequestHandler) applyWeightedSelectionForGRPC(behaviorStr string) []*service.UpstreamConfig {
	upstreams := h.config.Upstreams

	// Extract weights from behavior
	var weights map[string]int
	if behaviorStr != "" {
		if b, err := behavior.Parse(behaviorStr); err == nil && b.UpstreamWeights != nil {
			weights = b.UpstreamWeights.Weights
		}
	}

	// Check if any upstreams have groups or probability
	hasGroupsOrProbability := false
	for _, u := range upstreams {
		if u.Group != "" || u.Probability > 0 {
			hasGroupsOrProbability = true
			break
		}
	}

	// If no groups or probability, return all upstreams
	if !hasGroupsOrProbability {
		return upstreams
	}

	// Group upstreams by their Group field
	groups := make(map[string][]*service.UpstreamConfig)
	var ungrouped []*service.UpstreamConfig

	for _, u := range upstreams {
		if u.Group == "" {
			ungrouped = append(ungrouped, u)
		} else {
			groups[u.Group] = append(groups[u.Group], u)
		}
	}

	// Process ungrouped upstreams - apply probability filtering
	var result []*service.UpstreamConfig
	for _, u := range ungrouped {
		if u.Probability > 0 {
			// Roll probability to decide if included
			if rand.Float64() < u.Probability {
				result = append(result, u)
			}
		} else {
			// No probability set = always included
			result = append(result, u)
		}
	}

	// For each group, select one upstream based on weights
	for _, groupUpstreams := range groups {
		selected := selectWeightedUpstream(groupUpstreams, weights)
		if selected != nil {
			result = append(result, selected)
		}
	}

	return result
}

// selectWeightedUpstream selects one upstream from the group based on weights
func selectWeightedUpstream(upstreams []*service.UpstreamConfig, weights map[string]int) *service.UpstreamConfig {
	if len(upstreams) == 0 {
		return nil
	}
	if len(upstreams) == 1 {
		return upstreams[0]
	}

	// Calculate effective weights
	effectiveWeights := make([]int, len(upstreams))
	totalExplicit := 0
	explicitCount := 0

	for i, u := range upstreams {
		if w, ok := weights[u.Name]; ok && w > 0 {
			effectiveWeights[i] = w
			totalExplicit += w
			explicitCount++
		}
	}

	// Distribute remaining weight equally among unspecified upstreams
	unspecifiedCount := len(upstreams) - explicitCount
	if unspecifiedCount > 0 {
		remaining := 100 - totalExplicit
		if remaining < 0 {
			remaining = 0
		}
		perUnspecified := remaining / unspecifiedCount

		for i, u := range upstreams {
			if _, ok := weights[u.Name]; !ok {
				effectiveWeights[i] = perUnspecified
			}
		}
	}

	// If no weights specified at all, use equal distribution
	if explicitCount == 0 {
		for i := range effectiveWeights {
			effectiveWeights[i] = 100 / len(upstreams)
		}
	}

	// Calculate total weight
	totalWeight := 0
	for _, w := range effectiveWeights {
		totalWeight += w
	}

	if totalWeight <= 0 {
		// Fallback: pick first
		return upstreams[0]
	}

	// Random selection based on weights
	r := randomInt(totalWeight)
	cumulative := 0
	for i, w := range effectiveWeights {
		cumulative += w
		if r < cumulative {
			return upstreams[i]
		}
	}

	// Fallback
	return upstreams[len(upstreams)-1]
}

// randomInt returns a random integer in [0, n)
func randomInt(n int) int {
	return rand.Intn(n)
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
