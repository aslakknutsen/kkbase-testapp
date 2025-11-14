package behavior

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestParseBehavior tests basic behavior parsing
func TestParseBehavior(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		validate  func(t *testing.T, b *Behavior)
	}{
		{
			name:      "empty string",
			input:     "",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Latency != nil {
					t.Error("expected nil latency")
				}
				if b.Error != nil {
					t.Error("expected nil error")
				}
			},
		},
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
		{
			name:      "cpu spike",
			input:     "cpu=spike",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.CPU == nil {
					t.Fatal("expected cpu behavior")
				}
				if b.CPU.Pattern != "spike" {
					t.Errorf("expected spike pattern, got %s", b.CPU.Pattern)
				}
			},
		},
		{
			name:      "memory leak",
			input:     "memory=leak-slow",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Memory == nil {
					t.Fatal("expected memory behavior")
				}
				if b.Memory.Pattern != "leak-slow" {
					t.Errorf("expected leak-slow pattern, got %s", b.Memory.Pattern)
				}
			},
		},
		{
			name:      "memory spike with size",
			input:     "memory=spike:500Mi",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Memory == nil {
					t.Fatal("expected memory behavior")
				}
				if b.Memory.Pattern != "spike" {
					t.Errorf("expected spike pattern, got %s", b.Memory.Pattern)
				}
				expectedAmount := int64(500 * 1024 * 1024)
				if b.Memory.Amount != expectedAmount {
					t.Errorf("expected amount %d, got %d", expectedAmount, b.Memory.Amount)
				}
			},
		},
		{
			name:      "memory spike with size and duration",
			input:     "memory=spike:1Gi:30s",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Memory == nil {
					t.Fatal("expected memory behavior")
				}
				if b.Memory.Pattern != "spike" {
					t.Errorf("expected spike pattern, got %s", b.Memory.Pattern)
				}
				expectedAmount := int64(1024 * 1024 * 1024)
				if b.Memory.Amount != expectedAmount {
					t.Errorf("expected amount %d, got %d", expectedAmount, b.Memory.Amount)
				}
				expectedDuration := 30 * time.Second
				if b.Memory.Duration != expectedDuration {
					t.Errorf("expected duration %s, got %s", expectedDuration, b.Memory.Duration)
				}
			},
		},
		{
			name:      "memory spike with percentage",
			input:     "memory=spike:80%",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Memory == nil {
					t.Fatal("expected memory behavior")
				}
				if b.Memory.Pattern != "spike" {
					t.Errorf("expected spike pattern, got %s", b.Memory.Pattern)
				}
				if b.Memory.Percentage != 80 {
					t.Errorf("expected percentage 80, got %d", b.Memory.Percentage)
				}
			},
		},
		{
			name:      "memory spike with percentage and duration",
			input:     "memory=spike:80%:60s",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Memory == nil {
					t.Fatal("expected memory behavior")
				}
				if b.Memory.Pattern != "spike" {
					t.Errorf("expected spike pattern, got %s", b.Memory.Pattern)
				}
				if b.Memory.Percentage != 80 {
					t.Errorf("expected percentage 80, got %d", b.Memory.Percentage)
				}
				expectedDuration := 60 * time.Second
				if b.Memory.Duration != expectedDuration {
					t.Errorf("expected duration %s, got %s", expectedDuration, b.Memory.Duration)
				}
			},
		},
		{
			name:      "memory spike without size",
			input:     "memory=spike",
			wantError: true,
		},
		{
			name:      "memory spike with invalid percentage",
			input:     "memory=spike:150%",
			wantError: true,
		},
		{
			name:      "combined behaviors",
			input:     "latency=100ms,error=500:0.5",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Latency == nil {
					t.Error("expected latency behavior")
				}
				if b.Error == nil {
					t.Error("expected error behavior")
				}
				if b.Latency.Value != 100*time.Millisecond {
					t.Errorf("expected 100ms latency, got %v", b.Latency.Value)
				}
				if b.Error.Prob != 0.5 {
					t.Errorf("expected prob 0.5, got %v", b.Error.Prob)
				}
				if b.Error.Rate != 500 {
					t.Errorf("expected rate 500, got %v", b.Error.Rate)
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

// TestParseChain tests behavior chain parsing with service targeting
func TestParseChain(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		validate  func(t *testing.T, bc *BehaviorChain)
	}{
		{
			name:      "empty chain",
			input:     "",
			wantError: false,
			validate: func(t *testing.T, bc *BehaviorChain) {
				if len(bc.Behaviors) != 0 {
					t.Errorf("expected 0 behaviors, got %d", len(bc.Behaviors))
				}
			},
		},
		{
			name:      "single global behavior",
			input:     "latency=100ms",
			wantError: false,
			validate: func(t *testing.T, bc *BehaviorChain) {
				if len(bc.Behaviors) != 1 {
					t.Fatalf("expected 1 behavior, got %d", len(bc.Behaviors))
				}
				if bc.Behaviors[0].Service != "" {
					t.Errorf("expected global behavior, got service=%s", bc.Behaviors[0].Service)
				}
				if bc.Behaviors[0].Behavior.Latency == nil {
					t.Error("expected latency in behavior")
				}
			},
		},
		{
			name:      "single service-targeted behavior",
			input:     "order-api:error=500:0.5",
			wantError: false,
			validate: func(t *testing.T, bc *BehaviorChain) {
				if len(bc.Behaviors) != 1 {
					t.Fatalf("expected 1 behavior, got %d", len(bc.Behaviors))
				}
				if bc.Behaviors[0].Service != "order-api" {
					t.Errorf("expected order-api, got %s", bc.Behaviors[0].Service)
				}
				if bc.Behaviors[0].Behavior.Error == nil {
					t.Fatal("expected error behavior")
				}
				if bc.Behaviors[0].Behavior.Error.Prob != 0.5 {
					t.Errorf("expected prob 0.5, got %v", bc.Behaviors[0].Behavior.Error.Prob)
				}
				if bc.Behaviors[0].Behavior.Error.Rate != 500 {
					t.Errorf("expected rate 500, got %v", bc.Behaviors[0].Behavior.Error.Rate)
				}
			},
		},
		{
			name:      "multiple service-targeted behaviors",
			input:     "order-api:error=500:0.5,product-api:latency=200ms",
			wantError: false,
			validate: func(t *testing.T, bc *BehaviorChain) {
				if len(bc.Behaviors) != 2 {
					t.Fatalf("expected 2 behaviors, got %d", len(bc.Behaviors))
				}
				// Check order-api
				if bc.Behaviors[0].Service != "order-api" {
					t.Errorf("expected order-api, got %s", bc.Behaviors[0].Service)
				}
				if bc.Behaviors[0].Behavior.Error == nil {
					t.Error("expected error in order-api")
				}
				// Check product-api
				if bc.Behaviors[1].Service != "product-api" {
					t.Errorf("expected product-api, got %s", bc.Behaviors[1].Service)
				}
				if bc.Behaviors[1].Behavior.Latency == nil {
					t.Error("expected latency in product-api")
				}
			},
		},
		{
			name:      "service with chained behaviors",
			input:     "order-api:error=0.5,latency=50ms",
			wantError: false,
			validate: func(t *testing.T, bc *BehaviorChain) {
				if len(bc.Behaviors) != 1 {
					t.Fatalf("expected 1 behavior (both apply to order-api), got %d", len(bc.Behaviors))
				}
				// Should be order-api with both error and latency
				if bc.Behaviors[0].Service != "order-api" {
					t.Errorf("expected order-api, got %s", bc.Behaviors[0].Service)
				}
				if bc.Behaviors[0].Behavior.Error == nil {
					t.Error("expected error in order-api")
				}
				if bc.Behaviors[0].Behavior.Latency == nil {
					t.Error("expected latency in order-api")
				}
			},
		},
		{
			name:      "service with multiple behaviors combined",
			input:     "order-api:error=500:0.5,latency=100ms",
			wantError: false,
			validate: func(t *testing.T, bc *BehaviorChain) {
				if len(bc.Behaviors) != 1 {
					t.Fatalf("expected 1 behavior, got %d", len(bc.Behaviors))
				}
				if bc.Behaviors[0].Service != "order-api" {
					t.Errorf("expected order-api, got %s", bc.Behaviors[0].Service)
				}
				b := bc.Behaviors[0].Behavior
				if b.Error == nil {
					t.Error("expected error behavior")
				}
				if b.Latency == nil {
					t.Error("expected latency behavior")
				}
			},
		},
		{
			name:      "complex chain with multiple services",
			input:     "order-api:error=500:0.3,product-api:latency=100-200ms,payment-api:error=0.1,latency=50ms",
			wantError: false,
			validate: func(t *testing.T, bc *BehaviorChain) {
				if len(bc.Behaviors) != 3 {
					t.Fatalf("expected 3 behaviors, got %d", len(bc.Behaviors))
				}

				// Check order-api
				if bc.Behaviors[0].Service != "order-api" {
					t.Errorf("expected order-api, got %s", bc.Behaviors[0].Service)
				}
				if bc.Behaviors[0].Behavior.Error == nil {
					t.Error("expected error in order-api")
				}

				// Check product-api
				if bc.Behaviors[1].Service != "product-api" {
					t.Errorf("expected product-api, got %s", bc.Behaviors[1].Service)
				}
				if bc.Behaviors[1].Behavior.Latency == nil {
					t.Error("expected latency in product-api")
				}

				// Check payment-api (has both error and latency)
				if bc.Behaviors[2].Service != "payment-api" {
					t.Errorf("expected payment-api, got %s", bc.Behaviors[2].Service)
				}
				if bc.Behaviors[2].Behavior.Error == nil {
					t.Error("expected error in payment-api")
				}
				if bc.Behaviors[2].Behavior.Latency == nil {
					t.Error("expected latency in payment-api (chained)")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc, err := ParseChain(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ParseChain() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && tt.validate != nil {
				tt.validate(t, bc)
			}
		})
	}
}

// TestBehaviorChainForService tests the ForService method
func TestBehaviorChainForService(t *testing.T) {
	tests := []struct {
		name        string
		chain       string
		serviceName string
		validate    func(t *testing.T, b *Behavior)
	}{
		{
			name:        "specific behavior for service",
			chain:       "order-api:error=0.5,product-api:latency=100ms",
			serviceName: "order-api",
			validate: func(t *testing.T, b *Behavior) {
				if b == nil {
					t.Fatal("expected behavior for order-api")
				}
				if b.Error == nil {
					t.Error("expected error behavior")
				}
				if b.Latency != nil {
					t.Error("expected no latency behavior")
				}
			},
		},
		{
			name:        "global behavior applied to service",
			chain:       "latency=50ms",
			serviceName: "any-service",
			validate: func(t *testing.T, b *Behavior) {
				if b == nil {
					t.Fatal("expected global behavior")
				}
				if b.Latency == nil {
					t.Error("expected latency behavior")
				}
			},
		},
		{
			name:        "service with multiple behaviors in sequence",
			chain:       "order-api:error=0.5,latency=100ms",
			serviceName: "order-api",
			validate: func(t *testing.T, b *Behavior) {
				if b == nil {
					t.Fatal("expected behavior")
				}
				if b.Error == nil {
					t.Error("expected error from specific behavior")
				}
				if b.Latency == nil {
					t.Error("expected latency to be part of order-api behavior")
				}
			},
		},
		{
			name:        "no behavior for unmatched service",
			chain:       "order-api:error=0.5",
			serviceName: "product-api",
			validate: func(t *testing.T, b *Behavior) {
				if b != nil {
					t.Error("expected nil behavior for unmatched service")
				}
			},
		},
		{
			name:        "specific overrides global latency",
			chain:       "latency=100ms,order-api:latency=50ms",
			serviceName: "order-api",
			validate: func(t *testing.T, b *Behavior) {
				if b == nil {
					t.Fatal("expected behavior")
				}
				if b.Latency == nil {
					t.Error("expected latency")
				}
				if b.Latency.Value != 50*time.Millisecond {
					t.Errorf("expected specific 50ms, got %v", b.Latency.Value)
				}
			},
		},
		{
			name:        "global applied when no specific match",
			chain:       "latency=50ms,order-api:error=0.5",
			serviceName: "product-api",
			validate: func(t *testing.T, b *Behavior) {
				if b == nil {
					t.Fatal("expected global behavior")
				}
				if b.Latency == nil {
					t.Error("expected latency from global")
				}
				if b.Error != nil {
					t.Error("should not have error from order-api specific")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc, err := ParseChain(tt.chain)
			if err != nil {
				t.Fatalf("ParseChain() failed: %v", err)
			}
			b := bc.ForService(tt.serviceName)
			tt.validate(t, b)
		})
	}
}

// TestBehaviorString tests the String() method for round-trip conversion
func TestBehaviorString(t *testing.T) {
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
			name:     "error with default code",
			input:    "error=0.5",
			expected: "error=0.5",
		},
		{
			name:     "error with custom code",
			input:    "error=503:0.5",
			expected: "error=503:0.5",
		},
		{
			name:     "combined behaviors",
			input:    "latency=100ms,error=500:0.3",
			expected: "latency=100ms,error=0.3",
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

// TestBehaviorChainString tests the chain String() method
func TestBehaviorChainString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single global behavior",
			input:    "latency=100ms",
			expected: "latency=100ms",
		},
		{
			name:     "single targeted behavior",
			input:    "order-api:error=500:0.5",
			expected: "order-api:error=0.5",
		},
		{
			name:     "multiple targeted behaviors",
			input:    "order-api:error=0.5,product-api:latency=100ms",
			expected: "order-api:error=0.5,product-api:latency=100ms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc, err := ParseChain(tt.input)
			if err != nil {
				t.Fatalf("ParseChain() failed: %v", err)
			}
			result := bc.String()
			if result != tt.expected {
				t.Errorf("String() = %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestShouldError tests error probability logic
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
			input:          "error=0.5,code=500",
			iterations:     1000,
			expectedRate:   0.5,
			toleranceRange: 0.1, // 10% tolerance
		},
		{
			name:           "10% error rate",
			input:          "error=0.1,code=503",
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

// TestApplyBehavior tests behavior application
func TestApplyBehavior(t *testing.T) {
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

// TestGetAppliedBehaviors tests behavior reporting
func TestGetAppliedBehaviors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "latency only",
			input:    "latency=100ms",
			expected: []string{"latency:fixed:100ms"},
		},
		{
			name:     "error only",
			input:    "error=500:0.5",
			expected: []string{"error:500:0.50"},
		},
		{
			name:     "combined behaviors",
			input:    "latency=100ms,error=503:0.3",
			expected: []string{"latency:fixed:100ms", "error:503:0.30"},
		},
		{
			name:     "cpu behavior",
			input:    "cpu=spike",
			expected: []string{"cpu:spike:5s:intensity=80"},
		},
		{
			name:     "memory behavior",
			input:    "memory=leak-slow",
			expected: []string{"memory:leak-slow:10485760:10m0s"},
		},
		{
			name:     "custom parameters",
			input:    "latency=100ms,foo=bar,baz=qux",
			expected: []string{"latency:fixed:100ms", "custom:foo=bar", "custom:baz=qux"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() failed: %v", err)
			}

			applied := b.GetAppliedBehaviors()
			if len(applied) != len(tt.expected) {
				t.Fatalf("got %d behaviors, want %d\nGot: %v\nWant: %v", len(applied), len(tt.expected), applied, tt.expected)
			}

			// For custom params, order might vary (map iteration), so check existence
			if tt.name == "custom parameters" {
				expectedMap := make(map[string]bool)
				for _, e := range tt.expected {
					expectedMap[e] = true
				}
				for _, a := range applied {
					if !expectedMap[a] {
						t.Errorf("unexpected behavior: %s", a)
					}
				}
			} else {
				for i, want := range tt.expected {
					if applied[i] != want {
						t.Errorf("behavior[%d] = %s, want %s", i, applied[i], want)
					}
				}
			}
		})
	}
}

// TestCustomParameters tests custom parameter parsing and serialization
func TestCustomParameters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, b *Behavior)
	}{
		{
			name:  "single custom parameter",
			input: "foo=bar",
			validate: func(t *testing.T, b *Behavior) {
				if len(b.CustomParams) != 1 {
					t.Errorf("expected 1 custom param, got %d", len(b.CustomParams))
				}
				if b.CustomParams["foo"] != "bar" {
					t.Errorf("expected foo=bar, got foo=%s", b.CustomParams["foo"])
				}
			},
		},
		{
			name:  "multiple custom parameters",
			input: "foo=bar,baz=qux",
			validate: func(t *testing.T, b *Behavior) {
				if len(b.CustomParams) != 2 {
					t.Errorf("expected 2 custom params, got %d", len(b.CustomParams))
				}
				if b.CustomParams["foo"] != "bar" {
					t.Errorf("expected foo=bar, got foo=%s", b.CustomParams["foo"])
				}
				if b.CustomParams["baz"] != "qux" {
					t.Errorf("expected baz=qux, got baz=%s", b.CustomParams["baz"])
				}
			},
		},
		{
			name:  "mixed standard and custom",
			input: "latency=100ms,custom1=value1,error=0.5,custom2=value2",
			validate: func(t *testing.T, b *Behavior) {
				if b.Latency == nil {
					t.Error("expected latency")
				}
				if b.Error == nil {
					t.Error("expected error")
				}
				if len(b.CustomParams) != 2 {
					t.Errorf("expected 2 custom params, got %d", len(b.CustomParams))
				}
				if b.CustomParams["custom1"] != "value1" {
					t.Errorf("expected custom1=value1")
				}
				if b.CustomParams["custom2"] != "value2" {
					t.Errorf("expected custom2=value2")
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
			tt.validate(t, b)

			// Test round-trip via String()
			serialized := b.String()
			b2, err := Parse(serialized)
			if err != nil {
				t.Fatalf("Round-trip Parse() failed: %v", err)
			}

			// Verify custom params survived round-trip
			if len(b2.CustomParams) != len(b.CustomParams) {
				t.Errorf("round-trip custom params count: got %d, want %d", len(b2.CustomParams), len(b.CustomParams))
			}
			for k, v := range b.CustomParams {
				if b2.CustomParams[k] != v {
					t.Errorf("round-trip custom param %s: got %s, want %s", k, b2.CustomParams[k], v)
				}
			}
		})
	}
}

// TestFullRoundTrip tests complete round-trip serialization/deserialization
// for all behavior types including their specific fields
func TestFullRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, b1, b2 *Behavior)
	}{
		{
			name:  "error with custom code",
			input: "error=503:0.5",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.Error == nil || b2.Error == nil {
					t.Fatal("error behavior is nil")
				}
				if b1.Error.Rate != b2.Error.Rate {
					t.Errorf("error rate: got %d, want %d", b2.Error.Rate, b1.Error.Rate)
				}
				if b1.Error.Prob != b2.Error.Prob {
					t.Errorf("error prob: got %v, want %v", b2.Error.Prob, b1.Error.Prob)
				}
			},
		},
		{
			name:  "error with default code",
			input: "error=0.5",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.Error == nil || b2.Error == nil {
					t.Fatal("error behavior is nil")
				}
				if b1.Error.Rate != b2.Error.Rate {
					t.Errorf("error rate: got %d, want %d", b2.Error.Rate, b1.Error.Rate)
				}
				if b1.Error.Prob != b2.Error.Prob {
					t.Errorf("error prob: got %v, want %v", b2.Error.Prob, b1.Error.Prob)
				}
			},
		},
		{
			name:  "memory with byte amount Mi",
			input: "memory=10Mi",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.Memory == nil || b2.Memory == nil {
					t.Fatal("memory behavior is nil")
				}
				if b1.Memory.Amount != b2.Memory.Amount {
					t.Errorf("memory amount: got %d, want %d", b2.Memory.Amount, b1.Memory.Amount)
				}
				if b1.Memory.Pattern != b2.Memory.Pattern {
					t.Errorf("memory pattern: got %s, want %s", b2.Memory.Pattern, b1.Memory.Pattern)
				}
			},
		},
		{
			name:  "memory with byte amount Gi",
			input: "memory=1Gi",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.Memory == nil || b2.Memory == nil {
					t.Fatal("memory behavior is nil")
				}
				if b1.Memory.Amount != b2.Memory.Amount {
					t.Errorf("memory amount: got %d, want %d", b2.Memory.Amount, b1.Memory.Amount)
				}
			},
		},
		{
			name:  "memory leak pattern",
			input: "memory=leak-slow:5m",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.Memory == nil || b2.Memory == nil {
					t.Fatal("memory behavior is nil")
				}
				if b1.Memory.Pattern != b2.Memory.Pattern {
					t.Errorf("memory pattern: got %s, want %s", b2.Memory.Pattern, b1.Memory.Pattern)
				}
				if b1.Memory.Duration != b2.Memory.Duration {
					t.Errorf("memory duration: got %s, want %s", b2.Memory.Duration, b1.Memory.Duration)
				}
			},
		},
		{
			name:  "memory spike with size",
			input: "memory=spike:500Mi",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.Memory == nil || b2.Memory == nil {
					t.Fatal("memory behavior is nil")
				}
				if b1.Memory.Pattern != b2.Memory.Pattern {
					t.Errorf("memory pattern: got %s, want %s", b2.Memory.Pattern, b1.Memory.Pattern)
				}
				if b1.Memory.Amount != b2.Memory.Amount {
					t.Errorf("memory amount: got %d, want %d", b2.Memory.Amount, b1.Memory.Amount)
				}
			},
		},
		{
			name:  "memory spike with size and duration",
			input: "memory=spike:1Gi:30s",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.Memory == nil || b2.Memory == nil {
					t.Fatal("memory behavior is nil")
				}
				if b1.Memory.Pattern != b2.Memory.Pattern {
					t.Errorf("memory pattern: got %s, want %s", b2.Memory.Pattern, b1.Memory.Pattern)
				}
				if b1.Memory.Amount != b2.Memory.Amount {
					t.Errorf("memory amount: got %d, want %d", b2.Memory.Amount, b1.Memory.Amount)
				}
				if b1.Memory.Duration != b2.Memory.Duration {
					t.Errorf("memory duration: got %s, want %s", b2.Memory.Duration, b1.Memory.Duration)
				}
			},
		},
		{
			name:  "memory spike with percentage",
			input: "memory=spike:80%:1m",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.Memory == nil || b2.Memory == nil {
					t.Fatal("memory behavior is nil")
				}
				if b1.Memory.Pattern != b2.Memory.Pattern {
					t.Errorf("memory pattern: got %s, want %s", b2.Memory.Pattern, b1.Memory.Pattern)
				}
				if b1.Memory.Percentage != b2.Memory.Percentage {
					t.Errorf("memory percentage: got %d, want %d", b2.Memory.Percentage, b1.Memory.Percentage)
				}
				if b1.Memory.Duration != b2.Memory.Duration {
					t.Errorf("memory duration: got %s, want %s", b2.Memory.Duration, b1.Memory.Duration)
				}
			},
		},
		{
			name:  "cpu with explicit intensity",
			input: "cpu=spike:5s:80",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.CPU == nil || b2.CPU == nil {
					t.Fatal("cpu behavior is nil")
				}
				if b1.CPU.Pattern != b2.CPU.Pattern {
					t.Errorf("cpu pattern: got %s, want %s", b2.CPU.Pattern, b1.CPU.Pattern)
				}
				if b1.CPU.Duration != b2.CPU.Duration {
					t.Errorf("cpu duration: got %s, want %s", b2.CPU.Duration, b1.CPU.Duration)
				}
				if b1.CPU.Intensity != b2.CPU.Intensity {
					t.Errorf("cpu intensity: got %d, want %d", b2.CPU.Intensity, b1.CPU.Intensity)
				}
			},
		},
		{
			name:  "cpu with different intensity",
			input: "cpu=steady:10s:50",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.CPU == nil || b2.CPU == nil {
					t.Fatal("cpu behavior is nil")
				}
				if b1.CPU.Intensity != b2.CPU.Intensity {
					t.Errorf("cpu intensity: got %d, want %d", b2.CPU.Intensity, b1.CPU.Intensity)
				}
			},
		},
		{
			name:  "latency fixed",
			input: "latency=100ms",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.Latency == nil || b2.Latency == nil {
					t.Fatal("latency behavior is nil")
				}
				if b1.Latency.Type != b2.Latency.Type {
					t.Errorf("latency type: got %s, want %s", b2.Latency.Type, b1.Latency.Type)
				}
				if b1.Latency.Value != b2.Latency.Value {
					t.Errorf("latency value: got %s, want %s", b2.Latency.Value, b1.Latency.Value)
				}
			},
		},
		{
			name:  "latency range",
			input: "latency=50ms-200ms",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.Latency == nil || b2.Latency == nil {
					t.Fatal("latency behavior is nil")
				}
				if b1.Latency.Type != b2.Latency.Type {
					t.Errorf("latency type: got %s, want %s", b2.Latency.Type, b1.Latency.Type)
				}
				if b1.Latency.Min != b2.Latency.Min {
					t.Errorf("latency min: got %s, want %s", b2.Latency.Min, b1.Latency.Min)
				}
				if b1.Latency.Max != b2.Latency.Max {
					t.Errorf("latency max: got %s, want %s", b2.Latency.Max, b1.Latency.Max)
				}
			},
		},
		{
			name:  "combined behaviors",
			input: "latency=100ms,error=503:0.5,cpu=spike:5s:80,memory=10Mi",
			validate: func(t *testing.T, b1, b2 *Behavior) {
				if b1.Latency == nil || b2.Latency == nil {
					t.Fatal("latency behavior is nil")
				}
				if b1.Error == nil || b2.Error == nil {
					t.Fatal("error behavior is nil")
				}
				if b1.CPU == nil || b2.CPU == nil {
					t.Fatal("cpu behavior is nil")
				}
				if b1.Memory == nil || b2.Memory == nil {
					t.Fatal("memory behavior is nil")
				}

				// Validate all fields
				if b1.Error.Rate != b2.Error.Rate {
					t.Errorf("error rate: got %d, want %d", b2.Error.Rate, b1.Error.Rate)
				}
				if b1.Memory.Amount != b2.Memory.Amount {
					t.Errorf("memory amount: got %d, want %d", b2.Memory.Amount, b1.Memory.Amount)
				}
				if b1.CPU.Intensity != b2.CPU.Intensity {
					t.Errorf("cpu intensity: got %d, want %d", b2.CPU.Intensity, b1.CPU.Intensity)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse input
			b1, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() failed: %v", err)
			}

			// Serialize
			serialized := b1.String()
			t.Logf("Original: %s", tt.input)
			t.Logf("Serialized: %s", serialized)

			// Re-parse
			b2, err := Parse(serialized)
			if err != nil {
				t.Fatalf("Round-trip Parse() failed on %q: %v", serialized, err)
			}

			// Validate specific fields
			tt.validate(t, b1, b2)
		})
	}
}

// TestBehaviorChainRoundTrip tests complete round-trip for behavior chains
func TestBehaviorChainRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "single service with error",
			input: "order-api:error=503:0.5",
		},
		{
			name:  "multiple services",
			input: "order-api:error=503:0.5,product-api:latency=100ms",
		},
		{
			name:  "global and specific",
			input: "latency=50ms,order-api:error=0.5",
		},
		{
			name:  "complex behaviors",
			input: "order-api:latency=100ms,error=503:0.3,cpu=spike:5s:80,product-api:memory=10Mi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse chain
			bc1, err := ParseChain(tt.input)
			if err != nil {
				t.Fatalf("ParseChain() failed: %v", err)
			}

			// Serialize
			serialized := bc1.String()
			t.Logf("Original: %s", tt.input)
			t.Logf("Serialized: %s", serialized)

			// Re-parse
			bc2, err := ParseChain(serialized)
			if err != nil {
				t.Fatalf("Round-trip ParseChain() failed on %q: %v", serialized, err)
			}

			// Compare behavior counts
			if len(bc1.Behaviors) != len(bc2.Behaviors) {
				t.Fatalf("behavior count mismatch: got %d, want %d", len(bc2.Behaviors), len(bc1.Behaviors))
			}

			// Compare each service behavior
			for i := range bc1.Behaviors {
				sb1 := bc1.Behaviors[i]
				sb2 := bc2.Behaviors[i]

				if sb1.Service != sb2.Service {
					t.Errorf("behavior[%d] service: got %q, want %q", i, sb2.Service, sb1.Service)
				}

				// Validate behavior fields
				if sb1.Behavior.Latency != nil && sb2.Behavior.Latency != nil {
					if sb1.Behavior.Latency.Type != sb2.Behavior.Latency.Type {
						t.Errorf("behavior[%d] latency type mismatch", i)
					}
				}
				if sb1.Behavior.Error != nil && sb2.Behavior.Error != nil {
					if sb1.Behavior.Error.Rate != sb2.Behavior.Error.Rate {
						t.Errorf("behavior[%d] error rate: got %d, want %d", i, sb2.Behavior.Error.Rate, sb1.Behavior.Error.Rate)
					}
				}
				if sb1.Behavior.CPU != nil && sb2.Behavior.CPU != nil {
					if sb1.Behavior.CPU.Intensity != sb2.Behavior.CPU.Intensity {
						t.Errorf("behavior[%d] cpu intensity: got %d, want %d", i, sb2.Behavior.CPU.Intensity, sb1.Behavior.CPU.Intensity)
					}
				}
				if sb1.Behavior.Memory != nil && sb2.Behavior.Memory != nil {
					if sb1.Behavior.Memory.Amount != sb2.Behavior.Memory.Amount {
						t.Errorf("behavior[%d] memory amount: got %d, want %d", i, sb2.Behavior.Memory.Amount, sb1.Behavior.Memory.Amount)
					}
				}
			}
		})
	}
}

// TestParseCrashIfFile_SingleCondition tests parsing a single invalid content condition
func TestParseCrashIfFile_SingleCondition(t *testing.T) {
	result, err := parseCrashIfFile("/config/app.conf:invalid")
	if err != nil {
		t.Fatalf("parseCrashIfFile() failed: %v", err)
	}

	if result.FilePath != "/config/app.conf" {
		t.Errorf("FilePath: got %q, want %q", result.FilePath, "/config/app.conf")
	}

	if len(result.InvalidContent) != 1 {
		t.Fatalf("InvalidContent length: got %d, want 1", len(result.InvalidContent))
	}

	if result.InvalidContent[0] != "invalid" {
		t.Errorf("InvalidContent[0]: got %q, want %q", result.InvalidContent[0], "invalid")
	}
}

// TestParseCrashIfFile_MultipleConditions tests parsing multiple invalid content conditions
func TestParseCrashIfFile_MultipleConditions(t *testing.T) {
	result, err := parseCrashIfFile("/config/db.conf:bad;error;fail")
	if err != nil {
		t.Fatalf("parseCrashIfFile() failed: %v", err)
	}

	if result.FilePath != "/config/db.conf" {
		t.Errorf("FilePath: got %q, want %q", result.FilePath, "/config/db.conf")
	}

	if len(result.InvalidContent) != 3 {
		t.Fatalf("InvalidContent length: got %d, want 3", len(result.InvalidContent))
	}

	expected := []string{"bad", "error", "fail"}
	for i, want := range expected {
		if result.InvalidContent[i] != want {
			t.Errorf("InvalidContent[%d]: got %q, want %q", i, result.InvalidContent[i], want)
		}
	}
}

// TestParseCrashIfFile_InvalidFormat tests error handling for malformed input
func TestParseCrashIfFile_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no colon", "/config/app.conf"},
		{"empty path", ":invalid"},
		{"empty content", "/config/app.conf:"},
		{"only whitespace content", "/config/app.conf:   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCrashIfFile(tt.input)
			if err == nil {
				t.Errorf("parseCrashIfFile(%q) expected error, got nil", tt.input)
			}
		})
	}
}

// TestParseBehavior_CrashIfFile tests integration with Parse() function
func TestParseBehavior_CrashIfFile(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantFilePath string
		wantContent  []string
	}{
		{
			name:         "single crash-if-file",
			input:        "crash-if-file=/config/app.conf:invalid",
			wantFilePath: "/config/app.conf",
			wantContent:  []string{"invalid"},
		},
		{
			name:         "crash-if-file with multiple conditions",
			input:        "crash-if-file=/etc/config:bad;error",
			wantFilePath: "/etc/config",
			wantContent:  []string{"bad", "error"},
		},
		{
			name:         "combined with other behaviors",
			input:        "latency=100ms,crash-if-file=/config/db:fail,error=0.1",
			wantFilePath: "/config/db",
			wantContent:  []string{"fail"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() failed: %v", err)
			}

			if b.CrashIfFile == nil {
				t.Fatal("CrashIfFile is nil")
			}

			if b.CrashIfFile.FilePath != tt.wantFilePath {
				t.Errorf("FilePath: got %q, want %q", b.CrashIfFile.FilePath, tt.wantFilePath)
			}

			if len(b.CrashIfFile.InvalidContent) != len(tt.wantContent) {
				t.Fatalf("InvalidContent length: got %d, want %d", len(b.CrashIfFile.InvalidContent), len(tt.wantContent))
			}

			for i, want := range tt.wantContent {
				if b.CrashIfFile.InvalidContent[i] != want {
					t.Errorf("InvalidContent[%d]: got %q, want %q", i, b.CrashIfFile.InvalidContent[i], want)
				}
			}
		})
	}
}

// TestShouldCrashOnFile_MatchFound tests that crash is triggered when file contains invalid content
func TestShouldCrashOnFile_MatchFound(t *testing.T) {
	// Create temp file with invalid content
	tmpFile, err := os.CreateTemp("", "test-config-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "database_url=invalid\nother_setting=value"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	b := &Behavior{
		CrashIfFile: &CrashIfFileBehavior{
			FilePath:       tmpFile.Name(),
			InvalidContent: []string{"invalid"},
		},
	}

	shouldCrash, matched, msg := b.ShouldCrashOnFile()
	if !shouldCrash {
		t.Error("Expected shouldCrash to be true")
	}
	if matched != "invalid" {
		t.Errorf("Matched: got %q, want %q", matched, "invalid")
	}
	if !strings.Contains(msg, tmpFile.Name()) {
		t.Errorf("Message should contain file path: %s", msg)
	}
	if !strings.Contains(msg, "invalid") {
		t.Errorf("Message should contain matched content: %s", msg)
	}
}

// TestShouldCrashOnFile_NoMatch tests that no crash occurs when file is valid
func TestShouldCrashOnFile_NoMatch(t *testing.T) {
	// Create temp file with valid content
	tmpFile, err := os.CreateTemp("", "test-config-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "database_url=valid\nother_setting=value"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	b := &Behavior{
		CrashIfFile: &CrashIfFileBehavior{
			FilePath:       tmpFile.Name(),
			InvalidContent: []string{"invalid", "bad"},
		},
	}

	shouldCrash, matched, msg := b.ShouldCrashOnFile()
	if shouldCrash {
		t.Error("Expected shouldCrash to be false")
	}
	if matched != "" {
		t.Errorf("Expected empty matched string, got %q", matched)
	}
	if msg != "" {
		t.Errorf("Expected empty message, got %q", msg)
	}
}

// TestShouldCrashOnFile_FileNotFound tests graceful handling of missing file
func TestShouldCrashOnFile_FileNotFound(t *testing.T) {
	b := &Behavior{
		CrashIfFile: &CrashIfFileBehavior{
			FilePath:       "/nonexistent/config.conf",
			InvalidContent: []string{"invalid"},
		},
	}

	shouldCrash, matched, msg := b.ShouldCrashOnFile()
	if shouldCrash {
		t.Error("Expected shouldCrash to be false for missing file")
	}
	if matched != "" {
		t.Errorf("Expected empty matched string, got %q", matched)
	}
	if !strings.Contains(msg, "failed to read file") {
		t.Errorf("Expected error message about file read failure, got: %s", msg)
	}
}

// TestShouldCrashOnFile_MultipleInvalidStrings tests checking multiple invalid strings
func TestShouldCrashOnFile_MultipleInvalidStrings(t *testing.T) {
	// Create temp file with one of the invalid strings
	tmpFile, err := os.CreateTemp("", "test-config-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "database_url=error\nother_setting=value"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	b := &Behavior{
		CrashIfFile: &CrashIfFileBehavior{
			FilePath:       tmpFile.Name(),
			InvalidContent: []string{"invalid", "error", "bad"},
		},
	}

	shouldCrash, matched, msg := b.ShouldCrashOnFile()
	if !shouldCrash {
		t.Error("Expected shouldCrash to be true")
	}
	if matched != "error" {
		t.Errorf("Matched: got %q, want %q", matched, "error")
	}
	if !strings.Contains(msg, "error") {
		t.Errorf("Message should contain matched content: %s", msg)
	}
}

// TestBehavior_String_CrashIfFile tests serialization of crash-if-file behavior
func TestBehavior_String_CrashIfFile(t *testing.T) {
	tests := []struct {
		name     string
		behavior *Behavior
		want     string
	}{
		{
			name: "single invalid content",
			behavior: &Behavior{
				CrashIfFile: &CrashIfFileBehavior{
					FilePath:       "/config/app.conf",
					InvalidContent: []string{"invalid"},
				},
			},
			want: "crash-if-file=/config/app.conf:invalid",
		},
		{
			name: "multiple invalid content",
			behavior: &Behavior{
				CrashIfFile: &CrashIfFileBehavior{
					FilePath:       "/config/db.conf",
					InvalidContent: []string{"bad", "error"},
				},
			},
			want: "crash-if-file=/config/db.conf:bad;error",
		},
		{
			name: "combined with latency",
			behavior: &Behavior{
				Latency: &LatencyBehavior{
					Type:  "fixed",
					Value: 100 * time.Millisecond,
				},
				CrashIfFile: &CrashIfFileBehavior{
					FilePath:       "/etc/config",
					InvalidContent: []string{"fail"},
				},
			},
			want: "latency=100ms,crash-if-file=/etc/config:fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.behavior.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBehavior_GetAppliedBehaviors_CrashIfFile tests that crash-if-file appears in applied list
func TestBehavior_GetAppliedBehaviors_CrashIfFile(t *testing.T) {
	b := &Behavior{
		CrashIfFile: &CrashIfFileBehavior{
			FilePath:       "/config/app.conf",
			InvalidContent: []string{"invalid", "bad"},
		},
	}

	applied := b.GetAppliedBehaviors()
	found := false
	for _, a := range applied {
		if strings.Contains(a, "crash-if-file") {
			found = true
			if !strings.Contains(a, "/config/app.conf") {
				t.Errorf("Applied behavior should contain file path: %s", a)
			}
			if !strings.Contains(a, "invalid") {
				t.Errorf("Applied behavior should contain invalid content: %s", a)
			}
		}
	}
	if !found {
		t.Error("crash-if-file not found in applied behaviors")
	}
}

// TestMergeBehaviors_CrashIfFile tests merging behaviors with crash-if-file
func TestMergeBehaviors_CrashIfFile(t *testing.T) {
	b1 := &Behavior{
		Latency: &LatencyBehavior{
			Type:  "fixed",
			Value: 100 * time.Millisecond,
		},
		CrashIfFile: &CrashIfFileBehavior{
			FilePath:       "/config/app.conf",
			InvalidContent: []string{"invalid"},
		},
		CustomParams: make(map[string]string),
	}

	b2 := &Behavior{
		Error: &ErrorBehavior{
			Rate: 503,
			Prob: 0.5,
		},
		CrashIfFile: &CrashIfFileBehavior{
			FilePath:       "/config/db.conf",
			InvalidContent: []string{"bad"},
		},
		CustomParams: make(map[string]string),
	}

	merged := mergeBehaviors(b1, b2)

	// b2's CrashIfFile should override b1's
	if merged.CrashIfFile == nil {
		t.Fatal("merged.CrashIfFile is nil")
	}
	if merged.CrashIfFile.FilePath != "/config/db.conf" {
		t.Errorf("FilePath: got %q, want %q", merged.CrashIfFile.FilePath, "/config/db.conf")
	}

	// Other behaviors should be preserved
	if merged.Latency == nil {
		t.Error("Latency should be preserved from b1")
	}
	if merged.Error == nil {
		t.Error("Error should be preserved from b2")
	}
}
