package behavior

import (
	"context"
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
			expected: "error=0.5,code=503",
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
