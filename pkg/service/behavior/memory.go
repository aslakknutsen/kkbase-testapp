package behavior

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// MemoryBehavior controls memory usage patterns
type MemoryBehavior struct {
	Pattern    string // "leak-slow", "leak-fast", "steady", "spike"
	Amount     int64  // Bytes to allocate
	Duration   time.Duration
	Percentage int // If >0, use percentage of container limit instead of Amount
}

// String returns the string representation of memory behavior
func (mb *MemoryBehavior) String() string {
	memStr := ""
	if strings.HasPrefix(mb.Pattern, "leak") {
		memStr = fmt.Sprintf("memory=%s", mb.Pattern)
		if mb.Duration > 0 {
			memStr += fmt.Sprintf(":%s", mb.Duration)
		}
	} else if mb.Pattern == "spike" {
		// Format spike pattern with size and optional duration
		if mb.Percentage > 0 {
			memStr = fmt.Sprintf("memory=spike:%d%%", mb.Percentage)
		} else {
			memStr = fmt.Sprintf("memory=spike:%s", formatBytes(mb.Amount))
		}
		if mb.Duration > 0 {
			memStr += fmt.Sprintf(":%s", mb.Duration)
		}
	} else {
		memStr = fmt.Sprintf("memory=%s", formatBytes(mb.Amount))
	}
	return memStr
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

func init() {
	registerParser("memory", func(b *Behavior, value string) error {
		mem, err := parseMemory(value)
		if err != nil {
			return fmt.Errorf("invalid memory: %w", err)
		}
		b.Memory = mem
		return nil
	})
}

