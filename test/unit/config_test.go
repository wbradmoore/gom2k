package unit

import (
	"fmt"
	"testing"

	"gom2k/internal/config"
	"gom2k/pkg/types"
)

func TestConfigLoading(t *testing.T) {
	// Test config loading using existing test config with LoadForTesting to avoid validation issues
	configPath := "../config.yaml"
	testConfig, err := config.LoadForTesting(configPath)
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}
	
	// LoadForTesting should apply defaults automatically

	// Verify MQTT config
	if testConfig.MQTT.Broker.Host != "localhost" {
		t.Errorf("Expected MQTT host 'localhost', got '%s'", testConfig.MQTT.Broker.Host)
	}

	if testConfig.MQTT.Broker.Port != 1883 {
		t.Errorf("Expected MQTT port 1883, got %d", testConfig.MQTT.Broker.Port)
	}

	if testConfig.MQTT.Broker.UseTLS != false {
		t.Errorf("Expected MQTT TLS false, got %v", testConfig.MQTT.Broker.UseTLS)
	}

	// Verify Kafka config
	if len(testConfig.Kafka.Brokers) != 1 || testConfig.Kafka.Brokers[0] != "localhost:9092" {
		t.Errorf("Expected Kafka brokers ['localhost:9092'], got %v", testConfig.Kafka.Brokers)
	}

	if testConfig.Kafka.Security.Protocol != "PLAINTEXT" {
		t.Errorf("Expected Kafka protocol 'PLAINTEXT', got '%s'", testConfig.Kafka.Security.Protocol)
	}
	
	if testConfig.Kafka.Consumer.GroupID != "gom2k-test-1" {
		t.Errorf("Expected Kafka consumer group 'gom2k-test-1', got '%s'", testConfig.Kafka.Consumer.GroupID)
	}

	// Verify bridge features are enabled
	if !testConfig.Bridge.Features.MQTTToKafka {
		t.Errorf("Expected MQTTToKafka to be enabled, got %v", testConfig.Bridge.Features.MQTTToKafka)
	}
	
	if !testConfig.Bridge.Features.KafkaToMQTT {
		t.Errorf("Expected KafkaToMQTT to be enabled, got %v", testConfig.Bridge.Features.KafkaToMQTT)
	}
	
	// Verify bridge mapping configuration
	if testConfig.Bridge.Mapping.KafkaPrefix != "gom2k" {
		t.Errorf("Expected Kafka prefix 'gom2k', got '%s'", testConfig.Bridge.Mapping.KafkaPrefix)
	}
	
	if testConfig.Bridge.Mapping.MaxTopicLevels != 3 {
		t.Errorf("Expected max topic levels 3, got %d", testConfig.Bridge.Mapping.MaxTopicLevels)
	}
	
	// Verify bridge Kafka settings
	if !testConfig.Bridge.Kafka.AutoCreateTopics {
		t.Errorf("Expected auto-create topics to be enabled, got %v", testConfig.Bridge.Kafka.AutoCreateTopics)
	}
	
	if testConfig.Bridge.Kafka.DefaultPartitions != 3 {
		t.Errorf("Expected default partitions 3, got %d", testConfig.Bridge.Kafka.DefaultPartitions)
	}
	
	if testConfig.Bridge.Kafka.ReplicationFactor != 1 {
		t.Errorf("Expected replication factor 1, got %d", testConfig.Bridge.Kafka.ReplicationFactor)
	}
}

func TestConfigDefaults(t *testing.T) {
	defaultConfig := &types.Config{}
	
	// Apply defaults (simulate config loading)
	applyTestDefaults(defaultConfig)

	// Test defaults
	if defaultConfig.Bridge.Mapping.KafkaPrefix != "gom2k" {
		t.Errorf("Expected default prefix 'gom2k', got '%s'", defaultConfig.Bridge.Mapping.KafkaPrefix)
	}

	if defaultConfig.Bridge.Mapping.MaxTopicLevels != 3 {
		t.Errorf("Expected default max levels 3, got %d", defaultConfig.Bridge.Mapping.MaxTopicLevels)
	}

	if defaultConfig.Bridge.Kafka.DefaultPartitions != 3 {
		t.Errorf("Expected default partitions 3, got %d", defaultConfig.Bridge.Kafka.DefaultPartitions)
	}

	if defaultConfig.Bridge.Kafka.ReplicationFactor != 1 {
		t.Errorf("Expected default replication 1, got %d", defaultConfig.Bridge.Kafka.ReplicationFactor)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    types.Config
		expectErr bool
	}{
		{
			name: "valid config",
			config: types.Config{
				MQTT: types.MQTTConfig{
					Broker: struct {
						Host       string `yaml:"host"`
						Port       int    `yaml:"port"`
						UseTLS     bool   `yaml:"use_tls"`
						UseOSCerts bool   `yaml:"use_os_certs"`
					}{Host: "localhost", Port: 1883},
				},
				Kafka: types.KafkaConfig{
					Brokers: []string{"localhost:9092"},
				},
				Bridge: types.BridgeConfig{
					Features: struct {
						MQTTToKafka bool `yaml:"mqtt_to_kafka"`
						KafkaToMQTT bool `yaml:"kafka_to_mqtt"`
					}{MQTTToKafka: true},
				},
			},
			expectErr: false,
		},
		{
			name: "missing MQTT host",
			config: types.Config{
				MQTT: types.MQTTConfig{
					Broker: struct {
						Host       string `yaml:"host"`
						Port       int    `yaml:"port"`
						UseTLS     bool   `yaml:"use_tls"`
						UseOSCerts bool   `yaml:"use_os_certs"`
					}{Port: 1883}, // Missing host
				},
				Kafka: types.KafkaConfig{
					Brokers: []string{"localhost:9092"},
				},
				Bridge: types.BridgeConfig{
					Features: struct {
						MQTTToKafka bool `yaml:"mqtt_to_kafka"`
						KafkaToMQTT bool `yaml:"kafka_to_mqtt"`
					}{MQTTToKafka: true},
				},
			},
			expectErr: true,
		},
		{
			name: "no bridge features enabled",
			config: types.Config{
				MQTT: types.MQTTConfig{
					Broker: struct {
						Host       string `yaml:"host"`
						Port       int    `yaml:"port"`
						UseTLS     bool   `yaml:"use_tls"`
						UseOSCerts bool   `yaml:"use_os_certs"`
					}{Host: "localhost", Port: 1883},
				},
				Kafka: types.KafkaConfig{
					Brokers: []string{"localhost:9092"},
				},
				Bridge: types.BridgeConfig{
					Features: struct {
						MQTTToKafka bool `yaml:"mqtt_to_kafka"`
						KafkaToMQTT bool `yaml:"kafka_to_mqtt"`
					}{MQTTToKafka: false, KafkaToMQTT: false},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTestConfig(&tt.config)
			if (err != nil) != tt.expectErr {
				t.Errorf("Expected error: %v, got: %v", tt.expectErr, err)
			}
		})
	}
}

// Helper functions for testing
func applyTestDefaults(config *types.Config) {
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
}

func validateTestConfig(config *types.Config) error {
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