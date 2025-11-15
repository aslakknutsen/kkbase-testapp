package behavior

import (
	"testing"
	"time"
)

func TestParseCPU(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		validate  func(t *testing.T, b *Behavior)
	}{
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
			name:      "cpu with duration",
			input:     "cpu=steady:10s:50",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.CPU == nil {
					t.Fatal("expected cpu behavior")
				}
				if b.CPU.Pattern != "steady" {
					t.Errorf("expected steady pattern, got %s", b.CPU.Pattern)
				}
				if b.CPU.Duration != 10*time.Second {
					t.Errorf("expected 10s duration, got %v", b.CPU.Duration)
				}
				if b.CPU.Intensity != 50 {
					t.Errorf("expected intensity 50, got %d", b.CPU.Intensity)
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

func TestCPUString(t *testing.T) {
	b, err := Parse("cpu=spike:5s:80")
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	result := b.String()
	expected := "cpu=spike:5s:80"
	if result != expected {
		t.Errorf("String() = %s, want %s", result, expected)
	}
}

