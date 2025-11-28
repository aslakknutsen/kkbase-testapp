package router

import (
	"math/rand"
	"strings"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
)

// Router handles upstream service routing based on request paths
type Router interface {
	// Match returns upstreams that handle the given path
	// Returns nil if no upstreams match (404 case)
	Match(path string) []*service.UpstreamConfig

	// MatchWithWeights returns upstreams that handle the given path,
	// applying weighted selection for grouped upstreams
	MatchWithWeights(path string, weights map[string]int) []*service.UpstreamConfig

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

// Match returns upstreams that match the given path (no weighted selection)
func (r *PathRouter) Match(path string) []*service.UpstreamConfig {
	return r.MatchWithWeights(path, nil)
}

// MatchWithWeights returns upstreams that match the given path,
// applying weighted selection for grouped upstreams.
// For upstreams in the same group, one is selected based on weights.
// Ungrouped upstreams are always included.
func (r *PathRouter) MatchWithWeights(path string, weights map[string]int) []*service.UpstreamConfig {
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

	// Apply weighted selection for grouped upstreams
	return r.applyWeightedSelection(matched, weights)
}

// applyWeightedSelection applies weighted selection for grouped upstreams and probability for ungrouped
// - Upstreams with the same Group are mutually exclusive (one selected based on weights)
// - Ungrouped upstreams with Probability > 0: included based on probability roll
// - Ungrouped upstreams with Probability == 0: always included
func (r *PathRouter) applyWeightedSelection(upstreams []*service.UpstreamConfig, weights map[string]int) []*service.UpstreamConfig {
	if len(upstreams) == 0 {
		return nil
	}

	// Group upstreams by their Group field
	groups := make(map[string][]*service.UpstreamConfig)
	var ungrouped []*service.UpstreamConfig

	for _, u := range upstreams {
		if u.Group == "" {
			ungrouped = append(ungrouped, u)
		} else {
			groups[u.Group] = append(groups[u.Group], u)
		}
	}

	// Process ungrouped upstreams - apply probability filtering
	var result []*service.UpstreamConfig
	for _, u := range ungrouped {
		if u.Probability > 0 {
			// Roll probability to decide if included
			if rand.Float64() < u.Probability {
				result = append(result, u)
			}
		} else {
			// No probability set = always included
			result = append(result, u)
		}
	}

	// For each group, select one upstream based on weights
	for _, groupUpstreams := range groups {
		selected := selectWeighted(groupUpstreams, weights)
		if selected != nil {
			result = append(result, selected)
		}
	}

	return result
}

// selectWeighted selects one upstream from the group based on weights
// If weights are not specified for an upstream, it gets an equal share of remaining weight
func selectWeighted(upstreams []*service.UpstreamConfig, weights map[string]int) *service.UpstreamConfig {
	if len(upstreams) == 0 {
		return nil
	}
	if len(upstreams) == 1 {
		return upstreams[0]
	}

	// Calculate effective weights
	effectiveWeights := make([]int, len(upstreams))
	totalExplicit := 0
	explicitCount := 0

	for i, u := range upstreams {
		if w, ok := weights[u.Name]; ok && w > 0 {
			effectiveWeights[i] = w
			totalExplicit += w
			explicitCount++
		}
	}

	// Distribute remaining weight equally among unspecified upstreams
	unspecifiedCount := len(upstreams) - explicitCount
	if unspecifiedCount > 0 {
		// If total explicit is >= 100, unspecified get 0
		// Otherwise, remaining weight is split equally
		remaining := 100 - totalExplicit
		if remaining < 0 {
			remaining = 0
		}
		perUnspecified := remaining / unspecifiedCount

		for i, u := range upstreams {
			if _, ok := weights[u.Name]; !ok {
				effectiveWeights[i] = perUnspecified
			}
		}
	}

	// If no weights specified at all, use equal distribution
	if explicitCount == 0 {
		for i := range effectiveWeights {
			effectiveWeights[i] = 100 / len(upstreams)
		}
	}

	// Calculate total weight
	totalWeight := 0
	for _, w := range effectiveWeights {
		totalWeight += w
	}

	if totalWeight <= 0 {
		// Fallback: pick randomly with equal probability
		return upstreams[rand.Intn(len(upstreams))]
	}

	// Random selection based on weights
	r := rand.Intn(totalWeight)
	cumulative := 0
	for i, w := range effectiveWeights {
		cumulative += w
		if r < cumulative {
			return upstreams[i]
		}
	}

	// Fallback (shouldn't happen)
	return upstreams[len(upstreams)-1]
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

// MatchWithWeights always returns nil for NoOpRouter
func (r *NoOpRouter) MatchWithWeights(path string, weights map[string]int) []*service.UpstreamConfig {
	return nil
}

// GetForwardPath returns "/" for NoOpRouter
func (r *NoOpRouter) GetForwardPath(upstream *service.UpstreamConfig) string {
	return "/"
}
