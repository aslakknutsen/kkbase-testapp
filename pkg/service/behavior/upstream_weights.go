package behavior

import (
	"fmt"
	"strconv"
	"strings"
)

// UpstreamWeightsBehavior controls weighted selection of grouped upstreams
type UpstreamWeightsBehavior struct {
	Weights map[string]int // upstream ID -> weight (relative, normalized at selection time)
}

// String returns the string representation of upstream weights behavior
// Format: upstreamWeights=id1:weight1;id2:weight2
func (uw *UpstreamWeightsBehavior) String() string {
	if len(uw.Weights) == 0 {
		return ""
	}

	var parts []string
	for id, weight := range uw.Weights {
		parts = append(parts, fmt.Sprintf("%s:%d", id, weight))
	}
	return fmt.Sprintf("upstreamWeights=%s", strings.Join(parts, ";"))
}

// GetWeight returns the weight for a given upstream ID, or 0 if not set
func (uw *UpstreamWeightsBehavior) GetWeight(id string) int {
	if uw == nil || uw.Weights == nil {
		return 0
	}
	return uw.Weights[id]
}

// parseUpstreamWeights parses upstream weight specifications
// Format: id1:weight1;id2:weight2
// Example: "payment-processed:85;payment-failed:5;payment-refunded:10"
func parseUpstreamWeights(value string) (*UpstreamWeightsBehavior, error) {
	uw := &UpstreamWeightsBehavior{
		Weights: make(map[string]int),
	}

	// Split by semicolon (using ; to avoid conflict with , in behavior chain)
	parts := strings.Split(value, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by colon to get id:weight
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid upstream weight format: %s (expected id:weight)", part)
		}

		id := strings.TrimSpace(kv[0])
		weightStr := strings.TrimSpace(kv[1])

		weight, err := strconv.Atoi(weightStr)
		if err != nil {
			return nil, fmt.Errorf("invalid weight for %s: %w", id, err)
		}

		if weight < 0 {
			return nil, fmt.Errorf("weight for %s cannot be negative", id)
		}

		uw.Weights[id] = weight
	}

	if len(uw.Weights) == 0 {
		return nil, fmt.Errorf("no valid upstream weights found")
	}

	return uw, nil
}

func init() {
	registerParser("upstreamWeights", func(b *Behavior, value string) error {
		weights, err := parseUpstreamWeights(value)
		if err != nil {
			return fmt.Errorf("invalid upstreamWeights: %w", err)
		}
		b.UpstreamWeights = weights
		return nil
	})
}

