package behavior

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// LatencyBehavior controls request latency
type LatencyBehavior struct {
	Type  string // "fixed", "range", "percentile"
	Min   time.Duration
	Max   time.Duration
	Value time.Duration
}

// String returns the string representation of latency behavior
func (lb *LatencyBehavior) String() string {
	if lb.Type == "fixed" {
		return fmt.Sprintf("latency=%s", lb.Value)
	}
	return fmt.Sprintf("latency=%s-%s", lb.Min, lb.Max)
}

// parseLatency parses latency specifications
// Examples: "100ms", "50-200ms", "50ms-200ms", "5-20ms"
func parseLatency(value string) (*LatencyBehavior, error) {
	lb := &LatencyBehavior{}

	if strings.Contains(value, "-") {
		// Range: "50-200ms" or "50ms-200ms"
		parts := strings.Split(value, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format")
		}

		minStr := strings.TrimSpace(parts[0])
		maxStr := strings.TrimSpace(parts[1])

		// Try parsing min value
		min, err := time.ParseDuration(minStr)
		if err != nil {
			// If min doesn't have a unit, try to extract unit from max
			max, err2 := time.ParseDuration(maxStr)
			if err2 != nil {
				return nil, fmt.Errorf("invalid range: %w", err)
			}

			// Extract unit from max value (e.g., "200ms" -> "ms")
			unit := extractUnit(maxStr)
			if unit == "" {
				return nil, fmt.Errorf("could not determine unit from range")
			}

			// Append unit to min value
			minStr = minStr + unit
			min, err = time.ParseDuration(minStr)
			if err != nil {
				return nil, err
			}

			lb.Type = "range"
			lb.Min = min
			lb.Max = max
		} else {
			// Both values parsed successfully
			max, err := time.ParseDuration(maxStr)
			if err != nil {
				return nil, err
			}
			lb.Type = "range"
			lb.Min = min
			lb.Max = max
		}
	} else {
		// Fixed: "100ms"
		d, err := time.ParseDuration(value)
		if err != nil {
			return nil, err
		}
		lb.Type = "fixed"
		lb.Value = d
	}

	return lb, nil
}

// applyLatency applies latency behavior
func (b *Behavior) applyLatency(ctx context.Context) error {
	var delay time.Duration

	switch b.Latency.Type {
	case "fixed":
		delay = b.Latency.Value
	case "range":
		// Random duration between min and max
		diff := b.Latency.Max - b.Latency.Min
		delay = b.Latency.Min + time.Duration(rand.Int63n(int64(diff)))
	default:
		delay = b.Latency.Value
	}

	if delay > 0 {
		select {
		case <-time.After(delay):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func init() {
	registerParser("latency", func(b *Behavior, value string) error {
		latency, err := parseLatency(value)
		if err != nil {
			return fmt.Errorf("invalid latency: %w", err)
		}
		b.Latency = latency
		return nil
	})
}

