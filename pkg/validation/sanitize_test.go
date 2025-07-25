package validation

import (
	"testing"
)

func TestSanitizeClientID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal client ID",
			input:    "gom2k-client-1",
			expected: "gom2k-client-1",
		},
		{
			name:     "client ID with control characters",
			input:    "gom2k\x00client\n\r",
			expected: "gom2kclient",
		},
		{
			name:     "too long client ID",
			input:    "verylongclientidthatexceedsthemaximumlength",
			expected: "verylongclientidthatexc",
		},
		{
			name:     "empty after sanitization",
			input:    "\x00\x01\x02",
			expected: "gom2k-default",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeClientID(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeClientID() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizeUsername(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal username",
			input:    "user123",
			expected: "user123",
		},
		{
			name:     "username with quotes",
			input:    "user'test\"name",
			expected: "usertestname",
		},
		{
			name:     "username with control chars",
			input:    "user\x00test\nname",
			expected: "usertestname",
		},
		{
			name:     "username with spaces",
			input:    "  user test  ",
			expected: "user test",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeUsername(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeUsername() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizePassword(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal password",
			input:    "P@ssw0rd!",
			expected: "P@ssw0rd!",
		},
		{
			name:     "password with null byte",
			input:    "pass\x00word",
			expected: "password",
		},
		{
			name:     "password with allowed control chars",
			input:    "pass\tword\n",
			expected: "pass\tword\n",
		},
		{
			name:     "password with special chars",
			input:    "p@$$w0rd!#%^&*()",
			expected: "p@$$w0rd!#%^&*()",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizePassword(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizePassword() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizeConfigString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		expected  string
	}{
		{
			name:      "normal string",
			input:     "config value",
			maxLength: 50,
			expected:  "config value",
		},
		{
			name:      "string with control chars",
			input:     "config\x00value\r\n",
			maxLength: 50,
			expected:  "configvalue",
		},
		{
			name:      "string exceeding max length",
			input:     "very long config value",
			maxLength: 10,
			expected:  "very long ",
		},
		{
			name:      "string with spaces preserved",
			input:     "  config  value  ",
			maxLength: 50,
			expected:  "config  value",
		},
		{
			name:      "no max length",
			input:     "config value of any length",
			maxLength: 0,
			expected:  "config value of any length",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeConfigString(tt.input, tt.maxLength)
			if result != tt.expected {
				t.Errorf("SanitizeConfigString() = %v, want %v", result, tt.expected)
			}
		})
	}
}