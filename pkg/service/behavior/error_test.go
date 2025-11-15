package behavior

import (
	"testing"
)

func TestParseError(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		validate  func(t *testing.T, b *Behavior)
	}{
		{
			name:      "error with probability only",
			input:     "error=0.5",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Error == nil {
					t.Fatal("expected error behavior")
				}
				if b.Error.Prob != 0.5 {
					t.Errorf("expected prob 0.5, got %v", b.Error.Prob)
				}
				if b.Error.Rate != 500 {
					t.Errorf("expected default rate 500, got %v", b.Error.Rate)
				}
			},
		},
		{
			name:      "error with code only",
			input:     "error=503",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Error == nil {
					t.Fatal("expected error behavior")
				}
				if b.Error.Prob != 1.0 {
					t.Errorf("expected prob 1.0, got %v", b.Error.Prob)
				}
				if b.Error.Rate != 503 {
					t.Errorf("expected rate 503, got %v", b.Error.Rate)
				}
			},
		},
		{
			name:      "error with probability and code (colon syntax)",
			input:     "error=503:0.3",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Error == nil {
					t.Fatal("expected error behavior")
				}
				if b.Error.Prob != 0.3 {
					t.Errorf("expected prob 0.3, got %v", b.Error.Prob)
				}
				if b.Error.Rate != 503 {
					t.Errorf("expected rate 503, got %v", b.Error.Rate)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Parse(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("Parse() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && tt.validate != nil {
				tt.validate(t, b)
			}
		})
	}
}

func TestErrorString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "error with default code",
			input:    "error=0.5",
			expected: "error=500:0.5",
		},
		{
			name:     "error with custom code",
			input:    "error=503:0.5",
			expected: "error=503:0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() failed: %v", err)
			}
			result := b.String()
			if result != tt.expected {
				t.Errorf("String() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestShouldError(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		iterations     int
		expectedRate   float64
		toleranceRange float64 // acceptable variance
	}{
		{
			name:           "50% error rate",
			input:          "error=500:0.5",
			iterations:     1000,
			expectedRate:   0.5,
			toleranceRange: 0.1, // 10% tolerance
		},
		{
			name:           "10% error rate",
			input:          "error=503:0.1",
			iterations:     1000,
			expectedRate:   0.1,
			toleranceRange: 0.05,
		},
		{
			name:           "100% error rate",
			input:          "error=503",
			iterations:     100,
			expectedRate:   1.0,
			toleranceRange: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() failed: %v", err)
			}

			errorCount := 0
			for i := 0; i < tt.iterations; i++ {
				if shouldErr, _ := b.ShouldError(); shouldErr {
					errorCount++
				}
			}

			actualRate := float64(errorCount) / float64(tt.iterations)
			diff := actualRate - tt.expectedRate
			if diff < 0 {
				diff = -diff
			}

			if diff > tt.toleranceRange {
				t.Errorf("Error rate = %v, want %v (±%v)", actualRate, tt.expectedRate, tt.toleranceRange)
			}
		})
	}
}

