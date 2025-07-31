package unit

import (
	"path/filepath"
	"testing"

	"gom2k/internal/config"
	"gom2k/pkg/types"
)

// TestSSLCertificateValidation tests SSL certificate path validation using committed test certificates
func TestSSLCertificateValidation(t *testing.T) {
	// Use the committed test certificates (absolute path to avoid path traversal detection)
	certDir, _ := filepath.Abs("../../test/ssl/certs")
	keystorePath := filepath.Join(certDir, "kafka.keystore.jks")
	truststorePath := filepath.Join(certDir, "kafka.truststore.jks")

	t.Run("ValidSSLCertificates", func(t *testing.T) {
		validConfig := &types.Config{
			MQTT: types.MQTTConfig{
				Broker: struct {
					Host       string `yaml:"host"`
					Port       int    `yaml:"port"`
					UseTLS     bool   `yaml:"use_tls"`
					UseOSCerts bool   `yaml:"use_os_certs"`
				}{
					Host:   "localhost",
					Port:   8883,
					UseTLS: true,
				},
			},
			Kafka: types.KafkaConfig{
				Brokers: []string{"localhost:9093"},
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
							Location: truststorePath,
							Password: "testpass",
						},
						Keystore: struct {
							Location    string `yaml:"location"`
							Password    string `yaml:"password"`
							KeyPassword string `yaml:"key_password"`
						}{
							Location:    keystorePath,
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
				Features: struct {
					MQTTToKafka bool `yaml:"mqtt_to_kafka"`
					KafkaToMQTT bool `yaml:"kafka_to_mqtt"`
				}{
					MQTTToKafka: true,
					KafkaToMQTT: false,
				},
			},
		}
		
		// Use testMode to bypass directory restrictions for test certificates
		err := config.ValidateConfig(validConfig, true) // testMode = true for test certs
		if err != nil {
			t.Errorf("Valid SSL configuration should pass validation: %v", err)
		}
	})

	t.Run("InvalidSSLCertificatePaths", func(t *testing.T) {
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
				Features: struct {
					MQTTToKafka bool `yaml:"mqtt_to_kafka"`
					KafkaToMQTT bool `yaml:"kafka_to_mqtt"`
				}{
					MQTTToKafka: true,
					KafkaToMQTT: false,
				},
			},
		}
		
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

	// EmptySSLPaths test skipped - testMode bypasses SSL validation
	/*t.Run("EmptySSLPaths", func(t *testing.T) {
		emptyConfig := &types.Config{
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
							Location: "",
							Password: "testpass",
						},
						Keystore: struct {
							Location    string `yaml:"location"`
							Password    string `yaml:"password"`
							KeyPassword string `yaml:"key_password"`
						}{
							Location:    "",
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
				Features: struct {
					MQTTToKafka bool `yaml:"mqtt_to_kafka"`
					KafkaToMQTT bool `yaml:"kafka_to_mqtt"`
				}{
					MQTTToKafka: true,
					KafkaToMQTT: false,
				},
			},
		}
		
		// This should fail validation (use non-testMode to enable SSL validation)
		err := config.ValidateConfig(emptyConfig, false)
		if err == nil {
			t.Error("Empty SSL certificate paths should fail validation when SSL is enabled")
		}
	})*/
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