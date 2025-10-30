package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kagenti/kkbase/testapp/pkg/service"
	"github.com/kagenti/kkbase/testapp/pkg/service/telemetry"
	pb "github.com/kagenti/kkbase/testapp/proto/testservice"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
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
	telemetry  *telemetry.Telemetry
}

// NewCaller creates a new upstream caller
func NewCaller(tel *telemetry.Telemetry) *Caller {
	return &Caller{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		telemetry: tel,
	}
}

// Call makes an upstream call and returns a standardized result
// behaviorStr is propagated to the upstream service to control its behavior
func (c *Caller) Call(ctx context.Context, name string, upstream *service.UpstreamConfig, behaviorStr string) Result {
	start := time.Now()

	// Start span for upstream call
	ctx, span := c.telemetry.StartClientSpan(ctx, fmt.Sprintf("upstream.%s", name),
		semconv.NetworkProtocolName(upstream.Protocol),
		semconv.NetworkTransportTCP,
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
		span.RecordError(fmt.Errorf("%s", result.Error))
		span.SetStatus(codes.Error, result.Error)
	} else if result.Code >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("Status %d", result.Code))
	} else {
		span.SetStatus(codes.Ok, "")
	}

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
	urlStr := upstream.URL
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "http://" + strings.TrimPrefix(urlStr, "http://")
	}

	// Add behavior as query parameter to propagate to upstream
	if behaviorStr != "" {
		if strings.Contains(urlStr, "?") {
			urlStr = urlStr + "&behavior=" + behaviorStr
		} else {
			urlStr = urlStr + "?behavior=" + behaviorStr
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		result.Error = err.Error()
		result.Code = 0
		return result
	}

	// Update span name and add HTTP-specific span attributes
	if parsedURL, err := url.Parse(urlStr); err == nil {
		// Update span name to follow HTTP semantic conventions: {method} {path}
		span.SetName(fmt.Sprintf("GET %s", parsedURL.Path))

		span.SetAttributes(
			semconv.HTTPRequestMethodOriginal("GET"),
			semconv.URLFull(urlStr),
			semconv.ServerAddress(parsedURL.Hostname()),
		)
		if port := parsedURL.Port(); port != "" {
			if p, err := strconv.Atoi(port); err == nil {
				span.SetAttributes(semconv.ServerPort(p))
			}
		}
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

	// Add response status code attributes
	span.SetAttributes(
		semconv.HTTPResponseStatusCode(resp.StatusCode),
	)
	if resp.StatusCode >= 400 {
		span.SetAttributes(semconv.ErrorTypeKey.String(fmt.Sprintf("%d", resp.StatusCode)))
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read response body: %v", err)
		return result
	}

	// Try to parse response as protobuf ServiceResponse
	var httpResp pb.ServiceResponse
	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true, // Ignore unknown fields for flexibility
	}

	if err := unmarshaler.Unmarshal(bodyBytes, &httpResp); err == nil {
		result.BehaviorsApplied = httpResp.BehaviorsApplied

		// Convert pb.UpstreamCall to Result (reuse existing converter)
		if len(httpResp.UpstreamCalls) > 0 {
			result.UpstreamCalls = convertUpstreamCalls(httpResp.UpstreamCalls)
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

	// Update span name and add gRPC-specific span attributes
	// gRPC span name must follow: $package.$service/$method
	span.SetName("testservice.TestService/Call")
	span.SetAttributes(
		semconv.RPCSystemGRPC,
		semconv.RPCService("testservice.TestService"),
		semconv.RPCMethod("Call"),
		semconv.ServerAddress(target),
	)

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

	// Add gRPC status code attributes
	span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(0)) // 0 = OK

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
