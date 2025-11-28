package router

import (
	"strings"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
)

// Router handles upstream service routing based on request paths
type Router interface {
	// Match returns upstreams that handle the given path
	// Returns nil if no upstreams match (404 case)
	Match(path string) []*service.UpstreamConfig

	// GetForwardPath returns the path to use when calling the upstream
	// Returns the upstream's explicit Path if set, otherwise "/"
	GetForwardPath(upstream *service.UpstreamConfig) string

	// HasUpstreams returns true if any upstreams are configured
	HasUpstreams() bool
}

// PathRouter implements path-based routing for HTTP upstreams
type PathRouter struct {
	upstreams []*service.UpstreamConfig
}

// NewPathRouter creates a new path-based router
func NewPathRouter(upstreams []*service.UpstreamConfig) *PathRouter {
	return &PathRouter{
		upstreams: upstreams,
	}
}

// HasUpstreams returns true if any upstreams are configured
func (r *PathRouter) HasUpstreams() bool {
	return len(r.upstreams) > 0
}

// Match returns upstreams that match the given path
func (r *PathRouter) Match(path string) []*service.UpstreamConfig {
	if len(r.upstreams) == 0 {
		return nil
	}

	var matched []*service.UpstreamConfig
	hasAnyMatchConfig := false

	for _, upstream := range r.upstreams {
		if len(upstream.Match) == 0 {
			// No match configured = catch-all (always call this upstream)
			matched = append(matched, upstream)
		} else {
			hasAnyMatchConfig = true
			// Check if path matches any prefix in Match
			for _, prefix := range upstream.Match {
				if strings.HasPrefix(path, prefix) {
					matched = append(matched, upstream)
					break
				}
			}
		}
	}

	// If some upstreams have match config but none matched, return empty (404)
	if hasAnyMatchConfig && len(matched) == 0 {
		return nil
	}

	return matched
}

// GetForwardPath returns the path to use when calling the upstream
// Returns the upstream's explicit Path if set, otherwise "/"
func (r *PathRouter) GetForwardPath(upstream *service.UpstreamConfig) string {
	if upstream.Path != "" {
		return upstream.Path
	}
	return "/"
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
func (r *NoOpRouter) Match(path string) []*service.UpstreamConfig {
	return nil
}

// GetForwardPath returns "/" for NoOpRouter
func (r *NoOpRouter) GetForwardPath(upstream *service.UpstreamConfig) string {
	return "/"
}
