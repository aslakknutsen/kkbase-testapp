package behavior

import (
	"fmt"
	"os"
	"strings"
)

// CrashIfFileBehavior crashes if specified file contains invalid content
type CrashIfFileBehavior struct {
	FilePath       string   // Path to the file to check
	InvalidContent []string // List of invalid strings that trigger crash
}

// String returns the string representation of crash-if-file behavior
func (cf *CrashIfFileBehavior) String() string {
	return fmt.Sprintf("crash-if-file=%s:%s", cf.FilePath, strings.Join(cf.InvalidContent, ";"))
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

func init() {
	registerParser("crash-if-file", func(b *Behavior, value string) error {
		crashIfFile, err := parseCrashIfFile(value)
		if err != nil {
			return fmt.Errorf("invalid crash-if-file: %w", err)
		}
		b.CrashIfFile = crashIfFile
		return nil
	})
}

