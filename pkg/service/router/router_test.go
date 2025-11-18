package router

import (
	"testing"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
)

func TestPathRouter_Match(t *testing.T) {
	upstreams := map[string]*service.UpstreamConfig{
		"api": {
			Name:  "api",
			Paths: []string{"/api"},
		},
		"web": {
			Name:  "web",
			Paths: []string{"/web", "/assets"},
		},
		"catch-all": {
			Name:  "catch-all",
			Paths: nil, // Catch-all
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

			for _, name := range tt.expectMatches {
				if _, ok := matched[name]; !ok {
					t.Errorf("Expected %s to match, but didn't", name)
				}
			}
		})
	}
}

func TestPathRouter_NoMatch(t *testing.T) {
	upstreams := map[string]*service.UpstreamConfig{
		"api": {
			Name:  "api",
			Paths: []string{"/api"},
		},
	}

	router := NewPathRouter(upstreams)
	matched := router.Match("/other")

	if matched != nil {
		t.Error("Expected no match for /other, but got matches")
	}
}

func TestPathRouter_StripPrefix(t *testing.T) {
	upstream := &service.UpstreamConfig{
		Name:  "api",
		Paths: []string{"/api/v1", "/api"},
	}

	router := NewPathRouter(nil)

	tests := []struct {
		path     string
		expected string
	}{
		{"/api/v1/users", "/users"},   // Longest match
		{"/api/users", "/users"},      // Shorter match
		{"/api", "/"},                 // Exact match
		{"/other", "/other"},          // No match, return as-is
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := router.StripPrefix(tt.path, upstream)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestPathRouter_StripPrefix_NoPaths(t *testing.T) {
	upstream := &service.UpstreamConfig{
		Name:  "catch-all",
		Paths: nil, // No paths configured
	}

	router := NewPathRouter(nil)
	result := router.StripPrefix("/api/users", upstream)

	if result != "/api/users" {
		t.Errorf("Expected path unchanged for catch-all upstream, got %s", result)
	}
}

func TestPathRouter_HasUpstreams(t *testing.T) {
	tests := []struct {
		name      string
		upstreams map[string]*service.UpstreamConfig
		expected  bool
	}{
		{
			name:      "has upstreams",
			upstreams: map[string]*service.UpstreamConfig{"api": {Name: "api"}},
			expected:  true,
		},
		{
			name:      "no upstreams",
			upstreams: map[string]*service.UpstreamConfig{},
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
	router := NewPathRouter(map[string]*service.UpstreamConfig{})

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
	stripped := router.StripPrefix("/api/users", upstream)
	if stripped != "/api/users" {
		t.Errorf("NoOpRouter should return path unchanged, got %s", stripped)
	}
}

func TestPathRouter_MultiplePathsPerUpstream(t *testing.T) {
	upstreams := map[string]*service.UpstreamConfig{
		"content": {
			Name:  "content",
			Paths: []string{"/blog", "/news", "/articles"},
		},
	}

	router := NewPathRouter(upstreams)

	testPaths := []string{"/blog/post-1", "/news/latest", "/articles/tech"}
	for _, path := range testPaths {
		matched := router.Match(path)
		if len(matched) != 1 {
			t.Errorf("Expected 1 match for %s, got %d", path, len(matched))
		}
		if _, ok := matched["content"]; !ok {
			t.Errorf("Expected content to match for %s", path)
		}
	}
}

func TestPathRouter_LongestPrefixWins(t *testing.T) {
	upstream := &service.UpstreamConfig{
		Name:  "api",
		Paths: []string{"/api", "/api/v1", "/api/v2"},
	}

	router := NewPathRouter(nil)

	tests := []struct {
		path     string
		expected string
		desc     string
	}{
		{"/api/v1/users", "/users", "v1 prefix is longest"},
		{"/api/v2/products", "/products", "v2 prefix is longest"},
		{"/api/other", "/other", "only api prefix matches"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := router.StripPrefix(tt.path, upstream)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

