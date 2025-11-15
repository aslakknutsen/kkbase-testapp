package behavior

import (
	"context"
	"testing"
	"time"
)

func TestParseLatency(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		validate  func(t *testing.T, b *Behavior)
	}{
		{
			name:      "fixed latency",
			input:     "latency=100ms",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Latency == nil {
					t.Fatal("expected latency behavior")
				}
				if b.Latency.Type != "fixed" {
					t.Errorf("expected fixed type, got %s", b.Latency.Type)
				}
				if b.Latency.Value != 100*time.Millisecond {
					t.Errorf("expected 100ms, got %v", b.Latency.Value)
				}
			},
		},
		{
			name:      "range latency with units on both",
			input:     "latency=50ms-200ms",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Latency == nil {
					t.Fatal("expected latency behavior")
				}
				if b.Latency.Type != "range" {
					t.Errorf("expected range type, got %s", b.Latency.Type)
				}
				if b.Latency.Min != 50*time.Millisecond {
					t.Errorf("expected min 50ms, got %v", b.Latency.Min)
				}
				if b.Latency.Max != 200*time.Millisecond {
					t.Errorf("expected max 200ms, got %v", b.Latency.Max)
				}
			},
		},
		{
			name:      "range latency with unit only on max",
			input:     "latency=50-200ms",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Latency == nil {
					t.Fatal("expected latency behavior")
				}
				if b.Latency.Type != "range" {
					t.Errorf("expected range type, got %s", b.Latency.Type)
				}
				if b.Latency.Min != 50*time.Millisecond {
					t.Errorf("expected min 50ms, got %v", b.Latency.Min)
				}
				if b.Latency.Max != 200*time.Millisecond {
					t.Errorf("expected max 200ms, got %v", b.Latency.Max)
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

func TestApplyLatency(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, start time.Time, b *Behavior)
	}{
		{
			name:  "latency applied",
			input: "latency=100ms",
			validate: func(t *testing.T, start time.Time, b *Behavior) {
				elapsed := time.Since(start)
				if elapsed < 100*time.Millisecond {
					t.Errorf("expected at least 100ms delay, got %v", elapsed)
				}
				if elapsed > 150*time.Millisecond {
					t.Errorf("expected around 100ms delay, got %v (too long)", elapsed)
				}
			},
		},
		{
			name:  "range latency",
			input: "latency=50-100ms",
			validate: func(t *testing.T, start time.Time, b *Behavior) {
				elapsed := time.Since(start)
				if elapsed < 50*time.Millisecond {
					t.Errorf("expected at least 50ms delay, got %v", elapsed)
				}
				if elapsed > 150*time.Millisecond {
					t.Errorf("expected max 100ms delay (with tolerance), got %v", elapsed)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() failed: %v", err)
			}

			start := time.Now()
			ctx := context.Background()
			err = b.Apply(ctx)
			if err != nil {
				t.Fatalf("Apply() failed: %v", err)
			}

			tt.validate(t, start, b)
		})
	}
}

func TestLatencyString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "latency only",
			input:    "latency=100ms",
			expected: "latency=100ms",
		},
		{
			name:     "latency range",
			input:    "latency=50ms-200ms",
			expected: "latency=50ms-200ms",
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

