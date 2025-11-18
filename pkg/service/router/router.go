package router

import (
	"strings"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
)

// Router handles upstream service routing based on request paths
type Router interface {
	// Match returns upstreams that handle the given path
	// Returns nil if no upstreams match (404 case)
	Match(path string) map[string]*service.UpstreamConfig
	
	// StripPrefix removes the matched prefix from the path for upstream forwarding
	StripPrefix(path string, upstream *service.UpstreamConfig) string
	
	// HasUpstreams returns true if any upstreams are configured
	HasUpstreams() bool
}

// PathRouter implements path-based routing for HTTP upstreams
type PathRouter struct {
	upstreams map[string]*service.UpstreamConfig
}

// NewPathRouter creates a new path-based router
func NewPathRouter(upstreams map[string]*service.UpstreamConfig) *PathRouter {
	return &PathRouter{
		upstreams: upstreams,
	}
}

// HasUpstreams returns true if any upstreams are configured
func (r *PathRouter) HasUpstreams() bool {
	return len(r.upstreams) > 0
}

// Match returns upstreams that match the given path
func (r *PathRouter) Match(path string) map[string]*service.UpstreamConfig {
	if len(r.upstreams) == 0 {
		return nil
	}

	matched := make(map[string]*service.UpstreamConfig)
	hasAnyPathConfig := false

	for name, upstream := range r.upstreams {
		if len(upstream.Paths) == 0 {
			// No paths configured = catch-all
			matched[name] = upstream
		} else {
			hasAnyPathConfig = true
			// Check if path matches any prefix
			for _, prefix := range upstream.Paths {
				if strings.HasPrefix(path, prefix) {
					matched[name] = upstream
					break
				}
			}
		}
	}

	// If some upstreams have path config but none matched, return empty
	if hasAnyPathConfig && len(matched) == 0 {
		return nil
	}

	return matched
}

// StripPrefix removes the matched path prefix from the request path
func (r *PathRouter) StripPrefix(path string, upstream *service.UpstreamConfig) string {
	if len(upstream.Paths) == 0 {
		return path // No paths configured, don't strip
	}

	// Find longest matching prefix
	longestMatch := ""
	for _, prefix := range upstream.Paths {
		if strings.HasPrefix(path, prefix) && len(prefix) > len(longestMatch) {
			longestMatch = prefix
		}
	}

	if longestMatch != "" {
		stripped := strings.TrimPrefix(path, longestMatch)
		if stripped == "" {
			return "/"
		}
		return stripped
	}
	return path
}

// NoOpRouter is a router that never matches (for gRPC or leaf services)
type NoOpRouter struct{}

// NewNoOpRouter creates a new no-op router
func NewNoOpRouter() *NoOpRouter {
	return &NoOpRouter{}
}

// HasUpstreams always returns false for NoOpRouter
func (r *NoOpRouter) HasUpstreams() bool {
	return false
}

// Match always returns nil for NoOpRouter
func (r *NoOpRouter) Match(path string) map[string]*service.UpstreamConfig {
	return nil
}

// StripPrefix returns the path unchanged for NoOpRouter
func (r *NoOpRouter) StripPrefix(path string, upstream *service.UpstreamConfig) string {
	return path
}

