package behavior

import (
	"testing"
	"time"
)

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
			name:        "specific overrides global",
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
			name:     "error only",
			input:    "error=0.5",
			expected: "error=500:0.5",
		},
		{
			name:     "latency and error",
			input:    "latency=100ms,error=500:0.5",
			expected: "latency=100ms,error=500:0.5",
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

func TestBehaviorChainString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "global behavior",
			input:    "latency=100ms",
			expected: "latency=100ms",
		},
		{
			name:     "service-targeted",
			input:    "order-api:latency=100ms",
			expected: "order-api:latency=100ms",
		},
		{
			name:     "complex chain",
			input:    "order-api:error=500:0.5,latency=100ms",
			expected: "order-api:latency=100ms,error=500:0.5",
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

func TestBehaviorChainRoundTrip(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"latency=100ms", "latency=100ms"},
		{"error=503:0.5", "error=503:0.5"},
		{"latency=50ms-200ms,error=0.1", "latency=50ms-200ms,error=500:0.1"},
		{"order-api:latency=100ms", "order-api:latency=100ms"},
		{"order-api:error=500:0.5,product-api:latency=200ms", "order-api:error=500:0.5,product-api:latency=200ms"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			bc, err := ParseChain(tt.input)
			if err != nil {
				t.Fatalf("ParseChain() failed: %v", err)
			}
			output := bc.String()
			if output != tt.expected {
				t.Errorf("Round trip failed: input=%s, expected=%s, output=%s", tt.input, tt.expected, output)
			}
		})
	}
}

func TestMergeBehaviors(t *testing.T) {
	b1, _ := Parse("latency=100ms,error=0.5")
	b2, _ := Parse("latency=50ms,cpu=spike")

	merged := mergeBehaviors(b1, b2)

	// b2 latency should override b1
	if merged.Latency.Value != 50*time.Millisecond {
		t.Errorf("expected 50ms latency, got %v", merged.Latency.Value)
	}

	// b1 error should remain (no error in b2)
	if merged.Error == nil || merged.Error.Prob != 0.5 {
		t.Error("expected error from b1 to remain")
	}

	// b2 cpu should be present
	if merged.CPU == nil {
		t.Error("expected cpu from b2")
	}
}

