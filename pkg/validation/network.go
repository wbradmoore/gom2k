package validation

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ValidateBrokerAddress validates a broker address in the format host:port.
// It checks for valid hostnames/IP addresses and port ranges.
func ValidateBrokerAddress(address string) error {
	if address == "" {
		return fmt.Errorf("broker address cannot be empty")
	}
	
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		// Just pass through the original error - net.SplitHostPort gives clear messages
		return fmt.Errorf("invalid broker address format: %w", err)
	}
	
	// Validate host
	if host == "" {
		return fmt.Errorf("broker host cannot be empty")
	}
	
	// Check for invalid characters in hostname
	if strings.ContainsAny(host, " \t\n\r\"'`;") {
		return fmt.Errorf("broker host contains invalid characters")
	}
	
	// Check if it's a valid IP address
	if ip := net.ParseIP(host); ip != nil {
		// It's a valid IP address
		return validatePort(portStr)
	}
	
	// Not an IP, validate as hostname
	if err := validateHostname(host); err != nil {
		return fmt.Errorf("invalid hostname: %w", err)
	}
	
	return validatePort(portStr)
}

// ValidateMQTTBroker validates MQTT broker configuration (host and port separately).
func ValidateMQTTBroker(host string, port int) error {
	if host == "" {
		return fmt.Errorf("MQTT broker host cannot be empty")
	}
	
	// Check for invalid characters
	if strings.ContainsAny(host, " \t\n\r\"'`;") {
		return fmt.Errorf("MQTT broker host contains invalid characters")
	}
	
	// Validate as IP or hostname
	if ip := net.ParseIP(host); ip == nil {
		// Not an IP, validate as hostname
		if err := validateHostname(host); err != nil {
			return fmt.Errorf("invalid MQTT broker hostname: %w", err)
		}
	}
	
	// Validate port
	if port < 1 || port > 65535 {
		return fmt.Errorf("MQTT broker port must be between 1 and 65535, got %d", port)
	}
	
	return nil
}

// validateHostname validates a hostname according to RFC 1123.
func validateHostname(hostname string) error {
	if len(hostname) > 253 {
		return fmt.Errorf("hostname too long (max 253 characters)")
	}
	
	// Check each label in the hostname
	labels := strings.Split(hostname, ".")
	if len(labels) == 0 {
		return fmt.Errorf("invalid hostname format")
	}
	
	for _, label := range labels {
		if len(label) == 0 {
			return fmt.Errorf("empty label in hostname")
		}
		if len(label) > 63 {
			return fmt.Errorf("hostname label too long (max 63 characters)")
		}
		
		// Check label format: must start with alphanumeric, can contain hyphens
		// but cannot end with a hyphen
		for i, ch := range label {
			if i == 0 && !isAlphaNumeric(ch) {
				return fmt.Errorf("hostname label must start with alphanumeric character")
			}
			if i == len(label)-1 && ch == '-' {
				return fmt.Errorf("hostname label cannot end with hyphen")
			}
			if !isAlphaNumeric(ch) && ch != '-' {
				return fmt.Errorf("invalid character '%c' in hostname", ch)
			}
		}
	}
	
	return nil
}

// validatePort validates a port number string.
func validatePort(portStr string) error {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port number: %w", err)
	}
	
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}
	
	return nil
}

// isAlphaNumeric checks if a rune is alphanumeric.
func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

