package unit

import "strings"

// contains checks if a string contains a substring using Go's standard library
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}