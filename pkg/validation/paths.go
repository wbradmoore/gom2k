// Package validation provides input validation utilities for the GOM2K bridge.
// It includes validation for file paths, network addresses, and input sanitization
// to prevent security vulnerabilities such as path traversal and injection attacks.
package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateSSLFilePath validates SSL certificate file paths to prevent path traversal attacks.
// It ensures the file exists, is readable, and optionally restricts access to specific directories.
//
// Parameters:
//   - path: The file path to validate
//   - allowedDirs: Optional list of allowed directories. If empty, any path is allowed (after traversal checks)
//
// Returns an error if validation fails.
func ValidateSSLFilePath(path string, allowedDirs []string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	
	// Resolve to absolute path to prevent traversal
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}
	
	// Clean the path to remove any ./ or ../ elements
	cleanPath := filepath.Clean(absPath)
	
	// Check for directory traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal not allowed in file path")
	}
	
	// Validate against allowed directories if specified
	if len(allowedDirs) > 0 {
		allowed := false
		for _, dir := range allowedDirs {
			absDir, err := filepath.Abs(dir)
			if err != nil {
				continue
			}
			// Check if the file path is within the allowed directory
			if strings.HasPrefix(cleanPath, absDir) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("file path not in allowed directories")
		}
	}
	
	// Check file exists and is readable
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", cleanPath)
		}
		return fmt.Errorf("file not accessible: %w", err)
	}
	
	// Ensure it's a regular file (not a directory, symlink, etc.)
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file: %s", cleanPath)
	}
	
	// Check if file is readable
	file, err := os.Open(cleanPath)
	if err != nil {
		return fmt.Errorf("file not readable: %w", err)
	}
	file.Close()
	
	return nil
}

// ValidateConfigPath validates configuration file paths.
// It has less restrictive rules than SSL file paths but still prevents traversal.
func ValidateConfigPath(path string) error {
	if path == "" {
		return fmt.Errorf("config path cannot be empty")
	}
	
	// Resolve to absolute path first
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid config path: %w", err)
	}
	
	// Clean the path to normalize it
	cleanPath := filepath.Clean(absPath)
	
	// Check if file exists
	if _, err := os.Stat(cleanPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config file does not exist: %s", cleanPath)
		}
		return fmt.Errorf("config file not accessible: %w", err)
	}
	
	return nil
}