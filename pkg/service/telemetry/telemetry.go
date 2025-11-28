package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
)

// Telemetry holds all observability components
type Telemetry struct {
	Logger      *zap.Logger
	Tracer      trace.Tracer
	Metrics     *Metrics
	ServiceName string
	Namespace   string
}

// Metrics holds Prometheus metrics
type Metrics struct {
	// HTTP Server metrics (RED method)
	HTTPServerRequestsTotal   *prometheus.CounterVec
	HTTPServerRequestDuration *prometheus.HistogramVec
	HTTPServerActiveRequests  *prometheus.GaugeVec

	// HTTP Client metrics (Dependency monitoring)
	HTTPClientRequestsTotal   *prometheus.CounterVec
	HTTPClientRequestDuration *prometheus.HistogramVec
	HTTPClientActiveRequests  *prometheus.GaugeVec

	// gRPC Server metrics (application-level, supplements grpc_prometheus)
	GRPCServerRequestsTotal   *prometheus.CounterVec
	GRPCServerRequestDuration *prometheus.HistogramVec

	// Custom behavior metrics
	BehaviorAppliedTotal *prometheus.CounterVec
}

// InitTelemetry initializes all telemetry components
func InitTelemetry(serviceName, namespace, logLevel, otelEndpoint string, cfg *service.Config) (*Telemetry, error) {
	// Initialize logger
	logger, err := initLogger(serviceName, namespace, logLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	// Initialize tracer
	tracer, err := initTracer(serviceName, namespace, otelEndpoint, cfg)
	if err != nil {
		logger.Warn("Failed to init tracer, continuing without tracing", zap.Error(err))
		tracer = otel.Tracer(serviceName)
	}

	// Initialize metrics
	metrics := initMetrics()

	return &Telemetry{
		Logger:      logger,
		Tracer:      tracer,
		Metrics:     metrics,
		ServiceName: serviceName,
		Namespace:   namespace,
	}, nil
}

// initLogger creates a structured logger
func initLogger(serviceName, namespace, logLevel string) (*zap.Logger, error) {
	level := zapcore.InfoLevel
	switch logLevel {
	case "debug":
		level = zapcore.DebugLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	// Add default fields
	logger = logger.With(
		zap.String("service", serviceName),
		zap.String("namespace", namespace),
	)

	return logger, nil
}

// initTracer creates an OTEL tracer
func initTracer(serviceName, namespace, endpoint string, cfg *service.Config) (trace.Tracer, error) {
	if endpoint == "" {
		// No endpoint configured, return noop tracer
		return otel.Tracer(serviceName), nil
	}

	ctx := context.Background()

	// Create OTLP exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create resource with all available K8s attributes
	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
		semconv.ServiceNamespace(namespace),
	}

	// Service identity
	if cfg.Version != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.Version))
	}
	if cfg.PodName != "" {
		attrs = append(attrs, semconv.ServiceInstanceID(cfg.PodName))
	}

	// K8s metadata
	if cfg.Namespace != "" {
		attrs = append(attrs, semconv.K8SNamespaceName(cfg.Namespace))
	}
	if cfg.PodName != "" {
		attrs = append(attrs, semconv.K8SPodName(cfg.PodName))
	}
	if podUID := os.Getenv("POD_UID"); podUID != "" {
		attrs = append(attrs, semconv.K8SPodUID(podUID))
	}
	if cfg.NodeName != "" {
		attrs = append(attrs, semconv.K8SNodeName(cfg.NodeName))
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Tracer(serviceName), nil
}

// initMetrics creates Prometheus metrics
func initMetrics() *Metrics {
	return &Metrics{
		// HTTP Server metrics (RED method)
		HTTPServerRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_server_requests_total",
				Help: "Total number of HTTP server requests",
			},
			[]string{"method", "path", "status_code"},
		),
		HTTPServerRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_server_request_duration_seconds",
				Help:    "HTTP server request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status_code"},
		),
		HTTPServerActiveRequests: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "http_server_active_requests",
				Help: "Number of active HTTP server requests",
			},
			[]string{"method", "path"},
		),

		// HTTP Client metrics (Dependency monitoring)
		HTTPClientRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_client_requests_total",
				Help: "Total number of HTTP client requests",
			},
			[]string{"method", "destination_service", "status_code"},
		),
		HTTPClientRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_client_request_duration_seconds",
				Help:    "HTTP client request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "destination_service", "status_code"},
		),
		HTTPClientActiveRequests: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "http_client_active_requests",
				Help: "Number of active HTTP client requests",
			},
			[]string{"destination_service"},
		),

		// gRPC Server metrics (application-level, captures actual response codes)
		GRPCServerRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "grpc_server_requests_total",
				Help: "Total number of gRPC server requests by response code",
			},
			[]string{"method", "response_code"},
		),
		GRPCServerRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "grpc_server_request_duration_seconds",
				Help:    "gRPC server request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "response_code"},
		),

		// Custom behavior metrics
		BehaviorAppliedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "testservice_behavior_applied_total",
				Help: "Total number of behaviors applied",
			},
			[]string{"service", "behavior_type"},
		),
	}
}

// RecordRequest records metrics for an HTTP server request
func (t *Telemetry) RecordRequest(method, path string, statusCode int, duration time.Duration) {
	if t.Metrics == nil {
		return
	}
	
	statusCodeStr := fmt.Sprintf("%d", statusCode)
	
	if t.Metrics.HTTPServerRequestsTotal != nil {
		t.Metrics.HTTPServerRequestsTotal.WithLabelValues(
			method,
			path,
			statusCodeStr,
		).Inc()
	}

	if t.Metrics.HTTPServerRequestDuration != nil {
		t.Metrics.HTTPServerRequestDuration.WithLabelValues(
			method,
			path,
			statusCodeStr,
		).Observe(duration.Seconds())
	}
}

// RecordGRPCRequest records metrics for a gRPC server request (application-level)
func (t *Telemetry) RecordGRPCRequest(method string, responseCode int, duration time.Duration) {
	if t.Metrics == nil {
		return
	}

	responseCodeStr := fmt.Sprintf("%d", responseCode)

	if t.Metrics.GRPCServerRequestsTotal != nil {
		t.Metrics.GRPCServerRequestsTotal.WithLabelValues(
			method,
			responseCodeStr,
		).Inc()
	}

	if t.Metrics.GRPCServerRequestDuration != nil {
		t.Metrics.GRPCServerRequestDuration.WithLabelValues(
			method,
			responseCodeStr,
		).Observe(duration.Seconds())
	}
}

// RecordUpstreamCall records metrics for an HTTP client (upstream) call
func (t *Telemetry) RecordUpstreamCall(method, destinationService string, statusCode int, duration time.Duration) {
	if t.Metrics == nil {
		return
	}
	
	statusCodeStr := fmt.Sprintf("%d", statusCode)
	
	if t.Metrics.HTTPClientRequestsTotal != nil {
		t.Metrics.HTTPClientRequestsTotal.WithLabelValues(
			method,
			destinationService,
			statusCodeStr,
		).Inc()
	}

	if t.Metrics.HTTPClientRequestDuration != nil {
		t.Metrics.HTTPClientRequestDuration.WithLabelValues(
			method,
			destinationService,
			statusCodeStr,
		).Observe(duration.Seconds())
	}
}

// RecordBehavior records when a behavior is applied
func (t *Telemetry) RecordBehavior(behaviorType string) {
	if t.Metrics == nil || t.Metrics.BehaviorAppliedTotal == nil {
		return
	}
	t.Metrics.BehaviorAppliedTotal.WithLabelValues(
		t.ServiceName,
		behaviorType,
	).Inc()
}

// IncActiveRequests increments active HTTP server request counter
func (t *Telemetry) IncActiveRequests(method, path string) {
	if t.Metrics == nil || t.Metrics.HTTPServerActiveRequests == nil {
		return
	}
	t.Metrics.HTTPServerActiveRequests.WithLabelValues(method, path).Inc()
}

// DecActiveRequests decrements active HTTP server request counter
func (t *Telemetry) DecActiveRequests(method, path string) {
	if t.Metrics == nil || t.Metrics.HTTPServerActiveRequests == nil {
		return
	}
	t.Metrics.HTTPServerActiveRequests.WithLabelValues(method, path).Dec()
}

// IncActiveClientRequests increments active HTTP client request counter
func (t *Telemetry) IncActiveClientRequests(destinationService string) {
	if t.Metrics == nil || t.Metrics.HTTPClientActiveRequests == nil {
		return
	}
	t.Metrics.HTTPClientActiveRequests.WithLabelValues(destinationService).Inc()
}

// DecActiveClientRequests decrements active HTTP client request counter
func (t *Telemetry) DecActiveClientRequests(destinationService string) {
	if t.Metrics == nil || t.Metrics.HTTPClientActiveRequests == nil {
		return
	}
	t.Metrics.HTTPClientActiveRequests.WithLabelValues(destinationService).Dec()
}

// StartSpan starts a new span with common attributes
func (t *Telemetry) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return t.Tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// StartServerSpan starts a SERVER span for incoming requests
func (t *Telemetry) StartServerSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attrs...),
	}
	return t.Tracer.Start(ctx, name, opts...)
}

// StartClientSpan starts a CLIENT span for outgoing calls
func (t *Telemetry) StartClientSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	}
	return t.Tracer.Start(ctx, name, opts...)
}
