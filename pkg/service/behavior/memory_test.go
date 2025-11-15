package behavior

import (
	"testing"
	"time"
)

func TestParseMemory(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		validate  func(t *testing.T, b *Behavior)
	}{
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

