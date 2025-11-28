package router

import (
	"testing"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
)

func TestPathRouter_Match(t *testing.T) {
	upstreams := []*service.UpstreamConfig{
		{
			Name:  "api",
			Match: []string{"/api"},
		},
		{
			Name:  "web",
			Match: []string{"/web", "/assets"},
		},
		{
			Name:  "catch-all",
			Match: nil, // Catch-all
		},
	}

	router := NewPathRouter(upstreams)

	tests := []struct {
		name          string
		path          string
		expectMatches []string
	}{
		{
			name:          "matches api prefix",
			path:          "/api/users",
			expectMatches: []string{"api", "catch-all"},
		},
		{
			name:          "matches web prefix",
			path:          "/web/index.html",
			expectMatches: []string{"web", "catch-all"},
		},
		{
			name:          "matches assets prefix",
			path:          "/assets/logo.png",
			expectMatches: []string{"web", "catch-all"},
		},
		{
			name:          "only catch-all matches unknown path",
			path:          "/unknown",
			expectMatches: []string{"catch-all"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := router.Match(tt.path)

			if len(matched) != len(tt.expectMatches) {
				t.Errorf("Expected %d matches, got %d", len(tt.expectMatches), len(matched))
			}

			// Check that all expected names are in the matched slice
			for _, expectedName := range tt.expectMatches {
				found := false
				for _, m := range matched {
					if m.Name == expectedName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected %s to match, but didn't", expectedName)
				}
			}
		})
	}
}

func TestPathRouter_NoMatch(t *testing.T) {
	upstreams := []*service.UpstreamConfig{
		{
			Name:  "api",
			Match: []string{"/api"},
		},
	}

	router := NewPathRouter(upstreams)
	matched := router.Match("/other")

	if matched != nil {
		t.Error("Expected no match for /other, but got matches")
	}
}

func TestPathRouter_GetForwardPath(t *testing.T) {
	router := NewPathRouter(nil)

	tests := []struct {
		name     string
		upstream *service.UpstreamConfig
		expected string
	}{
		{
			name: "explicit path",
			upstream: &service.UpstreamConfig{
				Name: "api",
				Path: "/v2/api",
			},
			expected: "/v2/api",
		},
		{
			name: "empty path defaults to /",
			upstream: &service.UpstreamConfig{
				Name: "api",
				Path: "",
			},
			expected: "/",
		},
		{
			name: "nil upstream path defaults to /",
			upstream: &service.UpstreamConfig{
				Name: "api",
			},
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.GetForwardPath(tt.upstream)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestPathRouter_HasUpstreams(t *testing.T) {
	tests := []struct {
		name      string
		upstreams []*service.UpstreamConfig
		expected  bool
	}{
		{
			name:      "has upstreams",
			upstreams: []*service.UpstreamConfig{{Name: "api"}},
			expected:  true,
		},
		{
			name:      "no upstreams",
			upstreams: []*service.UpstreamConfig{},
			expected:  false,
		},
		{
			name:      "nil upstreams",
			upstreams: nil,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewPathRouter(tt.upstreams)
			result := router.HasUpstreams()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestPathRouter_EmptyUpstreams(t *testing.T) {
	router := NewPathRouter([]*service.UpstreamConfig{})

	matched := router.Match("/any")
	if matched != nil {
		t.Error("Expected nil for empty upstreams")
	}
}

func TestNoOpRouter(t *testing.T) {
	router := NewNoOpRouter()

	if router.HasUpstreams() {
		t.Error("NoOpRouter should not have upstreams")
	}

	matched := router.Match("/any/path")
	if matched != nil {
		t.Error("NoOpRouter should not match any path")
	}

	upstream := &service.UpstreamConfig{Name: "test"}
	forwardPath := router.GetForwardPath(upstream)
	if forwardPath != "/" {
		t.Errorf("NoOpRouter should return / as forward path, got %s", forwardPath)
	}
}

func TestPathRouter_MultiplePathsPerUpstream(t *testing.T) {
	upstreams := []*service.UpstreamConfig{
		{
			Name:  "content",
			Match: []string{"/blog", "/news", "/articles"},
		},
	}

	router := NewPathRouter(upstreams)

	testPaths := []string{"/blog/post-1", "/news/latest", "/articles/tech"}
	for _, path := range testPaths {
		matched := router.Match(path)
		if len(matched) != 1 {
			t.Errorf("Expected 1 match for %s, got %d", path, len(matched))
		}
		if matched[0].Name != "content" {
			t.Errorf("Expected content to match for %s", path)
		}
	}
}

func TestPathRouter_MatchWithExplicitPath(t *testing.T) {
	upstreams := []*service.UpstreamConfig{
		{
			Name:  "api-v2",
			Match: []string{"/api/v1"},
			Path:  "/v2",
		},
	}

	router := NewPathRouter(upstreams)

	// Should match
	matched := router.Match("/api/v1/users")
	if len(matched) != 1 {
		t.Errorf("Expected 1 match, got %d", len(matched))
	}

	// Forward path should be explicit
	forwardPath := router.GetForwardPath(matched[0])
	if forwardPath != "/v2" {
		t.Errorf("Expected forward path /v2, got %s", forwardPath)
	}
}

func TestPathRouter_MultipleSameNameUpstreams(t *testing.T) {
	// Test that multiple upstreams with the same name but different matches work
	upstreams := []*service.UpstreamConfig{
		{
			Name:  "notification",
			Match: []string{"/events/OrderCreated"},
		},
		{
			Name:  "notification",
			Match: []string{"/events/PaymentProcessed"},
		},
	}

	router := NewPathRouter(upstreams)

	// Should match first notification
	matched := router.Match("/events/OrderCreated")
	if len(matched) != 1 {
		t.Errorf("Expected 1 match, got %d", len(matched))
	}
	if matched[0].Name != "notification" {
		t.Errorf("Expected notification to match")
	}

	// Should match second notification
	matched = router.Match("/events/PaymentProcessed")
	if len(matched) != 1 {
		t.Errorf("Expected 1 match, got %d", len(matched))
	}
	if matched[0].Name != "notification" {
		t.Errorf("Expected notification to match")
	}

	// Should not match unknown
	matched = router.Match("/events/Unknown")
	if matched != nil {
		t.Error("Expected no match for unknown event")
	}
}
