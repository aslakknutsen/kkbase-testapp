package behavior

import (
	"testing"
	"time"
)

func TestParseDisk(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		validate  func(t *testing.T, b *Behavior)
	}{
		{
			name:      "disk with all params",
			input:     "disk=fill:500Mi:/cache:10m",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Disk == nil {
					t.Fatal("expected disk behavior")
				}
				if b.Disk.Size != 500*1024*1024 {
					t.Errorf("Size = %d, want %d", b.Disk.Size, 500*1024*1024)
				}
				if b.Disk.Path != "/cache" {
					t.Errorf("Path = %s, want /cache", b.Disk.Path)
				}
				if b.Disk.Duration != 10*time.Minute {
					t.Errorf("Duration = %v, want 10m", b.Disk.Duration)
				}
			},
		},
		{
			name:      "disk with default duration",
			input:     "disk=fill:1Gi:/data",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.Disk == nil {
					t.Fatal("expected disk behavior")
				}
				if b.Disk.Size != 1024*1024*1024 {
					t.Errorf("Size = %d, want %d", b.Disk.Size, 1024*1024*1024)
				}
				if b.Disk.Path != "/data" {
					t.Errorf("Path = %s, want /data", b.Disk.Path)
				}
				if b.Disk.Duration != 10*time.Minute {
					t.Errorf("Duration = %v, want 10m (default)", b.Disk.Duration)
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

