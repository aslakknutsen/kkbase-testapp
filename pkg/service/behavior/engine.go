package behavior

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Behavior represents parsed behavior directives
type Behavior struct {
	Latency      *LatencyBehavior
	Error        *ErrorBehavior
	CPU          *CPUBehavior
	Memory       *MemoryBehavior
	Panic        *PanicBehavior
	CrashIfFile  *CrashIfFileBehavior
	ErrorIfFile  *ErrorIfFileBehavior
	Disk         *DiskBehavior
	CustomParams map[string]string
}

// ServiceBehavior represents a behavior targeted at a specific service
type ServiceBehavior struct {
	Service  string    // Target service name (empty = applies to all)
	Behavior *Behavior // The actual behavior
}

// BehaviorChain represents multiple behaviors that can target different services
type BehaviorChain struct {
	Behaviors []ServiceBehavior
}

// ForService returns the behavior applicable to the given service name
func (bc *BehaviorChain) ForService(serviceName string) *Behavior {
	var specificBehavior *Behavior
	var globalBehavior *Behavior

	for _, sb := range bc.Behaviors {
		if sb.Service == serviceName {
			// Found behavior specifically for this service
			if specificBehavior == nil {
				specificBehavior = sb.Behavior
			} else {
				// Merge multiple behaviors for same service
				specificBehavior = mergeBehaviors(specificBehavior, sb.Behavior)
			}
		} else if sb.Service == "" {
			// Global behavior (no service prefix)
			if globalBehavior == nil {
				globalBehavior = sb.Behavior
			} else {
				globalBehavior = mergeBehaviors(globalBehavior, sb.Behavior)
			}
		}
	}

	// Specific behavior takes precedence over global
	if specificBehavior != nil {
		return specificBehavior
	}
	return globalBehavior
}

// String returns the behavior chain as a string for propagation
func (bc *BehaviorChain) String() string {
	if len(bc.Behaviors) == 0 {
		return ""
	}

	var parts []string
	for _, sb := range bc.Behaviors {
		behaviorStr := sb.Behavior.String()
		if behaviorStr == "" {
			continue
		}

		if sb.Service != "" {
			parts = append(parts, fmt.Sprintf("%s:%s", sb.Service, behaviorStr))
		} else {
			parts = append(parts, behaviorStr)
		}
	}

	return strings.Join(parts, ",")
}

// String returns the behavior as a string
func (b *Behavior) String() string {
	var parts []string

	if b.Latency != nil {
		if b.Latency.Type == "fixed" {
			parts = append(parts, fmt.Sprintf("latency=%s", b.Latency.Value))
		} else {
			parts = append(parts, fmt.Sprintf("latency=%s-%s", b.Latency.Min, b.Latency.Max))
		}
	}

	if b.Error != nil && b.Error.Prob > 0 {
		if b.Error.Rate != 500 {
			parts = append(parts, fmt.Sprintf("error=%d:%v", b.Error.Rate, b.Error.Prob))
		} else {
			parts = append(parts, fmt.Sprintf("error=%v", b.Error.Prob))
		}
	}

	if b.Panic != nil && b.Panic.Prob > 0 {
		parts = append(parts, fmt.Sprintf("panic=%v", b.Panic.Prob))
	}

	if b.CrashIfFile != nil {
		crashStr := fmt.Sprintf("crash-if-file=%s:%s", b.CrashIfFile.FilePath, strings.Join(b.CrashIfFile.InvalidContent, ";"))
		parts = append(parts, crashStr)
	}

	if b.ErrorIfFile != nil {
		errorStr := fmt.Sprintf("error-if-file=%s:%s", b.ErrorIfFile.FilePath, strings.Join(b.ErrorIfFile.InvalidContent, ";"))
		if b.ErrorIfFile.ErrorCode != 401 {
			errorStr += fmt.Sprintf(":%d", b.ErrorIfFile.ErrorCode)
		}
		parts = append(parts, errorStr)
	}

	if b.CPU != nil {
		cpuStr := fmt.Sprintf("cpu=%s", b.CPU.Pattern)
		if b.CPU.Duration > 0 {
			cpuStr += fmt.Sprintf(":%s:%d", b.CPU.Duration, b.CPU.Intensity)
		}
		parts = append(parts, cpuStr)
	}

	if b.Memory != nil {
		memStr := ""
		if strings.HasPrefix(b.Memory.Pattern, "leak") {
			memStr = fmt.Sprintf("memory=%s", b.Memory.Pattern)
			if b.Memory.Duration > 0 {
				memStr += fmt.Sprintf(":%s", b.Memory.Duration)
			}
		} else if b.Memory.Pattern == "spike" {
			// Format spike pattern with size and optional duration
			if b.Memory.Percentage > 0 {
				memStr = fmt.Sprintf("memory=spike:%d%%", b.Memory.Percentage)
			} else {
				memStr = fmt.Sprintf("memory=spike:%s", formatBytes(b.Memory.Amount))
			}
			if b.Memory.Duration > 0 {
				memStr += fmt.Sprintf(":%s", b.Memory.Duration)
			}
		} else {
			memStr = fmt.Sprintf("memory=%s", formatBytes(b.Memory.Amount))
		}
		parts = append(parts, memStr)
	}

	if b.Disk != nil {
		diskStr := fmt.Sprintf("disk=fill:%s:%s", formatBytes(b.Disk.Size), b.Disk.Path)
		if b.Disk.Duration != 10*time.Minute {
			diskStr += fmt.Sprintf(":%s", b.Disk.Duration)
		}
		parts = append(parts, diskStr)
	}

	// Include custom parameters
	if len(b.CustomParams) > 0 {
		for key, value := range b.CustomParams {
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return strings.Join(parts, ",")
}

// mergeBehaviors combines two behaviors
func mergeBehaviors(b1, b2 *Behavior) *Behavior {
	merged := &Behavior{
		CustomParams: make(map[string]string),
	}

	if b2.Latency != nil {
		merged.Latency = b2.Latency
	} else if b1.Latency != nil {
		merged.Latency = b1.Latency
	}

	if b2.Error != nil {
		merged.Error = b2.Error
	} else if b1.Error != nil {
		merged.Error = b1.Error
	}

	if b2.CPU != nil {
		merged.CPU = b2.CPU
	} else if b1.CPU != nil {
		merged.CPU = b1.CPU
	}

	if b2.Memory != nil {
		merged.Memory = b2.Memory
	} else if b1.Memory != nil {
		merged.Memory = b1.Memory
	}

	if b2.Panic != nil {
		merged.Panic = b2.Panic
	} else if b1.Panic != nil {
		merged.Panic = b1.Panic
	}

	if b2.CrashIfFile != nil {
		merged.CrashIfFile = b2.CrashIfFile
	} else if b1.CrashIfFile != nil {
		merged.CrashIfFile = b1.CrashIfFile
	}

	if b2.ErrorIfFile != nil {
		merged.ErrorIfFile = b2.ErrorIfFile
	} else if b1.ErrorIfFile != nil {
		merged.ErrorIfFile = b1.ErrorIfFile
	}

	if b2.Disk != nil {
		merged.Disk = b2.Disk
	} else if b1.Disk != nil {
		merged.Disk = b1.Disk
	}

	// Merge custom parameters (b2 overrides b1)
	for k, v := range b1.CustomParams {
		merged.CustomParams[k] = v
	}
	for k, v := range b2.CustomParams {
		merged.CustomParams[k] = v
	}

	return merged
}

// LatencyBehavior controls request latency
type LatencyBehavior struct {
	Type  string // "fixed", "range", "percentile"
	Min   time.Duration
	Max   time.Duration
	Value time.Duration
}

// ErrorBehavior controls error injection
type ErrorBehavior struct {
	Rate int     // HTTP status code to return
	Prob float64 // Probability (0.0-1.0)
}

// CPUBehavior controls CPU usage patterns
type CPUBehavior struct {
	Pattern   string // "spike", "steady", "ramp"
	Duration  time.Duration
	Intensity int // Percentage 0-100
}

// MemoryBehavior controls memory usage patterns
type MemoryBehavior struct {
	Pattern    string // "leak-slow", "leak-fast", "steady", "spike"
	Amount     int64  // Bytes to allocate
	Duration   time.Duration
	Percentage int // If >0, use percentage of container limit instead of Amount
}

// DiskBehavior controls disk space allocation
type DiskBehavior struct {
	Size     int64         // Bytes to allocate
	Path     string        // Directory to fill
	Duration time.Duration // How long to hold allocation
}

// PanicBehavior controls pod crash/panic
type PanicBehavior struct {
	Prob float64 // Probability (0.0-1.0)
}

// CrashIfFileBehavior crashes if specified file contains invalid content
type CrashIfFileBehavior struct {
	FilePath       string   // Path to the file to check
	InvalidContent []string // List of invalid strings that trigger crash
}

// ErrorIfFileBehavior returns error if specified file contains invalid content
type ErrorIfFileBehavior struct {
	FilePath       string   // Path to the file to check
	InvalidContent []string // List of invalid strings that trigger error
	ErrorCode      int      // HTTP status code to return (default: 401)
}

// Parse parses a behavior string into a Behavior struct
// Format: "latency=100ms,error=503:0.1,cpu=spike:5s,memory=leak-slow:10m"
func Parse(behaviorStr string) (*Behavior, error) {
	if behaviorStr == "" {
		return &Behavior{CustomParams: make(map[string]string)}, nil
	}

	b := &Behavior{
		CustomParams: make(map[string]string),
	}

	parts := strings.Split(behaviorStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch key {
		case "latency":
			latency, err := parseLatency(value)
			if err != nil {
				return nil, fmt.Errorf("invalid latency: %w", err)
			}
			b.Latency = latency

		case "error":
			errorBehavior, err := parseError(value)
			if err != nil {
				return nil, fmt.Errorf("invalid error: %w", err)
			}
			b.Error = errorBehavior

		case "cpu":
			cpu, err := parseCPU(value)
			if err != nil {
				return nil, fmt.Errorf("invalid cpu: %w", err)
			}
			b.CPU = cpu

		case "memory":
			mem, err := parseMemory(value)
			if err != nil {
				return nil, fmt.Errorf("invalid memory: %w", err)
			}
			b.Memory = mem

		case "panic":
			panicBehavior, err := parsePanic(value)
			if err != nil {
				return nil, fmt.Errorf("invalid panic: %w", err)
			}
			b.Panic = panicBehavior

		case "crash-if-file":
			crashIfFile, err := parseCrashIfFile(value)
			if err != nil {
				return nil, fmt.Errorf("invalid crash-if-file: %w", err)
			}
			b.CrashIfFile = crashIfFile

		case "error-if-file":
			errorIfFile, err := parseErrorIfFile(value)
			if err != nil {
				return nil, fmt.Errorf("invalid error-if-file: %w", err)
			}
			b.ErrorIfFile = errorIfFile

		case "disk":
			disk, err := parseDisk(value)
			if err != nil {
				return nil, fmt.Errorf("invalid disk: %w", err)
			}
			b.Disk = disk

		default:
			b.CustomParams[key] = value
		}
	}

	return b, nil
}

// ParseChain parses a behavior chain that can target specific services
// Syntax: "service1:latency=100ms,service2:error=0.5,latency=50ms"
// - "service1:latency=100ms" - applies only to service1
// - "latency=50ms" - applies to all services (no prefix)
func ParseChain(behaviorStr string) (*BehaviorChain, error) {
	if behaviorStr == "" {
		return &BehaviorChain{Behaviors: []ServiceBehavior{}}, nil
	}

	chain := &BehaviorChain{
		Behaviors: []ServiceBehavior{},
	}

	// Split by comma, but need to handle service:key=value format
	// Strategy: Look for patterns like "service:" or "key="
	var currentService string
	var currentBehaviorParts []string

	parts := strings.Split(behaviorStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if this part has a service prefix (contains : before =)
		colonPos := strings.Index(part, ":")
		equalsPos := strings.Index(part, "=")

		if colonPos > 0 && (equalsPos < 0 || colonPos < equalsPos) {
			// This is a service prefix: "service:latency=100ms"
			// Save previous behavior if any
			if len(currentBehaviorParts) > 0 {
				b, err := Parse(strings.Join(currentBehaviorParts, ","))
				if err != nil {
					return nil, err
				}
				chain.Behaviors = append(chain.Behaviors, ServiceBehavior{
					Service:  currentService,
					Behavior: b,
				})
				currentBehaviorParts = nil
			}

			// Extract service name and behavior
			serviceName := strings.TrimSpace(part[:colonPos])
			behaviorPart := strings.TrimSpace(part[colonPos+1:])

			currentService = serviceName
			if behaviorPart != "" {
				currentBehaviorParts = append(currentBehaviorParts, behaviorPart)
			}
		} else {
			// This is a regular behavior part
			currentBehaviorParts = append(currentBehaviorParts, part)
		}
	}

	// Don't forget the last behavior
	if len(currentBehaviorParts) > 0 {
		b, err := Parse(strings.Join(currentBehaviorParts, ","))
		if err != nil {
			return nil, err
		}
		chain.Behaviors = append(chain.Behaviors, ServiceBehavior{
			Service:  currentService,
			Behavior: b,
		})
	}

	return chain, nil
}

// parseLatency parses latency specifications
// Examples: "100ms", "50-200ms", "50ms-200ms", "5-20ms"
func parseLatency(value string) (*LatencyBehavior, error) {
	lb := &LatencyBehavior{}

	if strings.Contains(value, "-") {
		// Range: "50-200ms" or "50ms-200ms"
		parts := strings.Split(value, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format")
		}

		minStr := strings.TrimSpace(parts[0])
		maxStr := strings.TrimSpace(parts[1])

		// Try parsing min value
		min, err := time.ParseDuration(minStr)
		if err != nil {
			// If min doesn't have a unit, try to extract unit from max
			max, err2 := time.ParseDuration(maxStr)
			if err2 != nil {
				return nil, fmt.Errorf("invalid range: %w", err)
			}

			// Extract unit from max value (e.g., "200ms" -> "ms")
			unit := extractUnit(maxStr)
			if unit == "" {
				return nil, fmt.Errorf("could not determine unit from range")
			}

			// Append unit to min value
			minStr = minStr + unit
			min, err = time.ParseDuration(minStr)
			if err != nil {
				return nil, err
			}

			lb.Type = "range"
			lb.Min = min
			lb.Max = max
		} else {
			// Both values parsed successfully
			max, err := time.ParseDuration(maxStr)
			if err != nil {
				return nil, err
			}
			lb.Type = "range"
			lb.Min = min
			lb.Max = max
		}
	} else {
		// Fixed: "100ms"
		d, err := time.ParseDuration(value)
		if err != nil {
			return nil, err
		}
		lb.Type = "fixed"
		lb.Value = d
	}

	return lb, nil
}

// extractUnit extracts the unit suffix from a duration string
// e.g., "200ms" -> "ms", "5s" -> "s"
func extractUnit(s string) string {
	// Common time units in order of typical length
	units := []string{"ns", "us", "µs", "ms", "s", "m", "h"}
	for _, unit := range units {
		if strings.HasSuffix(s, unit) {
			return unit
		}
	}
	return ""
}

// formatBytes converts bytes to human-readable format (Mi, Gi)
// e.g., 1048576 -> "1Mi", 1073741824 -> "1Gi"
func formatBytes(bytes int64) string {
	const (
		_        = iota
		KB int64 = 1 << (10 * iota)
		MB
		GB
	)

	// Try Gi first
	if bytes >= GB && bytes%GB == 0 {
		return fmt.Sprintf("%dGi", bytes/GB)
	}
	// Try Mi
	if bytes >= MB && bytes%MB == 0 {
		return fmt.Sprintf("%dMi", bytes/MB)
	}
	// Try Ki
	if bytes >= KB && bytes%KB == 0 {
		return fmt.Sprintf("%dKi", bytes/KB)
	}
	// Return raw bytes
	return fmt.Sprintf("%d", bytes)
}

// parseBytes parses byte amounts with optional units
// Supports: "10Mi", "1Gi", "1024Ki", "1024" (raw bytes)
func parseBytes(value string) (int64, error) {
	const (
		_        = iota
		KB int64 = 1 << (10 * iota)
		MB
		GB
	)

	// Check for unit suffixes
	if strings.HasSuffix(value, "Gi") {
		numStr := strings.TrimSuffix(value, "Gi")
		num, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid Gi value: %w", err)
		}
		return num * GB, nil
	}
	if strings.HasSuffix(value, "Mi") {
		numStr := strings.TrimSuffix(value, "Mi")
		num, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid Mi value: %w", err)
		}
		return num * MB, nil
	}
	if strings.HasSuffix(value, "Ki") {
		numStr := strings.TrimSuffix(value, "Ki")
		num, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid Ki value: %w", err)
		}
		return num * KB, nil
	}

	// Parse as raw bytes
	num, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid byte value: %w", err)
	}
	return num, nil
}

// parseError parses error injection specifications
// Examples: "503", "0.1", "503:0.1"
func parseError(value string) (*ErrorBehavior, error) {
	eb := &ErrorBehavior{
		Rate: 500, // Default error code
		Prob: 0.0,
	}

	if strings.Contains(value, ":") {
		// Code and probability: "503:0.1"
		parts := strings.Split(value, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid error format")
		}
		code, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, err
		}
		prob, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, err
		}
		eb.Rate = code
		eb.Prob = prob
	} else {
		// Just probability or just code
		if strings.Contains(value, ".") {
			// Probability
			prob, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, err
			}
			eb.Prob = prob
		} else {
			// HTTP code with 100% probability
			code, err := strconv.Atoi(value)
			if err != nil {
				return nil, err
			}
			eb.Rate = code
			eb.Prob = 1.0
		}
	}

	return eb, nil
}

// parsePanic parses panic specifications
// Examples: "0.5", "1.0"
func parsePanic(value string) (*PanicBehavior, error) {
	prob, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, err
	}
	return &PanicBehavior{Prob: prob}, nil
}

// parseCrashIfFile parses crash-if-file specifications
// Format: "/path/to/file:invalid1;invalid2"
// Examples: "/config/app.conf:invalid", "/config/db.conf:bad;error"
// Note: Uses semicolon to separate multiple invalid strings (comma is used for behavior separation)
func parseCrashIfFile(value string) (*CrashIfFileBehavior, error) {
	// Split by first colon to separate path from invalid content
	colonIdx := strings.Index(value, ":")
	if colonIdx < 0 {
		return nil, fmt.Errorf("invalid format: expected 'path:invalid_content'")
	}

	filePath := strings.TrimSpace(value[:colonIdx])
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	invalidContentStr := strings.TrimSpace(value[colonIdx+1:])
	if invalidContentStr == "" {
		return nil, fmt.Errorf("invalid content list cannot be empty")
	}

	// Split invalid content by semicolon (to avoid conflict with behavior comma separator)
	var invalidContent []string
	for _, content := range strings.Split(invalidContentStr, ";") {
		if trimmed := strings.TrimSpace(content); trimmed != "" {
			invalidContent = append(invalidContent, trimmed)
		}
	}

	if len(invalidContent) == 0 {
		return nil, fmt.Errorf("at least one invalid content string required")
	}

	return &CrashIfFileBehavior{
		FilePath:       filePath,
		InvalidContent: invalidContent,
	}, nil
}

// parseErrorIfFile parses error-if-file specifications
// Format: "/path/to/file:invalid1;invalid2:code" or "/path/to/file:invalid1;invalid2"
// Examples: "/var/run/secrets/api-key:bad:401", "/var/run/secrets/api-key:invalid" (defaults to 401)
// Note: Uses semicolon to separate multiple invalid strings, optional error code at end
func parseErrorIfFile(value string) (*ErrorIfFileBehavior, error) {
	// Split by colon to get parts
	parts := strings.Split(value, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid format: expected 'path:invalid_content' or 'path:invalid_content:code'")
	}

	filePath := strings.TrimSpace(parts[0])
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	// Default error code
	errorCode := 401

	// Determine if last part is an error code
	var invalidContentStr string
	if len(parts) >= 3 {
		// Check if last part looks like an HTTP status code (3 digits)
		lastPart := strings.TrimSpace(parts[len(parts)-1])
		if code, err := strconv.Atoi(lastPart); err == nil && code >= 100 && code < 600 {
			// It's an error code
			errorCode = code
			// Join all parts between first and last as invalid content
			invalidContentStr = strings.Join(parts[1:len(parts)-1], ":")
		} else {
			// Not an error code, all remaining parts are invalid content
			invalidContentStr = strings.Join(parts[1:], ":")
		}
	} else {
		// Only 2 parts: path and invalid content
		invalidContentStr = parts[1]
	}

	invalidContentStr = strings.TrimSpace(invalidContentStr)
	if invalidContentStr == "" {
		return nil, fmt.Errorf("invalid content list cannot be empty")
	}

	// Split invalid content by semicolon (to avoid conflict with behavior comma separator)
	var invalidContent []string
	for _, content := range strings.Split(invalidContentStr, ";") {
		if trimmed := strings.TrimSpace(content); trimmed != "" {
			invalidContent = append(invalidContent, trimmed)
		}
	}

	if len(invalidContent) == 0 {
		return nil, fmt.Errorf("at least one invalid content string required")
	}

	return &ErrorIfFileBehavior{
		FilePath:       filePath,
		InvalidContent: invalidContent,
		ErrorCode:      errorCode,
	}, nil
}

// parseCPU parses CPU behavior specifications
// Examples: "spike", "spike:5s", "steady:10s:50"
func parseCPU(value string) (*CPUBehavior, error) {
	parts := strings.Split(value, ":")
	cb := &CPUBehavior{
		Pattern:   parts[0],
		Duration:  5 * time.Second,
		Intensity: 80,
	}

	if len(parts) > 1 {
		d, err := time.ParseDuration(parts[1])
		if err != nil {
			return nil, err
		}
		cb.Duration = d
	}

	if len(parts) > 2 {
		intensity, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, err
		}
		cb.Intensity = intensity
	}

	return cb, nil
}

// getContainerMemoryLimit attempts to determine the container memory limit
// using the following fallback chain:
// 1. GOMEMBALLAST environment variable
// 2. cgroup v1: /sys/fs/cgroup/memory/memory.limit_in_bytes
// 3. cgroup v2: /sys/fs/cgroup/memory.max
// Returns error if none are available or readable
func getContainerMemoryLimit() (int64, error) {
	// Try GOMEMBALLAST environment variable first
	if ballast := os.Getenv("GOMEMBALLAST"); ballast != "" {
		limit, err := parseBytes(ballast)
		if err == nil {
			return limit, nil
		}
	}

	// Try cgroup v1
	if data, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
		limitStr := strings.TrimSpace(string(data))
		limit, err := strconv.ParseInt(limitStr, 10, 64)
		if err == nil && limit > 0 {
			// Filter out "no limit" value (very large number)
			if limit < (1 << 62) {
				return limit, nil
			}
		}
	}

	// Try cgroup v2
	if data, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
		limitStr := strings.TrimSpace(string(data))
		if limitStr != "max" {
			limit, err := strconv.ParseInt(limitStr, 10, 64)
			if err == nil && limit > 0 {
				return limit, nil
			}
		}
	}

	return 0, fmt.Errorf("unable to determine container memory limit: GOMEMBALLAST not set and cgroup files not accessible")
}

// parseMemory parses memory behavior specifications
// Examples: "leak-slow", "leak-slow:10m", "10Mi", "1Gi", "spike:500Mi", "spike:80%:30s"
func parseMemory(value string) (*MemoryBehavior, error) {
	parts := strings.Split(value, ":")
	mb := &MemoryBehavior{
		Pattern:  parts[0],
		Amount:   10 * 1024 * 1024, // 10MB default
		Duration: 10 * time.Minute,
	}

	// Check if first part is a spike pattern
	if parts[0] == "spike" {
		// Spike pattern: spike:500Mi or spike:500Mi:30s or spike:80% or spike:80%:30s
		if len(parts) < 2 {
			return nil, fmt.Errorf("spike requires size: spike:500Mi or spike:80%%")
		}

		sizeStr := parts[1]

		// Check if it's a percentage
		if strings.HasSuffix(sizeStr, "%") {
			percentStr := strings.TrimSuffix(sizeStr, "%")
			percent, err := strconv.Atoi(percentStr)
			if err != nil {
				return nil, fmt.Errorf("invalid percentage: %w", err)
			}
			if percent < 1 || percent > 100 {
				return nil, fmt.Errorf("percentage must be between 1 and 100, got %d", percent)
			}
			mb.Percentage = percent
		} else {
			// Parse as byte amount
			amount, err := parseBytes(sizeStr)
			if err != nil {
				return nil, fmt.Errorf("invalid spike size: %w", err)
			}
			mb.Amount = amount
		}

		// Parse optional duration
		if len(parts) > 2 {
			d, err := time.ParseDuration(parts[2])
			if err != nil {
				return nil, fmt.Errorf("invalid spike duration: %w", err)
			}
			mb.Duration = d
		}
	} else if strings.HasPrefix(parts[0], "leak") {
		// It's a leak pattern like "leak-slow" or "leak-fast"
		if len(parts) > 1 {
			d, err := time.ParseDuration(parts[1])
			if err != nil {
				return nil, err
			}
			mb.Duration = d
		}
	} else {
		// Try to parse as byte amount (e.g., "10Mi", "1Gi", "1024")
		amount, err := parseBytes(parts[0])
		if err != nil {
			// If it fails, treat it as a pattern name (for backward compatibility)
			// This handles patterns like "steady" or other custom patterns
			mb.Pattern = parts[0]
		} else {
			// Successfully parsed as bytes
			mb.Amount = amount
			mb.Pattern = "steady" // Default pattern for amount-based allocation
		}
	}

	return mb, nil
}

// parseDisk parses disk behavior specifications
// Format: disk=fill:<size>:<path>:<duration>
// Examples: "fill:500Mi:/cache:10m", "fill:1Gi:/data"
func parseDisk(value string) (*DiskBehavior, error) {
	parts := strings.Split(value, ":")

	// Must start with "fill"
	if len(parts) < 3 || parts[0] != "fill" {
		return nil, fmt.Errorf("invalid format: expected 'fill:<size>:<path>[:<duration>]'")
	}

	// Parse size
	size, err := parseBytes(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid size: %w", err)
	}

	// Get path
	path := parts[2]
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	// Parse optional duration (default: 10m)
	duration := 10 * time.Minute
	if len(parts) > 3 {
		d, err := time.ParseDuration(parts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %w", err)
		}
		duration = d
	}

	return &DiskBehavior{
		Size:     size,
		Path:     path,
		Duration: duration,
	}, nil
}

// Apply applies the behavior to the current request
func (b *Behavior) Apply(ctx context.Context) error {
	if b.Latency != nil {
		if err := b.applyLatency(ctx); err != nil {
			return err
		}
	}

	if b.CPU != nil {
		b.applyCPU(ctx)
	}

	if b.Memory != nil {
		b.applyMemory(ctx)
	}

	return nil
}

// applyLatency applies latency behavior
func (b *Behavior) applyLatency(ctx context.Context) error {
	var delay time.Duration

	switch b.Latency.Type {
	case "fixed":
		delay = b.Latency.Value
	case "range":
		// Random duration between min and max
		diff := b.Latency.Max - b.Latency.Min
		delay = b.Latency.Min + time.Duration(rand.Int63n(int64(diff)))
	default:
		delay = b.Latency.Value
	}

	if delay > 0 {
		select {
		case <-time.After(delay):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// ShouldError determines if an error should be injected
func (b *Behavior) ShouldError() (bool, int) {
	if b.Error == nil {
		return false, 0
	}

	if rand.Float64() < b.Error.Prob {
		return true, b.Error.Rate
	}

	return false, 0
}

// ShouldPanic determines if a panic should be triggered
func (b *Behavior) ShouldPanic() bool {
	if b.Panic == nil {
		return false
	}

	return rand.Float64() < b.Panic.Prob
}

// ShouldCrashOnFile checks if the configured file contains invalid content
// Returns true if crash should occur, along with matched content and error message
func (b *Behavior) ShouldCrashOnFile() (bool, string, string) {
	if b.CrashIfFile == nil {
		return false, "", ""
	}

	// Read the file
	content, err := os.ReadFile(b.CrashIfFile.FilePath)
	if err != nil {
		// File read error - don't crash, just log
		return false, "", fmt.Sprintf("failed to read file %s: %v", b.CrashIfFile.FilePath, err)
	}

	// Check if file contains any invalid strings
	fileContent := string(content)
	for _, invalidStr := range b.CrashIfFile.InvalidContent {
		if strings.Contains(fileContent, invalidStr) {
			return true, invalidStr, fmt.Sprintf("Config file %s contains invalid content: '%s'", b.CrashIfFile.FilePath, invalidStr)
		}
	}

	return false, "", ""
}

// ShouldErrorOnFile checks if the configured file contains invalid content
// Returns true if error should be returned, along with error code, matched content, and error message
func (b *Behavior) ShouldErrorOnFile() (bool, int, string, string) {
	if b.ErrorIfFile == nil {
		return false, 0, "", ""
	}

	// Read the file
	content, err := os.ReadFile(b.ErrorIfFile.FilePath)
	if err != nil {
		// File read error - don't error, just log
		return false, 0, "", fmt.Sprintf("failed to read file %s: %v", b.ErrorIfFile.FilePath, err)
	}

	// Check if file contains any invalid strings
	fileContent := string(content)
	for _, invalidStr := range b.ErrorIfFile.InvalidContent {
		if strings.Contains(fileContent, invalidStr) {
			return true, b.ErrorIfFile.ErrorCode, invalidStr, fmt.Sprintf("File %s contains invalid content: '%s'", b.ErrorIfFile.FilePath, invalidStr)
		}
	}

	return false, 0, "", ""
}

// applyCPU applies CPU load
func (b *Behavior) applyCPU(ctx context.Context) {
	go func() {
		deadline := time.Now().Add(b.CPU.Duration)

		// Calculate work duration based on intensity
		// intensity = 80 means 80% busy, 20% idle
		workDuration := time.Duration(float64(b.CPU.Intensity) / 100.0 * float64(10*time.Millisecond))
		idleDuration := 10*time.Millisecond - workDuration

		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return
			default:
				// Do CPU-intensive work
				start := time.Now()
				for time.Since(start) < workDuration {
					// Busy loop - consume CPU
					_ = math.Sqrt(rand.Float64())
				}

				// Idle period
				if idleDuration > 0 {
					time.Sleep(idleDuration)
				}
			}
		}
	}()
}

// applyMemory applies memory allocation
func (b *Behavior) applyMemory(ctx context.Context) {
	go func() {
		var memHog [][]byte
		deadline := time.Now().Add(b.Memory.Duration)

		allocSize := 1024 * 1024 // 1MB chunks
		totalAllocated := int64(0)

		switch b.Memory.Pattern {
		case "leak-slow":
			interval := b.Memory.Duration / time.Duration(b.Memory.Amount/int64(allocSize))
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for time.Now().Before(deadline) && totalAllocated < b.Memory.Amount {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					chunk := make([]byte, allocSize)
					// Touch the memory to ensure it's allocated
					for i := 0; i < len(chunk); i += 4096 {
						chunk[i] = byte(i)
					}
					memHog = append(memHog, chunk)
					totalAllocated += int64(allocSize)
				}
			}

		case "leak-fast":
			// Allocate quickly
			for totalAllocated < b.Memory.Amount {
				chunk := make([]byte, allocSize)
				for i := 0; i < len(chunk); i += 4096 {
					chunk[i] = byte(i)
				}
				memHog = append(memHog, chunk)
				totalAllocated += int64(allocSize)
			}
			time.Sleep(b.Memory.Duration)

		case "spike":
			// Determine target allocation amount
			targetAmount := b.Memory.Amount
			if b.Memory.Percentage > 0 {
				// Calculate from container limit
				limit, err := getContainerMemoryLimit()
				if err != nil {
					// Log error but don't fail - this is best-effort
					fmt.Fprintf(os.Stderr, "Warning: unable to calculate percentage-based memory spike: %v\n", err)
					return
				}
				targetAmount = limit * int64(b.Memory.Percentage) / 100
			}

			// Allocate memory immediately in large chunks for faster allocation
			largeChunkSize := 10 * 1024 * 1024 // 10MB chunks for speed
			for totalAllocated < targetAmount {
				// Allocate the remaining or one chunk, whichever is smaller
				remaining := targetAmount - totalAllocated
				chunkSize := largeChunkSize
				if remaining < int64(chunkSize) {
					chunkSize = int(remaining)
				}

				chunk := make([]byte, chunkSize)
				// Touch all pages to ensure physical allocation
				for i := 0; i < len(chunk); i += 4096 {
					chunk[i] = byte(i)
				}
				memHog = append(memHog, chunk)
				totalAllocated += int64(chunkSize)
			}

			// Hold for the specified duration
			select {
			case <-ctx.Done():
				// Release and return early
				memHog = nil
				runtime.GC()
				return
			case <-time.After(b.Memory.Duration):
				// Duration elapsed, will release below
			}
		}

		// Keep memory allocated until context is done or duration expires
		select {
		case <-ctx.Done():
		case <-time.After(time.Until(deadline)):
		}

		// Allow GC to clean up
		memHog = nil
		runtime.GC()
	}()
}

// ApplyDisk fills disk space with a file
// Returns error immediately if file creation fails (e.g., disk full)
// Otherwise spawns background goroutine to hold allocation for duration
func (b *Behavior) ApplyDisk(ctx context.Context, traceID string) error {
	if b.Disk == nil {
		return nil
	}

	// Generate unique filename with trace ID
	filename := generateDiskFillFilename(b.Disk.Path, traceID)

	// Create and fill file synchronously to detect errors before returning
	if err := createDiskFillFile(filename, b.Disk.Size); err != nil {
		return err // Return error immediately (will be 507 if ENOSPC)
	}

	// File created successfully, now hold it in background
	go func() {
		// Hold for duration
		select {
		case <-ctx.Done():
			// Context cancelled, cleanup and return
			os.Remove(filename)
			return
		case <-time.After(b.Disk.Duration):
			// Duration elapsed, cleanup
			os.Remove(filename)
		}
	}()

	return nil
}

// generateDiskFillFilename creates a unique filename for disk fill
// Format: .testservice-fill-<traceID>-<random>.dat
func generateDiskFillFilename(path, traceID string) string {
	// Generate random suffix (8 hex chars)
	randSuffix := fmt.Sprintf("%08x", rand.Uint32())

	// Truncate trace ID if needed (use last 16 chars for readability)
	shortTraceID := traceID
	if len(traceID) > 16 {
		shortTraceID = traceID[len(traceID)-16:]
	}

	filename := fmt.Sprintf(".testservice-fill-%s-%s.dat", shortTraceID, randSuffix)
	return filepath.Join(path, filename)
}

// createDiskFillFile creates a file of specified size
// Uses sparse file technique (seek + write) for fast allocation
func createDiskFillFile(filename string, size int64) error {
	// Check if directory exists
	dir := filepath.Dir(filename)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dir)
	}

	// Create file
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	// Allocate space by seeking to size-1 and writing a byte
	// This creates a sparse file on most filesystems
	if _, err := f.Seek(size-1, 0); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	if _, err := f.Write([]byte{0}); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	// Sync to ensure space is actually allocated
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	return nil
}

// GetAppliedBehaviors returns a list of behaviors that were applied
func (b *Behavior) GetAppliedBehaviors() []string {
	var applied []string

	if b.Latency != nil {
		latencyStr := fmt.Sprintf("latency:%s", b.Latency.Type)
		if b.Latency.Type == "fixed" {
			latencyStr += fmt.Sprintf(":%s", b.Latency.Value)
		} else if b.Latency.Type == "range" {
			latencyStr += fmt.Sprintf(":%s-%s", b.Latency.Min, b.Latency.Max)
		}
		applied = append(applied, latencyStr)
	}
	if b.Error != nil {
		applied = append(applied, fmt.Sprintf("error:%d:%.2f", b.Error.Rate, b.Error.Prob))
	}
	if b.CPU != nil {
		cpuStr := fmt.Sprintf("cpu:%s", b.CPU.Pattern)
		if b.CPU.Duration > 0 {
			cpuStr += fmt.Sprintf(":%s", b.CPU.Duration)
		}
		if b.CPU.Intensity > 0 {
			cpuStr += fmt.Sprintf(":intensity=%d", b.CPU.Intensity)
		}
		applied = append(applied, cpuStr)
	}
	if b.Memory != nil {
		memStr := fmt.Sprintf("memory:%s", b.Memory.Pattern)
		if b.Memory.Amount > 0 {
			memStr += fmt.Sprintf(":%d", b.Memory.Amount)
		}
		if b.Memory.Duration > 0 {
			memStr += fmt.Sprintf(":%s", b.Memory.Duration)
		}
		applied = append(applied, memStr)
	}
	if b.Panic != nil {
		applied = append(applied, fmt.Sprintf("panic:%.2f", b.Panic.Prob))
	}
	if b.CrashIfFile != nil {
		applied = append(applied, fmt.Sprintf("crash-if-file:%s:%s", b.CrashIfFile.FilePath, strings.Join(b.CrashIfFile.InvalidContent, ";")))
	}
	if b.ErrorIfFile != nil {
		applied = append(applied, fmt.Sprintf("error-if-file:%s:%s:%d", b.ErrorIfFile.FilePath, strings.Join(b.ErrorIfFile.InvalidContent, ";"), b.ErrorIfFile.ErrorCode))
	}
	if b.Disk != nil {
		diskStr := fmt.Sprintf("disk:fill:%s:%s:%s",
			formatBytes(b.Disk.Size), b.Disk.Path, b.Disk.Duration)
		applied = append(applied, diskStr)
	}

	// Include custom parameters
	if len(b.CustomParams) > 0 {
		for key, value := range b.CustomParams {
			applied = append(applied, fmt.Sprintf("custom:%s=%s", key, value))
		}
	}

	return applied
}
