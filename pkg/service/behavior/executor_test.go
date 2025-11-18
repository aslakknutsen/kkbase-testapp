package behavior

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

// mockTelemetry implements TelemetryLogger for testing
type mockTelemetry struct {
	warnings []string
	fatals   []string
}

func (m *mockTelemetry) Warn(msg string, fields ...zap.Field) {
	m.warnings = append(m.warnings, msg)
}

func (m *mockTelemetry) Fatal(msg string, fields ...zap.Field) {
	m.fatals = append(m.fatals, msg)
	panic("fatal called: " + msg) // Fatal typically calls panic
}

func TestExecutor_NilBehavior(t *testing.T) {
	tel := &mockTelemetry{}
	executor := NewExecutor(nil, "trace123", "test-service", tel)

	result, err := executor.Execute(context.Background())

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result for nil behavior, got %+v", result)
	}
}

func TestExecutor_LatencyBehavior(t *testing.T) {
	tel := &mockTelemetry{}
	behavior := &Behavior{
		Latency: &LatencyBehavior{
			Type:  "fixed",
			Value: 10 * time.Millisecond,
		},
	}
	executor := NewExecutor(behavior, "trace123", "test-service", tel)

	start := time.Now()
	result, err := executor.Execute(context.Background())
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result (no early exit), got %+v", result)
	}
	if duration < 10*time.Millisecond {
		t.Errorf("Expected latency of at least 10ms, got %v", duration)
	}
}

func TestExecutor_DiskBehaviorSuccess(t *testing.T) {
	tel := &mockTelemetry{}
	
	// Create temp directory
	tmpDir := t.TempDir()
	
	behavior := &Behavior{
		Disk: &DiskBehavior{
			Size:     1024, // 1KB
			Path:     tmpDir,
			Duration: 100 * time.Millisecond,
		},
	}
	executor := NewExecutor(behavior, "trace123", "test-service", tel)

	result, err := executor.Execute(context.Background())

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result (no early exit), got %+v", result)
	}
	if len(tel.warnings) > 0 {
		t.Errorf("Expected no warnings, got %v", tel.warnings)
	}
}

func TestExecutor_DiskBehaviorFailure(t *testing.T) {
	tel := &mockTelemetry{}
	
	// Use non-existent directory to trigger failure
	behavior := &Behavior{
		Disk: &DiskBehavior{
			Size:     1024,
			Path:     "/nonexistent/path/that/should/not/exist",
			Duration: 1 * time.Second,
		},
	}
	executor := NewExecutor(behavior, "trace123", "test-service", tel)

	result, err := executor.Execute(context.Background())

	if err != nil {
		t.Errorf("Expected no error (executor returns result, not error), got %v", err)
	}
	if result == nil {
		t.Fatal("Expected ExecutionResult for disk failure")
	}
	if !result.ShouldReturn {
		t.Error("Expected ShouldReturn to be true")
	}
	if result.StatusCode != 507 {
		t.Errorf("Expected status code 507, got %d", result.StatusCode)
	}
	if result.BehaviorType != "disk-fill-failed" {
		t.Errorf("Expected behavior type 'disk-fill-failed', got %s", result.BehaviorType)
	}
	if len(tel.warnings) == 0 {
		t.Error("Expected warning to be logged")
	}
}

func TestExecutor_CrashIfFile(t *testing.T) {
	tel := &mockTelemetry{}
	
	// Create temp file with invalid content
	tmpFile := filepath.Join(t.TempDir(), "config.txt")
	if err := os.WriteFile(tmpFile, []byte("some text\ninvalid\nmore text"), 0644); err != nil {
		t.Fatal(err)
	}
	
	behavior := &Behavior{
		CrashIfFile: &CrashIfFileBehavior{
			FilePath:       tmpFile,
			InvalidContent: []string{"invalid"},
		},
	}
	executor := NewExecutor(behavior, "trace123", "test-service", tel)

	// Should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic, but did not panic")
		} else {
			// Verify Fatal was called
			if len(tel.fatals) == 0 {
				t.Error("Expected Fatal to be called before panic")
			}
		}
	}()

	executor.Execute(context.Background())
}

func TestExecutor_CrashIfFileNotMatched(t *testing.T) {
	tel := &mockTelemetry{}
	
	// Create temp file without invalid content
	tmpFile := filepath.Join(t.TempDir(), "config.txt")
	if err := os.WriteFile(tmpFile, []byte("valid content only"), 0644); err != nil {
		t.Fatal(err)
	}
	
	behavior := &Behavior{
		CrashIfFile: &CrashIfFileBehavior{
			FilePath:       tmpFile,
			InvalidContent: []string{"invalid"},
		},
	}
	executor := NewExecutor(behavior, "trace123", "test-service", tel)

	result, err := executor.Execute(context.Background())

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result (no early exit), got %+v", result)
	}
}

func TestExecutor_CrashIfFileReadError(t *testing.T) {
	tel := &mockTelemetry{}
	
	// Use non-existent file
	behavior := &Behavior{
		CrashIfFile: &CrashIfFileBehavior{
			FilePath:       "/nonexistent/file.txt",
			InvalidContent: []string{"invalid"},
		},
	}
	executor := NewExecutor(behavior, "trace123", "test-service", tel)

	result, err := executor.Execute(context.Background())

	// Should not crash on file read error, just log warning
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result (no early exit), got %+v", result)
	}
	if len(tel.warnings) == 0 {
		t.Error("Expected warning to be logged for file read error")
	}
}

func TestExecutor_ErrorIfFile(t *testing.T) {
	tel := &mockTelemetry{}
	
	// Create temp file with invalid content
	tmpFile := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(tmpFile, []byte("some secret\nbad_key\nmore data"), 0644); err != nil {
		t.Fatal(err)
	}
	
	behavior := &Behavior{
		ErrorIfFile: &ErrorIfFileBehavior{
			FilePath:       tmpFile,
			InvalidContent: []string{"bad_key"},
			ErrorCode:      403,
		},
	}
	executor := NewExecutor(behavior, "trace123", "test-service", tel)

	result, err := executor.Execute(context.Background())

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected ExecutionResult for error-if-file")
	}
	if !result.ShouldReturn {
		t.Error("Expected ShouldReturn to be true")
	}
	if result.StatusCode != 403 {
		t.Errorf("Expected status code 403, got %d", result.StatusCode)
	}
	if result.BehaviorType != "error-if-file" {
		t.Errorf("Expected behavior type 'error-if-file', got %s", result.BehaviorType)
	}
}

func TestExecutor_ErrorIfFileNotMatched(t *testing.T) {
	tel := &mockTelemetry{}
	
	// Create temp file without invalid content
	tmpFile := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(tmpFile, []byte("valid secret key"), 0644); err != nil {
		t.Fatal(err)
	}
	
	behavior := &Behavior{
		ErrorIfFile: &ErrorIfFileBehavior{
			FilePath:       tmpFile,
			InvalidContent: []string{"bad_key"},
			ErrorCode:      403,
		},
	}
	executor := NewExecutor(behavior, "trace123", "test-service", tel)

	result, err := executor.Execute(context.Background())

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result (no early exit), got %+v", result)
	}
}

func TestExecutor_PanicBehavior(t *testing.T) {
	tel := &mockTelemetry{}
	
	behavior := &Behavior{
		Panic: &PanicBehavior{
			Prob: 1.0, // 100% probability
		},
	}
	executor := NewExecutor(behavior, "trace123", "test-service", tel)

	// Should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic, but did not panic")
		} else {
			// Verify Fatal was called
			if len(tel.fatals) == 0 {
				t.Error("Expected Fatal to be called before panic")
			}
		}
	}()

	executor.Execute(context.Background())
}

func TestExecutor_ErrorBehavior(t *testing.T) {
	tel := &mockTelemetry{}
	
	behavior := &Behavior{
		Error: &ErrorBehavior{
			Rate: 503,
			Prob: 1.0, // 100% probability
		},
	}
	executor := NewExecutor(behavior, "trace123", "test-service", tel)

	result, err := executor.Execute(context.Background())

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected ExecutionResult for error injection")
	}
	if !result.ShouldReturn {
		t.Error("Expected ShouldReturn to be true")
	}
	if result.StatusCode != 503 {
		t.Errorf("Expected status code 503, got %d", result.StatusCode)
	}
	if result.BehaviorType != "error" {
		t.Errorf("Expected behavior type 'error', got %s", result.BehaviorType)
	}
}

func TestExecutor_PhaseOrdering(t *testing.T) {
	// Test that error-if-file comes before panic
	// If panic came first, the test would panic instead of returning error result
	tel := &mockTelemetry{}
	
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	
	behavior := &Behavior{
		ErrorIfFile: &ErrorIfFileBehavior{
			FilePath:       tmpFile,
			InvalidContent: []string{"invalid"},
			ErrorCode:      401,
		},
		Panic: &PanicBehavior{
			Prob: 1.0, // Would panic if reached
		},
	}
	executor := NewExecutor(behavior, "trace123", "test-service", tel)

	result, err := executor.Execute(context.Background())

	// Should return error-if-file result, not panic
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected ExecutionResult, not panic")
	}
	if result.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", result.StatusCode)
	}
}

func TestExecutor_String(t *testing.T) {
	tests := []struct {
		name     string
		behavior *Behavior
		expected string
	}{
		{
			name:     "nil behavior",
			behavior: nil,
			expected: "",
		},
		{
			name: "latency behavior",
			behavior: &Behavior{
				Latency: &LatencyBehavior{Type: "fixed", Value: 100 * time.Millisecond},
			},
			expected: "latency=100ms",
		},
		{
			name: "error behavior",
			behavior: &Behavior{
				Error: &ErrorBehavior{Rate: 500, Prob: 1.0},
			},
			expected: "error=500",
		},
	}

	tel := &mockTelemetry{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewExecutor(tt.behavior, "trace123", "test-service", tel)
			result := executor.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

