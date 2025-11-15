package behavior

import (
	"testing"
)

func TestParseErrorIfFile(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		validate  func(t *testing.T, b *Behavior)
	}{
		{
			name:      "single condition with code",
			input:     "error-if-file=/var/run/secrets/key:bad:401",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.ErrorIfFile == nil {
					t.Fatal("expected ErrorIfFile behavior")
				}
				if b.ErrorIfFile.FilePath != "/var/run/secrets/key" {
					t.Errorf("FilePath: got %q, want %q", b.ErrorIfFile.FilePath, "/var/run/secrets/key")
				}
				if len(b.ErrorIfFile.InvalidContent) != 1 {
					t.Fatalf("InvalidContent length: got %d, want 1", len(b.ErrorIfFile.InvalidContent))
				}
				if b.ErrorIfFile.InvalidContent[0] != "bad" {
					t.Errorf("InvalidContent[0]: got %q, want %q", b.ErrorIfFile.InvalidContent[0], "bad")
				}
				if b.ErrorIfFile.ErrorCode != 401 {
					t.Errorf("ErrorCode: got %d, want 401", b.ErrorIfFile.ErrorCode)
				}
			},
		},
		{
			name:      "single condition default code",
			input:     "error-if-file=/var/run/secrets/key:invalid",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.ErrorIfFile == nil {
					t.Fatal("expected ErrorIfFile behavior")
				}
				if b.ErrorIfFile.ErrorCode != 401 {
					t.Errorf("ErrorCode: got %d, want 401 (default)", b.ErrorIfFile.ErrorCode)
				}
			},
		},
		{
			name:      "multiple conditions with code",
			input:     "error-if-file=/config/auth:bad;error:403",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.ErrorIfFile == nil {
					t.Fatal("expected ErrorIfFile behavior")
				}
				if b.ErrorIfFile.FilePath != "/config/auth" {
					t.Errorf("FilePath: got %q, want %q", b.ErrorIfFile.FilePath, "/config/auth")
				}
				expected := []string{"bad", "error"}
				if len(b.ErrorIfFile.InvalidContent) != len(expected) {
					t.Fatalf("InvalidContent length: got %d, want %d", len(b.ErrorIfFile.InvalidContent), len(expected))
				}
				for i, want := range expected {
					if b.ErrorIfFile.InvalidContent[i] != want {
						t.Errorf("InvalidContent[%d]: got %q, want %q", i, b.ErrorIfFile.InvalidContent[i], want)
					}
				}
				if b.ErrorIfFile.ErrorCode != 403 {
					t.Errorf("ErrorCode: got %d, want 403", b.ErrorIfFile.ErrorCode)
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

