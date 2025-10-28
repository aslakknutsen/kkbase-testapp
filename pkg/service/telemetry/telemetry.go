package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	RequestsTotal        *prometheus.CounterVec
	RequestDuration      *prometheus.HistogramVec
	UpstreamCallsTotal   *prometheus.CounterVec
	UpstreamDuration     *prometheus.HistogramVec
	ActiveRequests       *prometheus.GaugeVec
	BehaviorAppliedTotal *prometheus.CounterVec
}

// InitTelemetry initializes all telemetry components
func InitTelemetry(serviceName, namespace, logLevel, otelEndpoint string) (*Telemetry, error) {
	// Initialize logger
	logger, err := initLogger(serviceName, namespace, logLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	// Initialize tracer
	tracer, err := initTracer(serviceName, namespace, otelEndpoint)
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
func initTracer(serviceName, namespace, endpoint string) (trace.Tracer, error) {
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

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceNamespace(namespace),
		),
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
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "testservice_requests_total",
				Help: "Total number of requests",
			},
			[]string{"service", "method", "status", "protocol"},
		),
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "testservice_request_duration_seconds",
				Help:    "Request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "method", "protocol"},
		),
		UpstreamCallsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "testservice_upstream_calls_total",
				Help: "Total number of upstream calls",
			},
			[]string{"service", "upstream", "status"},
		),
		UpstreamDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "testservice_upstream_duration_seconds",
				Help:    "Upstream call duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "upstream"},
		),
		ActiveRequests: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "testservice_active_requests",
				Help: "Number of active requests",
			},
			[]string{"service", "protocol"},
		),
		BehaviorAppliedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "testservice_behavior_applied_total",
				Help: "Total number of behaviors applied",
			},
			[]string{"service", "behavior_type"},
		),
	}
}

// RecordRequest records metrics for a request
func (t *Telemetry) RecordRequest(protocol, method string, statusCode int, duration time.Duration) {
	t.Metrics.RequestsTotal.WithLabelValues(
		t.ServiceName,
		method,
		fmt.Sprintf("%d", statusCode),
		protocol,
	).Inc()

	t.Metrics.RequestDuration.WithLabelValues(
		t.ServiceName,
		method,
		protocol,
	).Observe(duration.Seconds())
}

// RecordUpstreamCall records metrics for an upstream call
func (t *Telemetry) RecordUpstreamCall(upstream string, statusCode int, duration time.Duration) {
	t.Metrics.UpstreamCallsTotal.WithLabelValues(
		t.ServiceName,
		upstream,
		fmt.Sprintf("%d", statusCode),
	).Inc()

	t.Metrics.UpstreamDuration.WithLabelValues(
		t.ServiceName,
		upstream,
	).Observe(duration.Seconds())
}

// RecordBehavior records when a behavior is applied
func (t *Telemetry) RecordBehavior(behaviorType string) {
	t.Metrics.BehaviorAppliedTotal.WithLabelValues(
		t.ServiceName,
		behaviorType,
	).Inc()
}

// IncActiveRequests increments active request counter
func (t *Telemetry) IncActiveRequests(protocol string) {
	t.Metrics.ActiveRequests.WithLabelValues(t.ServiceName, protocol).Inc()
}

// DecActiveRequests decrements active request counter
func (t *Telemetry) DecActiveRequests(protocol string) {
	t.Metrics.ActiveRequests.WithLabelValues(t.ServiceName, protocol).Dec()
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
