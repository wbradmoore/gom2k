// Package config provides configuration loading and validation functionality for the bridge system.
// It supports YAML configuration files with environment variable substitution, default value
// application, and validation of required fields. The package includes special handling for
// viper boolean unmarshaling issues and provides separate loading modes for production and testing.
package config

import (
	"fmt"
	"os"
	"strings"

	"gom2k/pkg/types"
	"gom2k/pkg/validation"

	"github.com/spf13/viper"
)

// LoadFromFile reads configuration from a YAML file with full validation and default application.
// It loads MQTT, Kafka, and bridge configuration sections, applies sensible defaults for
// missing values, and validates that all required fields are present and at least one
// bridge direction is enabled.
func LoadFromFile(configPath string) (*types.Config, error) {
	// Default to config.yaml if no path specified
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Validate config path before loading
	if err := validation.ValidateConfigPath(configPath); err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	viperInstance := viper.New()
	viperInstance.SetConfigFile(configPath)
	
	// Read the configuration file
	if err := viperInstance.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	// Load configuration into struct
	config := &types.Config{}
	
	if err := viperInstance.UnmarshalKey("mqtt", &config.MQTT); err != nil {
		return nil, fmt.Errorf("failed to unmarshal MQTT config: %w", err)
	}
	
	if err := viperInstance.UnmarshalKey("kafka", &config.Kafka); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Kafka config: %w", err)
	}
	
	if err := viperInstance.UnmarshalKey("bridge", &config.Bridge); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bridge config: %w", err)
	}
	
	// Fix viper boolean unmarshaling issues
	applyViperWorkarounds(viperInstance, config)
	
	// Apply defaults and validate
	applyDefaults(config)
	
	if err := validate(config, false); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	return config, nil
}

// LoadForTesting loads config with test-friendly validation for connectivity tests
func LoadForTesting(configPath string) (*types.Config, error) {
	// Default to config.yaml if no path specified
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Validate config path before loading
	if err := validation.ValidateConfigPath(configPath); err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	testViperInstance := viper.New()
	testViperInstance.SetConfigFile(configPath)
	
	// Read the configuration file
	if err := testViperInstance.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	// Load configuration into struct
	config := &types.Config{}
	
	if err := testViperInstance.UnmarshalKey("mqtt", &config.MQTT); err != nil {
		return nil, fmt.Errorf("failed to unmarshal MQTT config: %w", err)
	}
	
	if err := testViperInstance.UnmarshalKey("kafka", &config.Kafka); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Kafka config: %w", err)
	}
	
	if err := testViperInstance.UnmarshalKey("bridge", &config.Bridge); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bridge config: %w", err)
	}
	
	// Fix viper boolean unmarshaling issues
	applyViperWorkarounds(testViperInstance, config)
	
	// Apply defaults and validate in test mode (skips SSL file validation)
	applyDefaults(config)
	
	if err := validate(config, true); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	return config, nil
}

// ValidateConfig exposes the validation function for testing
func ValidateConfig(config *types.Config, testMode bool) error {
	return validate(config, testMode)
}

// GetConfigPath returns the configuration file path from environment or default
func GetConfigPath() string {
	// Check for explicit config file path
	if configPath := os.Getenv("CONFIG_FILE"); configPath != "" {
		return configPath
	}
	
	// Check for test directory (if running from test.sh)
	if configDir := os.Getenv("CONFIGS_DIR"); configDir != "" {
		return configDir + "/config.yaml"
	}
	
	// Default to production config in configs directory
	return "./configs/config.yaml"
}

// applyDefaults sets default values for configuration fields
func applyDefaults(config *types.Config) {
	if config.Bridge.Mapping.KafkaPrefix == "" {
		config.Bridge.Mapping.KafkaPrefix = "gom2k"
	}
	if config.Bridge.Mapping.MaxTopicLevels == 0 {
		config.Bridge.Mapping.MaxTopicLevels = 3
	}
	if config.Bridge.Kafka.DefaultPartitions == 0 {
		config.Bridge.Kafka.DefaultPartitions = 3
	}
	if config.Bridge.Kafka.ReplicationFactor == 0 {
		config.Bridge.Kafka.ReplicationFactor = 1
	}
	// QoS defaults to 0 (no explicit setting needed)
}

// applyViperWorkarounds fixes viper's boolean and nested struct unmarshaling issues
func applyViperWorkarounds(v *viper.Viper, config *types.Config) {
	// Boolean unmarshaling fixes
	if v.IsSet("bridge.features.mqtt_to_kafka") {
		config.Bridge.Features.MQTTToKafka = v.GetBool("bridge.features.mqtt_to_kafka")
	}
	if v.IsSet("bridge.features.kafka_to_mqtt") {
		config.Bridge.Features.KafkaToMQTT = v.GetBool("bridge.features.kafka_to_mqtt")
	}
	
	// Nested structure unmarshaling fixes
	if v.IsSet("kafka.consumer.group_id") {
		config.Kafka.Consumer.GroupID = v.GetString("kafka.consumer.group_id")
	}
	if v.IsSet("bridge.kafka.auto_create_topics") {
		config.Bridge.Kafka.AutoCreateTopics = v.GetBool("bridge.kafka.auto_create_topics")
	}
	if v.IsSet("bridge.kafka.default_partitions") {
		config.Bridge.Kafka.DefaultPartitions = v.GetInt("bridge.kafka.default_partitions")
	}
	if v.IsSet("bridge.kafka.replication_factor") {
		config.Bridge.Kafka.ReplicationFactor = v.GetInt("bridge.kafka.replication_factor")
	}
}

// validate checks configuration for required fields and logical consistency
func validate(config *types.Config, testMode bool) error {
	if config.MQTT.Broker.Host == "" {
		return fmt.Errorf("MQTT broker host is required")
	}
	if config.MQTT.Broker.Port == 0 {
		return fmt.Errorf("MQTT broker port is required")
	}
	
	// Validate MQTT broker address
	if err := validation.ValidateMQTTBroker(config.MQTT.Broker.Host, config.MQTT.Broker.Port); err != nil {
		return fmt.Errorf("invalid MQTT broker configuration: %w", err)
	}
	
	if len(config.Kafka.Brokers) == 0 {
		return fmt.Errorf("at least one Kafka broker is required")
	}
	
	// Validate each Kafka broker address
	for _, broker := range config.Kafka.Brokers {
		if err := validation.ValidateBrokerAddress(broker); err != nil {
			return fmt.Errorf("invalid Kafka broker address %s: %w", broker, err)
		}
	}
	
	// Validate SSL certificate paths if SSL is enabled (skip in test mode)
	if config.Kafka.Security.Protocol == "SSL" && !testMode {
		// Define allowed directories for SSL certificates
		allowedDirs := []string{"/etc/ssl", "/opt/kafka/ssl", "./ssl", "./certs", "./config/ssl"}
		
		// Add environment-specific allowed directories
		if homeDir, err := os.UserHomeDir(); err == nil {
			allowedDirs = append(allowedDirs, fmt.Sprintf("%s/.kafka/ssl", homeDir))
			allowedDirs = append(allowedDirs, fmt.Sprintf("%s/.ssl", homeDir))
		}
		
		if config.Kafka.Security.SSL.Keystore.Location != "" {
			if err := validation.ValidateSSLFilePath(config.Kafka.Security.SSL.Keystore.Location, allowedDirs); err != nil {
				return fmt.Errorf("invalid keystore path: %w", err)
			}
		}
		
		if config.Kafka.Security.SSL.Truststore.Location != "" {
			if err := validation.ValidateSSLFilePath(config.Kafka.Security.SSL.Truststore.Location, allowedDirs); err != nil {
				return fmt.Errorf("invalid truststore path: %w", err)
			}
		}
	}
	
	// Sanitize and validate authentication credentials
	if config.MQTT.Auth.Username != "" {
		config.MQTT.Auth.Username = validation.SanitizeUsername(config.MQTT.Auth.Username)
	}
	if config.MQTT.Auth.Password != "" {
		config.MQTT.Auth.Password = validation.SanitizePassword(config.MQTT.Auth.Password)
	}
	
	// Sanitize client ID
	if config.MQTT.Client.ClientID != "" {
		// Extract the base client ID (before any {random} template)
		baseClientID := config.MQTT.Client.ClientID
		if idx := strings.Index(baseClientID, "{"); idx > 0 {
			baseClientID = baseClientID[:idx]
		}
		
		// Sanitize the base and reconstruct
		sanitizedBase := validation.SanitizeClientID(baseClientID)
		if strings.Contains(config.MQTT.Client.ClientID, "{random}") {
			config.MQTT.Client.ClientID = sanitizedBase + "-{random}"
		} else {
			config.MQTT.Client.ClientID = sanitizedBase
		}
	}
	
	if !config.Bridge.Features.MQTTToKafka && !config.Bridge.Features.KafkaToMQTT {
		return fmt.Errorf("at least one bridge direction must be enabled")
	}
	
	return nil
}