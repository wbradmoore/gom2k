package config

import (
	"fmt"
	"os"

	"gom2k/pkg/types"

	"github.com/spf13/viper"
)

// LoadFromFile reads configuration from a single YAML file
func LoadFromFile(configPath string) (*types.Config, error) {
	// Default to config.yaml if no path specified
	if configPath == "" {
		configPath = "config.yaml"
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	
	// Read the configuration file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	// Load configuration into struct
	config := &types.Config{}
	
	if err := v.UnmarshalKey("mqtt", &config.MQTT); err != nil {
		return nil, fmt.Errorf("failed to unmarshal MQTT config: %w", err)
	}
	
	if err := v.UnmarshalKey("kafka", &config.Kafka); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Kafka config: %w", err)
	}
	
	if err := v.UnmarshalKey("bridge", &config.Bridge); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bridge config: %w", err)
	}
	
	// Workaround for viper boolean unmarshaling issues
	if v.IsSet("bridge.features.mqtt_to_kafka") {
		config.Bridge.Features.MQTTToKafka = v.GetBool("bridge.features.mqtt_to_kafka")
	}
	if v.IsSet("bridge.features.kafka_to_mqtt") {
		config.Bridge.Features.KafkaToMQTT = v.GetBool("bridge.features.kafka_to_mqtt")
	}
	
	// Apply defaults and validate
	applyDefaults(config)
	
	if err := validate(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	return config, nil
}

// LoadForTesting loads config with minimal validation for connectivity tests
func LoadForTesting(configPath string) (*types.Config, error) {
	// Default to config.yaml if no path specified
	if configPath == "" {
		configPath = "config.yaml"
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	
	// Read the configuration file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	// Load configuration into struct
	config := &types.Config{}
	
	if err := v.UnmarshalKey("mqtt", &config.MQTT); err != nil {
		return nil, fmt.Errorf("failed to unmarshal MQTT config: %w", err)
	}
	
	if err := v.UnmarshalKey("kafka", &config.Kafka); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Kafka config: %w", err)
	}
	
	if err := v.UnmarshalKey("bridge", &config.Bridge); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bridge config: %w", err)
	}
	
	// Workaround for viper boolean unmarshaling issues
	if v.IsSet("bridge.features.mqtt_to_kafka") {
		config.Bridge.Features.MQTTToKafka = v.GetBool("bridge.features.mqtt_to_kafka")
	}
	if v.IsSet("bridge.features.kafka_to_mqtt") {
		config.Bridge.Features.KafkaToMQTT = v.GetBool("bridge.features.kafka_to_mqtt")
	}
	
	// Apply defaults but skip validation for testing
	applyDefaults(config)
	
	return config, nil
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
	if config.MQTT.Client.QoS == 0 {
		config.MQTT.Client.QoS = 0 // Explicit default
	}
}

// validate checks configuration for required fields and logical consistency
func validate(config *types.Config) error {
	if config.MQTT.Broker.Host == "" {
		return fmt.Errorf("MQTT broker host is required")
	}
	if config.MQTT.Broker.Port == 0 {
		return fmt.Errorf("MQTT broker port is required")
	}
	if len(config.Kafka.Brokers) == 0 {
		return fmt.Errorf("at least one Kafka broker is required")
	}
	if !config.Bridge.Features.MQTTToKafka && !config.Bridge.Features.KafkaToMQTT {
		return fmt.Errorf("at least one bridge direction must be enabled")
	}
	return nil
}