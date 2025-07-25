package validation

import (
	"strings"
	"unicode"
)

// SanitizeClientID sanitizes an MQTT client ID by removing control characters
// and limiting length to MQTT specification (23 characters).
func SanitizeClientID(clientID string) string {
	// Remove control characters and non-printable characters
	sanitized := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, clientID)
	
	// MQTT client ID should be 1-23 characters
	if len(sanitized) > 23 {
		sanitized = sanitized[:23]
	}
	
	// Ensure it's not empty after sanitization
	if sanitized == "" {
		sanitized = "gom2k-default"
	}
	
	return sanitized
}

// SanitizeUsername sanitizes a username by removing control characters
// and potentially dangerous characters.
func SanitizeUsername(username string) string {
	// Remove control characters, quotes, and other potentially dangerous characters
	sanitized := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' || r == '\'' || r == '\\' || r == '\x00' {
			return -1
		}
		return r
	}, username)
	
	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)
	
	// Limit length to reasonable size
	if len(sanitized) > 128 {
		sanitized = sanitized[:128]
	}
	
	return sanitized
}

// SanitizePassword sanitizes a password by removing only null bytes
// and control characters that could break protocols.
func SanitizePassword(password string) string {
	// For passwords, we're more permissive but still remove dangerous characters
	sanitized := strings.Map(func(r rune) rune {
		// Remove null bytes and other control characters except common ones
		if r == '\x00' || (unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r') {
			return -1
		}
		return r
	}, password)
	
	return sanitized
}

// SanitizeConfigString sanitizes general configuration strings by removing
// control characters and limiting length.
func SanitizeConfigString(input string, maxLength int) string {
	// Remove control characters except spaces
	sanitized := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != ' ' && r != '\t' {
			return -1
		}
		return r
	}, input)
	
	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)
	
	// Apply length limit
	if maxLength > 0 && len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
	}
	
	return sanitized
}