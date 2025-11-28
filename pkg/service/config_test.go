package service

import (
	"os"
	"testing"
)

// findUpstreamByName finds an upstream by name in the slice
// Returns nil if not found
func findUpstreamByName(upstreams []*UpstreamConfig, name string) *UpstreamConfig {
	for _, u := range upstreams {
		if u.Name == name {
			return u
		}
	}
	return nil
}

func TestLoadConfigFromEnv_UpstreamsParsing(t *testing.T) {
	tests := []struct {
		name              string
		upstreamsEnv      string
		expectedUpstreams map[string]struct {
			url      string
			protocol string
			match    []string
			path     string
		}
	}{
		{
			name:         "simple http URL without match or path",
			upstreamsEnv: "payment=http://payment.payments.svc.cluster.local:8080",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"payment": {
					url:      "http://payment.payments.svc.cluster.local:8080",
					protocol: "http",
					match:    nil,
					path:     "",
				},
			},
		},
		{
			name:         "simple grpc URL without match or path",
			upstreamsEnv: "payment=grpc://payment.payments.svc.cluster.local:9090",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"payment": {
					url:      "grpc://payment.payments.svc.cluster.local:9090",
					protocol: "grpc",
					match:    nil,
					path:     "",
				},
			},
		},
		{
			name:         "http URL with single match",
			upstreamsEnv: "order-api=http://order-api.orders.svc.cluster.local:8080:match=/orders",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"order-api": {
					url:      "http://order-api.orders.svc.cluster.local:8080",
					protocol: "http",
					match:    []string{"/orders"},
					path:     "",
				},
			},
		},
		{
			name:         "http URL with multiple matches",
			upstreamsEnv: "order-api=http://order-api:8080:match=/orders,/cart,/checkout",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"order-api": {
					url:      "http://order-api:8080",
					protocol: "http",
					match:    []string{"/orders", "/cart", "/checkout"},
					path:     "",
				},
			},
		},
		{
			name:         "http URL with path only",
			upstreamsEnv: "message-bus=http://message-bus.infra.svc.cluster.local:8080:path=/events/OrderCreated",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"message-bus": {
					url:      "http://message-bus.infra.svc.cluster.local:8080",
					protocol: "http",
					match:    nil,
					path:     "/events/OrderCreated",
				},
			},
		},
		{
			name:         "http URL with both match and path",
			upstreamsEnv: "api=http://api:8080:match=/api/v1:path=/v2",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"api": {
					url:      "http://api:8080",
					protocol: "http",
					match:    []string{"/api/v1"},
					path:     "/v2",
				},
			},
		},
		{
			name:         "multiple upstreams without match or path",
			upstreamsEnv: "payment=grpc://payment:9090|inventory=grpc://inventory:9090|shipping=http://shipping:8080",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"payment": {
					url:      "grpc://payment:9090",
					protocol: "grpc",
					match:    nil,
					path:     "",
				},
				"inventory": {
					url:      "grpc://inventory:9090",
					protocol: "grpc",
					match:    nil,
					path:     "",
				},
				"shipping": {
					url:      "http://shipping:8080",
					protocol: "http",
					match:    nil,
					path:     "",
				},
			},
		},
		{
			name:         "mixed upstreams with match and path",
			upstreamsEnv: "order-api=http://order:8080:match=/orders|message-bus=http://bus:8080:path=/events/Order",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"order-api": {
					url:      "http://order:8080",
					protocol: "http",
					match:    []string{"/orders"},
					path:     "",
				},
				"message-bus": {
					url:      "http://bus:8080",
					protocol: "http",
					match:    nil,
					path:     "/events/Order",
				},
			},
		},
		{
			name:         "match with spaces gets trimmed",
			upstreamsEnv: "api-gateway=http://api-gateway:8080:match=/api/v1/products, /api/v1/catalog",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"api-gateway": {
					url:      "http://api-gateway:8080",
					protocol: "http",
					match:    []string{"/api/v1/products", "/api/v1/catalog"},
					path:     "",
				},
			},
		},
		{
			name:         "backward compatibility - old format without protocol prefix (deprecated)",
			upstreamsEnv: "product-api:product.svc,cart-api:cart.svc",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"product-api": {
					url:      "product.svc",
					protocol: "http", // defaults to http
					match:    nil,
					path:     "",
				},
				"cart-api": {
					url:      "cart.svc",
					protocol: "http",
					match:    nil,
					path:     "",
				},
			},
		},
		{
			name:         "edge case - port 80 without explicit port number",
			upstreamsEnv: "web=http://web.frontend.svc.cluster.local",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"web": {
					url:      "http://web.frontend.svc.cluster.local",
					protocol: "http",
					match:    nil,
					path:     "",
				},
			},
		},
		{
			name:         "edge case - high port numbers",
			upstreamsEnv: "custom=http://custom.svc.cluster.local:50051",
			expectedUpstreams: map[string]struct {
				url      string
				protocol string
				match    []string
				path     string
			}{
				"custom": {
					url:      "http://custom.svc.cluster.local:50051",
					protocol: "http",
					match:    nil,
					path:     "",
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
				upstream := findUpstreamByName(cfg.Upstreams, name)
				if upstream == nil {
					t.Errorf("upstream %q not found", name)
					continue
				}

				if upstream.URL != expected.url {
					t.Errorf("upstream %q: expected URL %q, got %q", name, expected.url, upstream.URL)
				}

				if upstream.Protocol != expected.protocol {
					t.Errorf("upstream %q: expected protocol %q, got %q", name, expected.protocol, upstream.Protocol)
				}

				// Compare match
				if !stringSlicesEqual(upstream.Match, expected.match) {
					t.Errorf("upstream %q: expected match %v, got %v", name, expected.match, upstream.Match)
				}

				// Compare path
				if upstream.Path != expected.path {
					t.Errorf("upstream %q: expected path %q, got %q", name, expected.path, upstream.Path)
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
	// This test specifically checks the bug where port numbers were parsed incorrectly
	t.Run("port number not treated as match or path", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("UPSTREAMS", "payment=grpc://payment.sf-payments.svc.cluster.local:9090")
		os.Setenv("SERVICE_NAME", "order-management")

		cfg := LoadConfigFromEnv()

		upstream := findUpstreamByName(cfg.Upstreams, "payment")
		if upstream == nil {
			t.Fatal("payment upstream not found")
		}

		// The bug: port "9090" was being parsed incorrectly
		if len(upstream.Match) != 0 {
			t.Errorf("BUG REGRESSION: port number parsed as match! Expected empty match, got %v", upstream.Match)
		}

		if upstream.Path != "" {
			t.Errorf("BUG REGRESSION: port number parsed as path! Expected empty path, got %q", upstream.Path)
		}

		if upstream.URL != "grpc://payment.sf-payments.svc.cluster.local:9090" {
			t.Errorf("URL incorrectly parsed: expected full URL with port, got %q", upstream.URL)
		}
	})

	t.Run("match after port correctly identified", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("UPSTREAMS", "order-api=http://order-api:8080:match=/orders")
		os.Setenv("SERVICE_NAME", "api-gateway")

		cfg := LoadConfigFromEnv()

		upstream := findUpstreamByName(cfg.Upstreams, "order-api")
		if upstream == nil {
			t.Fatal("order-api upstream not found")
		}

		if len(upstream.Match) != 1 {
			t.Errorf("expected 1 match, got %d", len(upstream.Match))
		}

		if len(upstream.Match) > 0 && upstream.Match[0] != "/orders" {
			t.Errorf("expected match /orders, got %q", upstream.Match[0])
		}

		if upstream.URL != "http://order-api:8080" {
			t.Errorf("URL should not include match: got %q", upstream.URL)
		}
	})

	t.Run("path after port correctly identified", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("UPSTREAMS", "message-bus=http://message-bus:8080:path=/events/OrderCreated")
		os.Setenv("SERVICE_NAME", "order-management")

		cfg := LoadConfigFromEnv()

		upstream := findUpstreamByName(cfg.Upstreams, "message-bus")
		if upstream == nil {
			t.Fatal("message-bus upstream not found")
		}

		if upstream.Path != "/events/OrderCreated" {
			t.Errorf("expected path /events/OrderCreated, got %q", upstream.Path)
		}

		if upstream.URL != "http://message-bus:8080" {
			t.Errorf("URL should not include path: got %q", upstream.URL)
		}
	})
}

func TestLoadConfigFromEnv_MultipleSameNameUpstreams(t *testing.T) {
	// Test that multiple upstreams with the same name are preserved (not overwritten)
	t.Run("multiple same-name upstreams preserved", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("UPSTREAMS", "notification=http://notification:8080:match=/events/Order|notification=http://notification:8080:match=/events/Payment")
		os.Setenv("SERVICE_NAME", "message-bus")

		cfg := LoadConfigFromEnv()

		// Should have 2 upstreams, not 1
		if len(cfg.Upstreams) != 2 {
			t.Errorf("expected 2 upstreams, got %d (same-name upstreams were overwritten)", len(cfg.Upstreams))
		}

		// Both should be named "notification"
		for _, u := range cfg.Upstreams {
			if u.Name != "notification" {
				t.Errorf("expected name 'notification', got %q", u.Name)
			}
		}

		// Should have different matches
		matches := make(map[string]bool)
		for _, u := range cfg.Upstreams {
			if len(u.Match) > 0 {
				matches[u.Match[0]] = true
			}
		}

		if !matches["/events/Order"] {
			t.Error("expected /events/Order match")
		}
		if !matches["/events/Payment"] {
			t.Error("expected /events/Payment match")
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
