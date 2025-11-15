package behavior

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ErrorIfFileBehavior returns error if specified file contains invalid content
type ErrorIfFileBehavior struct {
	FilePath       string   // Path to the file to check
	InvalidContent []string // List of invalid strings that trigger error
	ErrorCode      int      // HTTP status code to return (default: 401)
}

// String returns the string representation of error-if-file behavior
func (ef *ErrorIfFileBehavior) String() string {
	errorStr := fmt.Sprintf("error-if-file=%s:%s", ef.FilePath, strings.Join(ef.InvalidContent, ";"))
	if ef.ErrorCode != 401 {
		errorStr += fmt.Sprintf(":%d", ef.ErrorCode)
	}
	return errorStr
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

func init() {
	registerParser("error-if-file", func(b *Behavior, value string) error {
		errorIfFile, err := parseErrorIfFile(value)
		if err != nil {
			return fmt.Errorf("invalid error-if-file: %w", err)
		}
		b.ErrorIfFile = errorIfFile
		return nil
	})
}

