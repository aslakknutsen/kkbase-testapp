package behavior

import (
	"testing"
)

func TestParsePanic(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		validate  func(t *testing.T, b *Behavior)
	}{
		{
			name:      "panic with probability",
			input:     "panic=0.5",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Panic == nil {
					t.Fatal("expected panic behavior")
				}
				if b.Panic.Prob != 0.5 {
					t.Errorf("expected prob 0.5, got %v", b.Panic.Prob)
				}
			},
		},
		{
			name:      "panic with 100% probability",
			input:     "panic=1.0",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Panic == nil {
					t.Fatal("expected panic behavior")
				}
				if b.Panic.Prob != 1.0 {
					t.Errorf("expected prob 1.0, got %v", b.Panic.Prob)
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

func TestPanicString(t *testing.T) {
	b, err := Parse("panic=0.5")
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	result := b.String()
	expected := "panic=0.5"
	if result != expected {
		t.Errorf("String() = %s, want %s", result, expected)
	}
}

