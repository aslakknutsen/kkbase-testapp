package grpc

import (
	"context"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/metadata"
)

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

// ExtractTraceContext extracts trace context from gRPC metadata
func ExtractTraceContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	propagator := otel.GetTextMapPropagator()
	return propagator.Extract(ctx, metadataCarrier{md: &md})
}

// InjectTraceContext injects trace context into gRPC metadata
func InjectTraceContext(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, metadataCarrier{md: &md})

	return metadata.NewOutgoingContext(ctx, md)
}
