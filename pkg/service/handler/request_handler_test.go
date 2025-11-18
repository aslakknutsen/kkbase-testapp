package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/client"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/telemetry"
	pb "github.com/aslakknutsen/kkbase/testapp/proto/testservice"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func createTestConfig() *service.Config {
	return &service.Config{
		Name:            "test-service",
		Version:         "1.0.0",
		Namespace:       "test-ns",
		PodName:         "test-pod",
		NodeName:        "test-node",
		HTTPPort:        8080,
		GRPCPort:        9090,
		MetricsPort:     9091,
		DefaultBehavior: "",
		Upstreams:       make(map[string]*service.UpstreamConfig),
	}
}

func createTestTelemetry() *telemetry.Telemetry {
	logger, _ := zap.NewDevelopment()
	
	// Initialize metrics with nil values (tests don't need real metrics)
	metrics := &telemetry.Metrics{
		HTTPServerRequestsTotal:   nil,
		HTTPServerRequestDuration: nil,
		HTTPServerActiveRequests:  nil,
		HTTPClientRequestsTotal:   nil,
		HTTPClientRequestDuration: nil,
		HTTPClientActiveRequests:  nil,
		BehaviorAppliedTotal:      nil,
	}
	
	// Use a no-op tracer for tests
	tracer := otel.Tracer("test-service")
	
	return &telemetry.Telemetry{
		Logger:      logger,
		Tracer:      tracer,
		ServiceName: "test-service",
		Namespace:   "test-ns",
		Metrics:     metrics,
	}
}

func TestProcessRequest_NoBehavior(t *testing.T) {
	cfg := createTestConfig()
	tel := createTestTelemetry()
	caller := client.NewCaller(tel)
	handler := NewRequestHandler(cfg, caller, tel)

	reqCtx := &RequestContext{
		Ctx:         context.Background(),
		StartTime:   time.Now(),
		TraceID:     "trace123",
		SpanID:      "span456",
		BehaviorStr: "",
		Protocol:    "http",
	}

	resp, earlyExit, err := handler.ProcessRequest(reqCtx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if earlyExit {
		t.Error("Expected no early exit for no behavior")
	}
	if resp != nil {
		t.Errorf("Expected nil response (no early exit), got %+v", resp)
	}
}

func TestProcessRequest_LatencyBehavior(t *testing.T) {
	cfg := createTestConfig()
	tel := createTestTelemetry()
	caller := client.NewCaller(tel)
	handler := NewRequestHandler(cfg, caller, tel)

	reqCtx := &RequestContext{
		Ctx:         context.Background(),
		StartTime:   time.Now(),
		TraceID:     "trace123",
		SpanID:      "span456",
		BehaviorStr: "latency=50ms",
		Protocol:    "http",
	}

	start := time.Now()
	resp, earlyExit, err := handler.ProcessRequest(reqCtx)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if earlyExit {
		t.Error("Expected no early exit for latency behavior")
	}
	if resp != nil {
		t.Errorf("Expected nil response (no early exit), got %+v", resp)
	}
	if duration < 50*time.Millisecond {
		t.Errorf("Expected at least 50ms latency, got %v", duration)
	}
}

func TestProcessRequest_ErrorBehavior(t *testing.T) {
	cfg := createTestConfig()
	tel := createTestTelemetry()
	caller := client.NewCaller(tel)
	handler := NewRequestHandler(cfg, caller, tel)

	reqCtx := &RequestContext{
		Ctx:         context.Background(),
		StartTime:   time.Now(),
		TraceID:     "trace123",
		SpanID:      "span456",
		BehaviorStr: "error=503",
		Protocol:    "http",
	}

	resp, earlyExit, err := handler.ProcessRequest(reqCtx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !earlyExit {
		t.Error("Expected early exit for error behavior")
	}
	if resp == nil {
		t.Fatal("Expected response for error behavior")
	}
	if resp.Code != 503 {
		t.Errorf("Expected status code 503, got %d", resp.Code)
	}
	if resp.BehaviorsApplied == "" {
		t.Error("Expected BehaviorsApplied to be set")
	}
}

func TestProcessRequest_DiskBehaviorFailure(t *testing.T) {
	cfg := createTestConfig()
	tel := createTestTelemetry()
	caller := client.NewCaller(tel)
	handler := NewRequestHandler(cfg, caller, tel)

	reqCtx := &RequestContext{
		Ctx:         context.Background(),
		StartTime:   time.Now(),
		TraceID:     "trace123",
		SpanID:      "span456",
		BehaviorStr: "disk=fill:1Ki:/nonexistent/path:1s",
		Protocol:    "http",
	}

	resp, earlyExit, err := handler.ProcessRequest(reqCtx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !earlyExit {
		t.Error("Expected early exit for disk failure")
	}
	if resp == nil {
		t.Fatal("Expected response for disk failure")
	}
	if resp.Code != 507 {
		t.Errorf("Expected status code 507, got %d", resp.Code)
	}
}

func TestProcessRequest_ErrorIfFile(t *testing.T) {
	cfg := createTestConfig()
	tel := createTestTelemetry()
	caller := client.NewCaller(tel)
	handler := NewRequestHandler(cfg, caller, tel)

	// Create temp file with invalid content
	tmpFile := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(tmpFile, []byte("invalid_key"), 0644); err != nil {
		t.Fatal(err)
	}

	reqCtx := &RequestContext{
		Ctx:         context.Background(),
		StartTime:   time.Now(),
		TraceID:     "trace123",
		SpanID:      "span456",
		BehaviorStr: fmt.Sprintf("error-if-file=%s:invalid_key:401", tmpFile),
		Protocol:    "http",
	}

	resp, earlyExit, err := handler.ProcessRequest(reqCtx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !earlyExit {
		t.Error("Expected early exit for error-if-file")
	}
	if resp == nil {
		t.Fatal("Expected response for error-if-file")
	}
	if resp.Code != 401 {
		t.Errorf("Expected status code 401, got %d", resp.Code)
	}
}

func TestProcessRequest_TargetedBehavior(t *testing.T) {
	cfg := createTestConfig()
	cfg.Name = "service-a"
	tel := createTestTelemetry()
	caller := client.NewCaller(tel)
	handler := NewRequestHandler(cfg, caller, tel)

	// Behavior targeted at service-b, should not apply to service-a
	reqCtx := &RequestContext{
		Ctx:         context.Background(),
		StartTime:   time.Now(),
		TraceID:     "trace123",
		SpanID:      "span456",
		BehaviorStr: "service-b:error=500",
		Protocol:    "http",
	}

	resp, earlyExit, err := handler.ProcessRequest(reqCtx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if earlyExit {
		t.Error("Expected no early exit when behavior targets different service")
	}
	if resp != nil {
		t.Errorf("Expected nil response (no early exit), got %+v", resp)
	}
}

func TestCallUpstreams_NoUpstreams(t *testing.T) {
	cfg := createTestConfig()
	tel := createTestTelemetry()
	caller := client.NewCaller(tel)
	handler := NewRequestHandler(cfg, caller, tel)

	calls, err := handler.CallUpstreams(context.Background(), "", nil)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("Expected 0 calls, got %d", len(calls))
	}
}

func TestCallUpstreams_WithMatchedUpstreams(t *testing.T) {
	cfg := createTestConfig()
	cfg.Upstreams["service-b"] = &service.UpstreamConfig{
		Name:     "service-b",
		URL:      "http://localhost:8081",
		Protocol: "http",
	}
	cfg.Upstreams["service-c"] = &service.UpstreamConfig{
		Name:     "service-c",
		URL:      "http://localhost:8082",
		Protocol: "http",
	}
	
	tel := createTestTelemetry()
	caller := client.NewCaller(tel)
	handler := NewRequestHandler(cfg, caller, tel)

	// Only service-b is matched
	matchedUpstreams := map[string]*service.UpstreamConfig{
		"service-b": cfg.Upstreams["service-b"],
	}

	calls, err := handler.CallUpstreams(context.Background(), "", matchedUpstreams)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// Should only call matched upstream (service-b), even though service-c is configured
	// However, both will fail to connect, so we expect 1 call with error
	if len(calls) != 1 {
		t.Errorf("Expected 1 call, got %d", len(calls))
	}
}

func TestCheckUpstreamFailures(t *testing.T) {
	cfg := createTestConfig()
	tel := createTestTelemetry()
	caller := client.NewCaller(tel)
	handler := NewRequestHandler(cfg, caller, tel)

	tests := []struct {
		name     string
		calls    []*pb.UpstreamCall
		expected bool
	}{
		{
			name:     "no calls",
			calls:    nil,
			expected: false,
		},
		{
			name: "all success",
			calls: []*pb.UpstreamCall{
				{Name: "service-a", Code: 200},
				{Name: "service-b", Code: 201},
			},
			expected: false,
		},
		{
			name: "one failure",
			calls: []*pb.UpstreamCall{
				{Name: "service-a", Code: 200},
				{Name: "service-b", Code: 500},
			},
			expected: true,
		},
		{
			name: "connection error (code 0) is not failure",
			calls: []*pb.UpstreamCall{
				{Name: "service-a", Code: 0, Error: "connection refused"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failed := handler.CheckUpstreamFailures(tt.calls)
			if (failed != nil) != tt.expected {
				t.Errorf("Expected failure=%v, got %v", tt.expected, failed != nil)
			}
		})
	}
}

func TestBuildSuccessResponse(t *testing.T) {
	cfg := createTestConfig()
	tel := createTestTelemetry()
	caller := client.NewCaller(tel)
	handler := NewRequestHandler(cfg, caller, tel)

	reqCtx := &RequestContext{
		Ctx:       context.Background(),
		StartTime: time.Now(),
		TraceID:   "trace123",
		SpanID:    "span456",
		Protocol:  "http",
	}

	resp := handler.BuildSuccessResponse(reqCtx, "latency=100ms", nil)

	if resp.Code != 200 {
		t.Errorf("Expected code 200, got %d", resp.Code)
	}
	if resp.Service.Name != "test-service" {
		t.Errorf("Expected service name 'test-service', got %s", resp.Service.Name)
	}
	if resp.BehaviorsApplied != "latency=100ms" {
		t.Errorf("Expected behaviors applied 'latency=100ms', got %s", resp.BehaviorsApplied)
	}
	if resp.TraceId != "trace123" {
		t.Errorf("Expected trace ID 'trace123', got %s", resp.TraceId)
	}
}

func TestBuildUpstreamErrorResponse(t *testing.T) {
	cfg := createTestConfig()
	tel := createTestTelemetry()
	caller := client.NewCaller(tel)
	handler := NewRequestHandler(cfg, caller, tel)

	reqCtx := &RequestContext{
		Ctx:       context.Background(),
		StartTime: time.Now(),
		TraceID:   "trace123",
		SpanID:    "span456",
		Protocol:  "grpc",
	}

	failedCall := &pb.UpstreamCall{
		Name: "service-b",
		Code: 500,
	}

	resp := handler.BuildUpstreamErrorResponse(reqCtx, failedCall, "", nil)

	if resp.Code != 502 {
		t.Errorf("Expected code 502, got %d", resp.Code)
	}
	if resp.Body == "" {
		t.Error("Expected error message in body")
	}
}

