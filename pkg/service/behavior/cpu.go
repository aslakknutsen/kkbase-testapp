package behavior

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// CPUBehavior controls CPU usage patterns
type CPUBehavior struct {
	Pattern   string // "spike", "steady", "ramp"
	Duration  time.Duration
	Intensity int // Percentage 0-100
}

// String returns the string representation of CPU behavior
func (cb *CPUBehavior) String() string {
	cpuStr := fmt.Sprintf("cpu=%s", cb.Pattern)
	if cb.Duration > 0 {
		cpuStr += fmt.Sprintf(":%s:%d", cb.Duration, cb.Intensity)
	}
	return cpuStr
}

// parseCPU parses CPU behavior specifications
// Examples: "spike", "spike:5s", "steady:10s:50"
func parseCPU(value string) (*CPUBehavior, error) {
	parts := strings.Split(value, ":")
	cb := &CPUBehavior{
		Pattern:   parts[0],
		Duration:  5 * time.Second,
		Intensity: 80,
	}

	if len(parts) > 1 {
		d, err := time.ParseDuration(parts[1])
		if err != nil {
			return nil, err
		}
		cb.Duration = d
	}

	if len(parts) > 2 {
		intensity, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, err
		}
		cb.Intensity = intensity
	}

	return cb, nil
}

// applyCPU applies CPU load
func (b *Behavior) applyCPU(ctx context.Context) {
	go func() {
		deadline := time.Now().Add(b.CPU.Duration)

		// Calculate work duration based on intensity
		// intensity = 80 means 80% busy, 20% idle
		workDuration := time.Duration(float64(b.CPU.Intensity) / 100.0 * float64(10*time.Millisecond))
		idleDuration := 10*time.Millisecond - workDuration

		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return
			default:
				// Do CPU-intensive work
				start := time.Now()
				for time.Since(start) < workDuration {
					// Busy loop - consume CPU
					_ = math.Sqrt(rand.Float64())
				}

				// Idle period
				if idleDuration > 0 {
					time.Sleep(idleDuration)
				}
			}
		}
	}()
}

func init() {
	registerParser("cpu", func(b *Behavior, value string) error {
		cpu, err := parseCPU(value)
		if err != nil {
			return fmt.Errorf("invalid cpu: %w", err)
		}
		b.CPU = cpu
		return nil
	})
}
