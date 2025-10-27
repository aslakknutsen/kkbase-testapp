package service

import (
	"os"
	"testing"
)

func TestLoadConfigFromEnv_UpstreamsParsing(t *testing.T) {
	tests := []struct {
		name              string
		upstreamsEnv      string
		expectedUpstreams map[string]struct {
			url      string
			protocol string
			paths    []string
		}
	}{
		{
			name:         "simple http URL without paths",
			upstreamsEnv: "payment=http://payment.payments.svc.cluster.local:8080",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				paths    []string
			}{
				"payment": {
					url:      "http://payment.payments.svc.cluster.local:8080",
					protocol: "http",
					paths:    nil,
				},
			},
		},
		{
			name:         "simple grpc URL without paths",
			upstreamsEnv: "payment=grpc://payment.payments.svc.cluster.local:9090",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				paths    []string
			}{
				"payment": {
					url:      "grpc://payment.payments.svc.cluster.local:9090",
					protocol: "grpc",
					paths:    nil,
				},
			},
		},
		{
			name:         "http URL with single path",
			upstreamsEnv: "message-bus=http://message-bus.infra.svc.cluster.local:8080:/events/OrderCreated",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				paths    []string
			}{
				"message-bus": {
					url:      "http://message-bus.infra.svc.cluster.local:8080",
					protocol: "http",
					paths:    []string{"/events/OrderCreated"},
				},
			},
		},
		{
			name:         "http URL with multiple paths - single upstream only",
			upstreamsEnv: "message-bus=http://message-bus.infra.svc.cluster.local:8080:/events/OrderCreated,/events/OrderUpdated,/events/OrderCancelled",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				paths    []string
			}{
				"message-bus": {
					url:      "http://message-bus.infra.svc.cluster.local:8080",
					protocol: "http",
					paths:    []string{"/events/OrderCreated", "/events/OrderUpdated", "/events/OrderCancelled"},
				},
			},
		},
		{
			name:         "multiple upstreams without paths",
			upstreamsEnv: "payment=grpc://payment:9090|inventory=grpc://inventory:9090|shipping=http://shipping:8080",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				paths    []string
			}{
				"payment": {
					url:      "grpc://payment:9090",
					protocol: "grpc",
					paths:    nil,
				},
				"inventory": {
					url:      "grpc://inventory:9090",
					protocol: "grpc",
					paths:    nil,
				},
				"shipping": {
					url:      "http://shipping:8080",
					protocol: "http",
					paths:    nil,
				},
			},
		},
		{
			name:         "mixed upstreams with and without paths",
			upstreamsEnv: "inventory=grpc://inventory.products.svc.cluster.local:9090|search=http://search.products.svc.cluster.local:8080|message-bus=http://message-bus.infra.svc.cluster.local:8080:/events/ProductUpdated,/events/PriceChanged",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				paths    []string
			}{
				"inventory": {
					url:      "grpc://inventory.products.svc.cluster.local:9090",
					protocol: "grpc",
					paths:    nil,
				},
				"search": {
					url:      "http://search.products.svc.cluster.local:8080",
					protocol: "http",
					paths:    nil,
				},
				"message-bus": {
					url:      "http://message-bus.infra.svc.cluster.local:8080",
					protocol: "http",
					paths:    []string{"/events/ProductUpdated", "/events/PriceChanged"},
				},
			},
		},
		{
			name:         "paths with spaces get trimmed - single upstream",
			upstreamsEnv: "api-gateway=http://api-gateway:8080:/api/v1/products, /api/v1/catalog",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				paths    []string
			}{
				"api-gateway": {
					url:      "http://api-gateway:8080",
					protocol: "http",
					paths:    []string{"/api/v1/products", "/api/v1/catalog"},
				},
			},
		},
		{
			name:         "backward compatibility - old format without protocol prefix (deprecated)",
			upstreamsEnv: "product-api:product.svc,cart-api:cart.svc",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				paths    []string
			}{
				"product-api": {
					url:      "product.svc",
					protocol: "http", // defaults to http
					paths:    nil,
				},
				"cart-api": {
					url:      "cart.svc",
					protocol: "http",
					paths:    nil,
				},
			},
		},
		{
			name:         "edge case - port 80 without explicit port number",
			upstreamsEnv: "web=http://web.frontend.svc.cluster.local",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				paths    []string
			}{
				"web": {
					url:      "http://web.frontend.svc.cluster.local",
					protocol: "http",
					paths:    nil,
				},
			},
		},
		{
			name:         "edge case - high port numbers",
			upstreamsEnv: "custom=http://custom.svc.cluster.local:50051",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				paths    []string
			}{
				"custom": {
					url:      "http://custom.svc.cluster.local:50051",
					protocol: "http",
					paths:    nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment
			os.Setenv("UPSTREAMS", tt.upstreamsEnv)
			os.Setenv("SERVICE_NAME", "test-service")

			// Load config
			cfg := LoadConfigFromEnv()

			// Verify upstreams count
			if len(cfg.Upstreams) != len(tt.expectedUpstreams) {
				t.Errorf("expected %d upstreams, got %d", len(tt.expectedUpstreams), len(cfg.Upstreams))
			}

			// Verify each upstream
			for name, expected := range tt.expectedUpstreams {
				upstream, exists := cfg.Upstreams[name]
				if !exists {
					t.Errorf("upstream %q not found", name)
					continue
				}

				if upstream.URL != expected.url {
					t.Errorf("upstream %q: expected URL %q, got %q", name, expected.url, upstream.URL)
				}

				if upstream.Protocol != expected.protocol {
					t.Errorf("upstream %q: expected protocol %q, got %q", name, expected.protocol, upstream.Protocol)
				}

				// Compare paths
				if !stringSlicesEqual(upstream.Paths, expected.paths) {
					t.Errorf("upstream %q: expected paths %v, got %v", name, expected.paths, upstream.Paths)
				}
			}
		})
	}
}

func TestLoadConfigFromEnv_EdgeCases(t *testing.T) {
	tests := []struct {
		name              string
		upstreamsEnv      string
		expectUpstreamCnt int
	}{
		{
			name:              "empty upstreams",
			upstreamsEnv:      "",
			expectUpstreamCnt: 0,
		},
		{
			name:              "whitespace only",
			upstreamsEnv:      "   ",
			expectUpstreamCnt: 0,
		},
		{
			name:              "malformed - missing url",
			upstreamsEnv:      "payment=",
			expectUpstreamCnt: 0, // Should skip malformed entries
		},
		{
			name:              "empty paths list",
			upstreamsEnv:      "api=http://api:8080:",
			expectUpstreamCnt: 1, // Should parse but with empty paths
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("UPSTREAMS", tt.upstreamsEnv)
			os.Setenv("SERVICE_NAME", "test-service")

			cfg := LoadConfigFromEnv()

			if len(cfg.Upstreams) != tt.expectUpstreamCnt {
				t.Errorf("expected %d upstreams, got %d", tt.expectUpstreamCnt, len(cfg.Upstreams))
			}
		})
	}
}

func TestLoadConfigFromEnv_CriticalBugRegression(t *testing.T) {
	// This test specifically checks the bug where port numbers were parsed as paths
	t.Run("port number not treated as path", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("UPSTREAMS", "payment=grpc://payment.sf-payments.svc.cluster.local:9090")
		os.Setenv("SERVICE_NAME", "order-management")

		cfg := LoadConfigFromEnv()

		upstream, exists := cfg.Upstreams["payment"]
		if !exists {
			t.Fatal("payment upstream not found")
		}

		// The bug: port "9090" was being parsed as a path
		if len(upstream.Paths) != 0 {
			t.Errorf("BUG REGRESSION: port number parsed as path! Expected empty paths, got %v", upstream.Paths)
		}

		if upstream.URL != "grpc://payment.sf-payments.svc.cluster.local:9090" {
			t.Errorf("URL incorrectly parsed: expected full URL with port, got %q", upstream.URL)
		}
	})

	t.Run("path after port correctly identified", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("UPSTREAMS", "message-bus=http://message-bus:8080:/events/OrderCreated")
		os.Setenv("SERVICE_NAME", "order-management")

		cfg := LoadConfigFromEnv()

		upstream, exists := cfg.Upstreams["message-bus"]
		if !exists {
			t.Fatal("message-bus upstream not found")
		}

		if len(upstream.Paths) != 1 {
			t.Errorf("expected 1 path, got %d", len(upstream.Paths))
		}

		if len(upstream.Paths) > 0 && upstream.Paths[0] != "/events/OrderCreated" {
			t.Errorf("expected path /events/OrderCreated, got %q", upstream.Paths[0])
		}

		if upstream.URL != "http://message-bus:8080" {
			t.Errorf("URL should not include path: got %q", upstream.URL)
		}
	})
}

// Helper function to compare string slices
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
