package behavior

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

// ErrorBehavior controls error injection
type ErrorBehavior struct {
	Rate int     // HTTP status code to return
	Prob float64 // Probability (0.0-1.0)
}

// String returns the string representation of error behavior
func (eb *ErrorBehavior) String() string {
	// Always include rate when prob < 1.0, omit when prob is 1.0 and rate is 500
	if eb.Prob < 1.0 || eb.Rate != 500 {
		return fmt.Sprintf("error=%d:%v", eb.Rate, eb.Prob)
	}
	return fmt.Sprintf("error=%d", eb.Rate)
}

// parseError parses error injection specifications
// Examples: "503", "0.1", "503:0.1"
func parseError(value string) (*ErrorBehavior, error) {
	eb := &ErrorBehavior{
		Rate: 500, // Default error code
		Prob: 0.0,
	}

	if strings.Contains(value, ":") {
		// Code and probability: "503:0.1"
		parts := strings.Split(value, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid error format")
		}
		code, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, err
		}
		prob, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, err
		}
		eb.Rate = code
		eb.Prob = prob
	} else {
		// Just probability or just code
		if strings.Contains(value, ".") {
			// Probability
			prob, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, err
			}
			eb.Prob = prob
		} else {
			// HTTP code with 100% probability
			code, err := strconv.Atoi(value)
			if err != nil {
				return nil, err
			}
			eb.Rate = code
			eb.Prob = 1.0
		}
	}

	return eb, nil
}

// ShouldError determines if an error should be injected
func (b *Behavior) ShouldError() (bool, int) {
	if b.Error == nil {
		return false, 0
	}

	if rand.Float64() < b.Error.Prob {
		return true, b.Error.Rate
	}

	return false, 0
}

func init() {
	registerParser("error", func(b *Behavior, value string) error {
		errorBehavior, err := parseError(value)
		if err != nil {
			return fmt.Errorf("invalid error: %w", err)
		}
		b.Error = errorBehavior
		return nil
	})
}

