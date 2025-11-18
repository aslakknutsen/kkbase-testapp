package behavior

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// ExecutionResult indicates what action to take after behavior execution
type ExecutionResult struct {
	ShouldReturn bool   // Whether to return early (before calling upstreams)
	StatusCode   int    // HTTP status code to return
	ErrorMessage string // Error message for response body
	BehaviorType string // Type of behavior that triggered the result (for telemetry)
}

// TelemetryLogger is the interface for logging warnings
type TelemetryLogger interface {
	Warn(msg string, fields ...zap.Field)
	Fatal(msg string, fields ...zap.Field)
}

// Executor handles ordered behavior execution with explicit phases
type Executor struct {
	behavior    *Behavior
	traceID     string
	serviceName string
	telemetry   TelemetryLogger
}

// NewExecutor creates a behavior executor
func NewExecutor(b *Behavior, traceID string, serviceName string, tel TelemetryLogger) *Executor {
	return &Executor{
		behavior:    b,
		traceID:     traceID,
		serviceName: serviceName,
		telemetry:   tel,
	}
}

// Execute runs behaviors in the required order, returning early if needed
// Execution phases (explicit ordering):
//  1. Apply non-terminating behaviors (latency/CPU/memory via existing Apply)
//  2. Disk behavior (returns 507 on failure)
//  3. Crash-if-file (panics)
//  4. Error-if-file (returns configured error code)
//  5. Panic injection (panics)
//  6. Error injection (returns error code)
func (e *Executor) Execute(ctx context.Context) (*ExecutionResult, error) {
	if e.behavior == nil {
		return nil, nil
	}

	// Phase 1: Apply non-terminating behaviors (latency, CPU, memory)
	if err := e.behavior.Apply(ctx); err != nil {
		return nil, fmt.Errorf("apply behavior: %w", err)
	}

	// Phase 2: Disk behavior (can fail with 507)
	if e.behavior.Disk != nil {
		if err := e.behavior.ApplyDisk(ctx, e.traceID); err != nil {
			e.telemetry.Warn("Disk fill failed",
				zap.Error(err),
				zap.String("path", e.behavior.Disk.Path),
				zap.Int64("size", e.behavior.Disk.Size),
			)
			return &ExecutionResult{
				ShouldReturn: true,
				StatusCode:   507,
				ErrorMessage: fmt.Sprintf("Disk fill failed: %v", err),
				BehaviorType: "disk-fill-failed",
			}, nil
		}
	}

	// Phase 3: Crash-if-file (terminates process)
	if shouldCrash, matched, msg := e.behavior.ShouldCrashOnFile(); shouldCrash {
		e.telemetry.Fatal("Config file contains invalid content - crashing as configured",
			zap.String("service", e.serviceName),
			zap.String("file", e.behavior.CrashIfFile.FilePath),
			zap.String("matched_content", matched),
			zap.String("message", msg),
		)
		panic(fmt.Sprintf("Config file crash: %s", msg))
	} else if msg != "" {
		// Log file read errors without crashing
		e.telemetry.Warn("Failed to check config file for invalid content",
			zap.String("file", e.behavior.CrashIfFile.FilePath),
			zap.String("error", msg),
		)
	}

	// Phase 4: Error-if-file (returns error response)
	if shouldErr, errCode, matched, msg := e.behavior.ShouldErrorOnFile(); shouldErr {
		e.telemetry.Warn("File contains invalid content - returning error as configured",
			zap.String("service", e.serviceName),
			zap.String("file", e.behavior.ErrorIfFile.FilePath),
			zap.String("matched_content", matched),
			zap.Int("error_code", errCode),
			zap.String("message", msg),
		)
		return &ExecutionResult{
			ShouldReturn: true,
			StatusCode:   errCode,
			ErrorMessage: fmt.Sprintf("File validation failed: %s", msg),
			BehaviorType: "error-if-file",
		}, nil
	} else if msg != "" {
		// Log file read errors without returning error
		e.telemetry.Warn("Failed to check file for invalid content",
			zap.String("file", e.behavior.ErrorIfFile.FilePath),
			zap.String("error", msg),
		)
	}

	// Phase 5: Panic injection
	if e.behavior.ShouldPanic() {
		e.telemetry.Fatal("Panic behavior triggered - crashing pod",
			zap.String("service", e.serviceName),
			zap.Float64("panic_prob", e.behavior.Panic.Prob),
		)
		panic(fmt.Sprintf("Panic behavior triggered in service %s", e.serviceName))
	}

	// Phase 6: Error injection
	if shouldErr, errCode := e.behavior.ShouldError(); shouldErr {
		return &ExecutionResult{
			ShouldReturn: true,
			StatusCode:   errCode,
			ErrorMessage: fmt.Sprintf("Injected error: %d", errCode),
			BehaviorType: "error",
		}, nil
	}

	return nil, nil
}

// String returns the behavior string for propagation
func (e *Executor) String() string {
	if e.behavior == nil {
		return ""
	}
	return e.behavior.String()
}
