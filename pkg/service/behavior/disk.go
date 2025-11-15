package behavior

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DiskBehavior controls disk space allocation
type DiskBehavior struct {
	Size     int64         // Bytes to allocate
	Path     string        // Directory to fill
	Duration time.Duration // How long to hold allocation
}

// String returns the string representation of disk behavior
func (db *DiskBehavior) String() string {
	diskStr := fmt.Sprintf("disk=fill:%s:%s", formatBytes(db.Size), db.Path)
	if db.Duration != 10*time.Minute {
		diskStr += fmt.Sprintf(":%s", db.Duration)
	}
	return diskStr
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

func init() {
	registerParser("disk", func(b *Behavior, value string) error {
		disk, err := parseDisk(value)
		if err != nil {
			return fmt.Errorf("invalid disk: %w", err)
		}
		b.Disk = disk
		return nil
	})
}

