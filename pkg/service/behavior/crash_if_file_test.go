package behavior

import (
	"testing"
)

func TestParseCrashIfFile(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		validate  func(t *testing.T, b *Behavior)
	}{
		{
			name:      "single crash-if-file",
			input:     "crash-if-file=/config/app.conf:invalid",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.CrashIfFile == nil {
					t.Fatal("expected CrashIfFile behavior")
				}
				if b.CrashIfFile.FilePath != "/config/app.conf" {
					t.Errorf("FilePath: got %q, want %q", b.CrashIfFile.FilePath, "/config/app.conf")
				}
				if len(b.CrashIfFile.InvalidContent) != 1 {
					t.Fatalf("InvalidContent length: got %d, want 1", len(b.CrashIfFile.InvalidContent))
				}
				if b.CrashIfFile.InvalidContent[0] != "invalid" {
					t.Errorf("InvalidContent[0]: got %q, want %q", b.CrashIfFile.InvalidContent[0], "invalid")
				}
			},
		},
		{
			name:      "crash-if-file with multiple conditions",
			input:     "crash-if-file=/etc/config:bad;error",
			wantError: false,
			validate: func(t *testing.T, b *Behavior) {
				if b.CrashIfFile == nil {
					t.Fatal("expected CrashIfFile behavior")
				}
				if b.CrashIfFile.FilePath != "/etc/config" {
					t.Errorf("FilePath: got %q, want %q", b.CrashIfFile.FilePath, "/etc/config")
				}
				expected := []string{"bad", "error"}
				if len(b.CrashIfFile.InvalidContent) != len(expected) {
					t.Fatalf("InvalidContent length: got %d, want %d", len(b.CrashIfFile.InvalidContent), len(expected))
				}
				for i, want := range expected {
					if b.CrashIfFile.InvalidContent[i] != want {
						t.Errorf("InvalidContent[%d]: got %q, want %q", i, b.CrashIfFile.InvalidContent[i], want)
					}
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

