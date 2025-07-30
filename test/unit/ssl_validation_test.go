package unit

import (
	"os"
	"path/filepath"
	"testing"

	"gom2k/internal/config"
	"gom2k/pkg/types"
	"gom2k/test/testcerts"
)

// TestSSLCertificateValidation tests SSL certificate path validation without bypassing security checks
func TestSSLCertificateValidation(t *testing.T) {
	// Create test certificates
	fixtures, err := testcerts.NewFixtures(t)
	if err != nil {
		t.Skipf("Skipping SSL validation test: %v", err)
	}

	t.Run("ValidSSLCertificates", func(t *testing.T) {
		// Test with valid SSL certificate paths
		validConfig := fixtures.CompleteValidConfig()
		
		// This should pass validation WITHOUT testMode bypass
		err := config.ValidateConfig(validConfig, false) // testMode = false!
		if err != nil {
			t.Errorf("Valid SSL configuration should pass validation: %v", err)
		}
	})

	t.Run("InvalidSSLCertificatePaths", func(t *testing.T) {
		// Test with nonexistent SSL certificate paths
		invalidConfig := fixtures.CompleteInvalidSSLConfig()
		
		// This should fail validation
		err := config.ValidateConfig(invalidConfig, false) // testMode = false!
		if err == nil {
			t.Error("Invalid SSL certificate paths should fail validation")
		}
		
		// Verify error message contains SSL-related information
		if err != nil && !containsSSLError(err.Error()) {
			t.Errorf("Expected SSL-related error, got: %v", err)
		}
	})

	t.Run("EmptySSLPaths", func(t *testing.T) {
		// Test with empty SSL certificate paths when SSL is enabled
		emptyConfig := fixtures.CompleteValidConfig()
		emptyConfig.Kafka.Security.SSL.Keystore.Location = ""
		emptyConfig.Kafka.Security.SSL.Truststore.Location = ""
		
		// This should fail validation
		err := config.ValidateConfig(emptyConfig, false)
		if err == nil {
			t.Error("Empty SSL certificate paths should fail validation when SSL is enabled")
		}
	})

	t.Run("DisallowedSSLDirectories", func(t *testing.T) {
		// Test with SSL certificates in disallowed directories
		disallowedConfig := fixtures.DisallowedDirectoryConfig()
		fullConfig := &types.Config{
			MQTT:   *fixtures.ValidMQTTTLSConfig(),
			Kafka:  *disallowedConfig,
			Bridge: fixtures.CompleteValidConfig().Bridge,
		}
		
		// This should fail validation due to directory restrictions
		err := config.ValidateConfig(fullConfig, false)
		if err == nil {
			t.Error("SSL certificates in disallowed directories should fail validation")
		}
	})
}

// TestSSLValidationBypass tests that testMode properly bypasses SSL validation
func TestSSLValidationBypass(t *testing.T) {
	// Create config with invalid SSL paths
	invalidConfig := &types.Config{
		MQTT: types.MQTTConfig{
			Broker: struct {
				Host       string `yaml:"host"`
				Port       int    `yaml:"port"`
				UseTLS     bool   `yaml:"use_tls"`
				UseOSCerts bool   `yaml:"use_os_certs"`
			}{
				Host:   "localhost",
				Port:   1883,
				UseTLS: false,
			},
		},
		Kafka: types.KafkaConfig{
			Brokers: []string{"localhost:9092"},
			Security: struct {
				Protocol string `yaml:"protocol"`
				SSL      struct {
					Truststore struct {
						Location string `yaml:"location"`
						Password string `yaml:"password"`
					} `yaml:"truststore"`
					Keystore struct {
						Location    string `yaml:"location"`
						Password    string `yaml:"password"`
						KeyPassword string `yaml:"key_password"`
					} `yaml:"keystore"`
				} `yaml:"ssl"`
			}{
				Protocol: "SSL",
				SSL: struct {
					Truststore struct {
						Location string `yaml:"location"`
						Password string `yaml:"password"`
					} `yaml:"truststore"`
					Keystore struct {
						Location    string `yaml:"location"`
						Password    string `yaml:"password"`
						KeyPassword string `yaml:"key_password"`
					} `yaml:"keystore"`
				}{
					Truststore: struct {
						Location string `yaml:"location"`
						Password string `yaml:"password"`
					}{
						Location: "/nonexistent/truststore.jks",
						Password: "testpass",
					},
					Keystore: struct {
						Location    string `yaml:"location"`
						Password    string `yaml:"password"`
						KeyPassword string `yaml:"key_password"`
					}{
						Location:    "/nonexistent/keystore.jks",
						Password:    "testpass",
						KeyPassword: "testpass",
					},
				},
			},
		},
		Bridge: types.BridgeConfig{
			Mapping: struct {
				KafkaPrefix    string `yaml:"kafka_prefix"`
				MaxTopicLevels int    `yaml:"max_topic_levels"`
			}{
				KafkaPrefix:    "test",
				MaxTopicLevels: 3,
			},
		},
	}

	t.Run("TestModeBypassesSSLValidation", func(t *testing.T) {
		// With testMode=true, SSL validation should be skipped
		err := config.ValidateConfig(invalidConfig, true) // testMode = true
		if err == nil {
			t.Error("Config validation should fail due to other validation errors even in test mode")
		}
		// But it should NOT fail due to SSL certificate paths
		if err != nil && containsSSLError(err.Error()) {
			t.Errorf("SSL validation should be bypassed in test mode, but got SSL error: %v", err)
		}
	})

	t.Run("ProductionModeRequiresSSLValidation", func(t *testing.T) {
		// With testMode=false, SSL validation should be enforced
		err := config.ValidateConfig(invalidConfig, false) // testMode = false
		if err == nil {
			t.Error("Config validation should fail with invalid SSL certificates in production mode")
		}
		// Error should be SSL-related
		if err != nil && !containsSSLError(err.Error()) {
			t.Errorf("Expected SSL validation error in production mode, got: %v", err)
		}
	})
}

// TestSSLFilePathValidation tests the underlying SSL file path validation function
func TestSSLFilePathValidation(t *testing.T) {
	fixtures, err := testcerts.NewFixtures(t)
	if err != nil {
		t.Skipf("Skipping SSL file path validation test: %v", err)
	}

	t.Run("ValidSSLFilePaths", func(t *testing.T) {
		validConfig := fixtures.ValidKafkaSSLConfig()
		
		// Test that valid paths in allowed directories pass validation
		keystorePath := validConfig.Security.SSL.Keystore.Location
		truststorePath := validConfig.Security.SSL.Truststore.Location
		
		// Verify files exist
		if _, err := os.Stat(keystorePath); os.IsNotExist(err) {
			t.Errorf("Test keystore should exist at %s", keystorePath)
		}
		
		if _, err := os.Stat(truststorePath); os.IsNotExist(err) {
			t.Errorf("Test truststore should exist at %s", truststorePath)
		}
	})

	t.Run("PathTraversalAttempts", func(t *testing.T) {
		// Test that path traversal attempts are blocked
		maliciousConfig := fixtures.ValidKafkaSSLConfig()
		maliciousConfig.Security.SSL.Keystore.Location = "../../../etc/passwd"
		maliciousConfig.Security.SSL.Truststore.Location = "../../../../etc/hosts"
		
		fullConfig := &types.Config{
			MQTT:   *fixtures.ValidMQTTTLSConfig(),
			Kafka:  *maliciousConfig,
			Bridge: fixtures.CompleteValidConfig().Bridge,
		}
		
		err := config.ValidateConfig(fullConfig, false)
		if err == nil {
			t.Error("Path traversal attempts should be blocked by SSL validation")
		}
	})
}

// TestCertificateGeneration tests the test certificate generation infrastructure itself
func TestCertificateGeneration(t *testing.T) {
	t.Run("DefaultCertificateGeneration", func(t *testing.T) {
		certs, err := testcerts.CreateTestCertificates(t, testcerts.DefaultOptions())
		if err != nil {
			t.Skipf("Skipping certificate generation test: %v", err)
		}

		// Verify all expected files exist
		if !certs.Exists() {
			t.Error("Generated certificates should exist")
		}

		// Verify paths are returned correctly
		if certs.GetKeystorePath() == "" {
			t.Error("Keystore path should not be empty")
		}
		
		if certs.GetTruststorePath() == "" {
			t.Error("Truststore path should not be empty")
		}

		// Verify files are in temporary directory
		tempDir := certs.GetTempDir()
		if !filepath.IsAbs(tempDir) {
			t.Error("Temp directory should be absolute path")
		}
		
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			t.Error("Temp directory should exist")
		}
	})

	t.Run("CustomHostsCertificateGeneration", func(t *testing.T) {
		opts := &testcerts.CertificateOptions{
			Hosts:         []string{"example.com", "test.local"},
			Password:      "custompass",
			ValidityHours: 1,
		}
		
		certs, err := testcerts.CreateTestCertificates(t, opts)
		if err != nil {
			t.Skipf("Skipping custom certificate generation test: %v", err)
		}

		// Verify certificates were generated
		if !certs.Exists() {
			t.Error("Custom certificates should exist")
		}
		
		// Password should match custom setting
		if certs.GetPassword() != "custompass" {
			t.Errorf("Expected password 'custompass', got '%s'", certs.GetPassword())
		}
	})

	t.Run("CertificateCleanup", func(t *testing.T) {
		certs, err := testcerts.CreateTestCertificates(t, testcerts.DefaultOptions())
		if err != nil {
			t.Skipf("Skipping certificate cleanup test: %v", err)
		}

		tempDir := certs.GetTempDir()
		
		// Verify directory exists before cleanup
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			t.Error("Temp directory should exist before cleanup")
		}

		// Manual cleanup (automatic cleanup also happens via t.Cleanup)
		certs.Cleanup()
		
		// Verify directory is removed after cleanup
		if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
			t.Error("Temp directory should be removed after cleanup")
		}
	})
}

// containsSSLError checks if an error message contains SSL-related keywords
func containsSSLError(errMsg string) bool {
	sslKeywords := []string{
		"keystore", "truststore", "SSL", "ssl", 
		"certificate", "cert", ".jks", "path",
	}
	
	for _, keyword := range sslKeywords {
		if contains(errMsg, keyword) {
			return true
		}
	}
	return false
}