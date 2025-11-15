package behavior

import (
	"fmt"
	"math/rand"
	"strconv"
)

// PanicBehavior controls pod crash/panic
type PanicBehavior struct {
	Prob float64 // Probability (0.0-1.0)
}

// String returns the string representation of panic behavior
func (pb *PanicBehavior) String() string {
	return fmt.Sprintf("panic=%v", pb.Prob)
}

// parsePanic parses panic specifications
// Examples: "0.5", "1.0"
func parsePanic(value string) (*PanicBehavior, error) {
	prob, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, err
	}
	return &PanicBehavior{Prob: prob}, nil
}

// ShouldPanic determines if a panic should be triggered
func (b *Behavior) ShouldPanic() bool {
	if b.Panic == nil {
		return false
	}

	return rand.Float64() < b.Panic.Prob
}

func init() {
	registerParser("panic", func(b *Behavior, value string) error {
		panicBehavior, err := parsePanic(value)
		if err != nil {
			return fmt.Errorf("invalid panic: %w", err)
		}
		b.Panic = panicBehavior
		return nil
	})
}

