package behavior

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

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

