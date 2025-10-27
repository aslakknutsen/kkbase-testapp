package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kagenti/kkbase/testapp/pkg/service"
	pb "github.com/kagenti/kkbase/testapp/proto/testservice"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Result represents the standardized result of an upstream call
type Result struct {
	Name             string
	URL              string
	Protocol         string
	Duration         time.Duration
	Code             int
	Error            string
	BehaviorsApplied []string
	UpstreamCalls    []Result
}

// Caller handles upstream calls to both HTTP and gRPC services
type Caller struct {
	httpClient *http.Client
	tracer     trace.Tracer
}

// NewCaller creates a new upstream caller
func NewCaller() *Caller {
	return &Caller{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		tracer: otel.Tracer("testservice.client"),
	}
}

// Call makes an upstream call and returns a standardized result
// behaviorStr is propagated to the upstream service to control its behavior
func (c *Caller) Call(ctx context.Context, name string, upstream *service.UpstreamConfig, behaviorStr string) Result {
	start := time.Now()

	// Start span for upstream call
	ctx, span := c.tracer.Start(ctx, fmt.Sprintf("upstream.%s", name),
		trace.WithAttributes(
			attribute.String("upstream.name", name),
			attribute.String("upstream.protocol", upstream.Protocol),
			attribute.String("upstream.url", upstream.URL),
		),
	)
	defer span.End()

	result := Result{
		Name:     name,
		URL:      upstream.URL,
		Protocol: upstream.Protocol,
	}

	// Route based on protocol
	if upstream.Protocol == "grpc" {
		result = c.callGRPC(ctx, name, upstream, behaviorStr, span, start)
	} else {
		result = c.callHTTP(ctx, name, upstream, behaviorStr, span, start)
	}

	result.Duration = time.Since(start)

	// Update span status
	if result.Error != "" {
		span.RecordError(fmt.Errorf(result.Error))
		span.SetStatus(codes.Error, result.Error)
	} else if result.Code >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("Status %d", result.Code))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(attribute.Int("http.status_code", result.Code))

	return result
}

// callHTTP makes an HTTP call to an upstream service
func (c *Caller) callHTTP(ctx context.Context, name string, upstream *service.UpstreamConfig, behaviorStr string, span trace.Span, start time.Time) Result {
	result := Result{
		Name:     name,
		URL:      upstream.URL,
		Protocol: "http",
	}

	// Ensure URL has http:// prefix
	url := upstream.URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + strings.TrimPrefix(url, "http://")
	}

	// Add behavior as query parameter to propagate to upstream
	if behaviorStr != "" {
		if strings.Contains(url, "?") {
			url = url + "&behavior=" + behaviorStr
		} else {
			url = url + "?behavior=" + behaviorStr
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		result.Error = err.Error()
		result.Code = 0
		return result
	}

	// Propagate trace context via HTTP headers
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

	// Make the call
	resp, err := c.httpClient.Do(req)
	if err != nil {
		result.Error = err.Error()
		result.Code = 0
		return result
	}
	defer resp.Body.Close()

	result.Code = resp.StatusCode

	// Try to parse response for nested upstream calls and behaviors
	// Use the existing service.Response type which is already recursive
	var httpResp struct {
		BehaviorsApplied []string           `json:"behaviors_applied,omitempty"`
		UpstreamCalls    []service.Response `json:"upstream_calls,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&httpResp); err == nil {
		result.BehaviorsApplied = httpResp.BehaviorsApplied

		// Recursive function to convert service.Response to Result
		var convertUpstreamCall func(service.Response) Result
		convertUpstreamCall = func(uc service.Response) Result {
			duration, _ := time.ParseDuration(uc.Duration)
			r := Result{
				Name:             uc.Service.Name,
				URL:              uc.URL,
				Protocol:         uc.Service.Protocol,
				Duration:         duration,
				Code:             uc.Code,
				Error:            uc.Error,
				BehaviorsApplied: uc.BehaviorsApplied,
			}
			// Recursively convert nested upstream calls
			for _, nested := range uc.UpstreamCalls {
				r.UpstreamCalls = append(r.UpstreamCalls, convertUpstreamCall(nested))
			}
			return r
		}

		for _, uc := range httpResp.UpstreamCalls {
			result.UpstreamCalls = append(result.UpstreamCalls, convertUpstreamCall(uc))
		}
	}

	return result
}

// callGRPC makes a gRPC call to an upstream service
func (c *Caller) callGRPC(ctx context.Context, name string, upstream *service.UpstreamConfig, behaviorStr string, span trace.Span, start time.Time) Result {
	result := Result{
		Name:     name,
		URL:      upstream.URL,
		Protocol: "grpc",
	}

	// Extract target from grpc://host:port URL
	target := strings.TrimPrefix(upstream.URL, "grpc://")

	// gRPC doesn't use URL paths like HTTP does, strip any trailing path
	// (e.g., "host:9090/" becomes "host:9090")
	if idx := strings.Index(target, "/"); idx != -1 {
		target = target[:idx]
	}

	// Create gRPC connection
	conn, err := grpc.Dial(target, grpc.WithInsecure())
	if err != nil {
		result.Error = err.Error()
		result.Code = 0
		return result
	}
	defer conn.Close()

	// Create client
	client := pb.NewTestServiceClient(conn)

	// Propagate trace context via gRPC metadata
	md := metadata.New(nil)
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, metadataCarrier{md: &md})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Make the call with behavior propagated
	resp, err := client.Call(ctx, &pb.CallRequest{
		Behavior: behaviorStr,
	})
	if err != nil {
		result.Error = err.Error()
		result.Code = 500 // Map gRPC error to HTTP 500
		return result
	}

	// Use the actual code from the response (could be 200, 500, etc.)
	result.Code = int(resp.Code)
	result.BehaviorsApplied = resp.BehaviorsApplied

	// Convert nested gRPC upstream calls to Result
	if len(resp.UpstreamCalls) > 0 {
		for _, uc := range resp.UpstreamCalls {
			duration, _ := time.ParseDuration(uc.Duration)
			// Recursively convert nested calls
			nestedResult := Result{
				Name:             uc.Name,
				URL:              uc.Uri,
				Protocol:         uc.Protocol,
				Duration:         duration,
				Code:             int(uc.Code),
				Error:            uc.Error,
				BehaviorsApplied: convertBehaviorsApplied(uc),
			}
			// Handle nested upstream calls recursively
			if len(uc.UpstreamCalls) > 0 {
				nestedResult.UpstreamCalls = convertUpstreamCalls(uc.UpstreamCalls)
			}
			result.UpstreamCalls = append(result.UpstreamCalls, nestedResult)
		}
	}

	return result
}

// convertUpstreamCalls recursively converts protobuf UpstreamCall to Result
func convertUpstreamCalls(pbCalls []*pb.UpstreamCall) []Result {
	results := make([]Result, 0, len(pbCalls))
	for _, uc := range pbCalls {
		duration, _ := time.ParseDuration(uc.Duration)
		result := Result{
			Name:             uc.Name,
			URL:              uc.Uri,
			Protocol:         uc.Protocol,
			Duration:         duration,
			Code:             int(uc.Code),
			Error:            uc.Error,
			BehaviorsApplied: convertBehaviorsApplied(uc),
		}
		if len(uc.UpstreamCalls) > 0 {
			result.UpstreamCalls = convertUpstreamCalls(uc.UpstreamCalls)
		}
		results = append(results, result)
	}
	return results
}

// convertBehaviorsApplied extracts behaviors_applied from protobuf UpstreamCall
func convertBehaviorsApplied(uc *pb.UpstreamCall) []string {
	if uc.BehaviorsApplied == nil {
		return []string{}
	}
	return uc.BehaviorsApplied
}

// metadataCarrier adapts metadata.MD to propagation.TextMapCarrier
type metadataCarrier struct {
	md *metadata.MD
}

func (mc metadataCarrier) Get(key string) string {
	values := mc.md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (mc metadataCarrier) Set(key, value string) {
	mc.md.Set(key, value)
}

func (mc metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(*mc.md))
	for k := range *mc.md {
		keys = append(keys, k)
	}
	return keys
}
