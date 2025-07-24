//go:build integration

package integration

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"gom2k/internal/kafka"
	"gom2k/pkg/types"
)

func TestKafkaProducerConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	kafkaConfig := getKafkaTestConfig()
	bridgeConfig := getBridgeTestConfig()
	producer := kafka.NewProducer(kafkaConfig, bridgeConfig)

	// Test connection by connecting
	if err := producer.Connect(); err != nil {
		t.Fatalf("Failed to connect to Kafka: %v", err)
	}
	defer producer.Close()

	t.Log("Successfully connected to Kafka")
}

func TestKafkaProduceMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	kafkaConfig := getKafkaTestConfig()
	bridgeConfig := getBridgeTestConfig()
	producer := kafka.NewProducer(kafkaConfig, bridgeConfig)

	// Connect first
	if err := producer.Connect(); err != nil {
		t.Fatalf("Failed to connect to Kafka: %v", err)
	}
	defer producer.Close()

	// Test message
	testTopic := "gom2k-test-produce"
	testMessage := &types.KafkaMessage{
		Key:   "test-key",
		Value: []byte(`{"payload":"test","timestamp":"2023-01-01T00:00:00Z"}`),
		Topic: testTopic,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Produce message (auto-creates topic if needed)
	if err := producer.WriteMessage(ctx, testMessage); err != nil {
		t.Fatalf("Failed to produce message: %v", err)
	}

	t.Log("Successfully produced message to Kafka")
}

func TestKafkaSSLConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip if no SSL broker configured
	if os.Getenv("KAFKA_SSL_BROKERS") == "" {
		t.Skip("No SSL Kafka broker configured (set KAFKA_SSL_BROKERS)")
	}

	kafkaConfig := &types.KafkaConfig{}
	kafkaConfig.Brokers = []string{getEnv("KAFKA_SSL_BROKERS", "localhost:9093")}
	kafkaConfig.Security.Protocol = "SSL"
	kafkaConfig.Security.SSL.Truststore.Location = getEnv("KAFKA_SSL_TRUSTSTORE", "../ssl/certs/kafka.truststore.jks")
	kafkaConfig.Security.SSL.Truststore.Password = getEnv("KAFKA_SSL_TRUSTSTORE_PASSWORD", "testpass")
	kafkaConfig.Security.SSL.Keystore.Location = getEnv("KAFKA_SSL_KEYSTORE", "../ssl/certs/kafka.keystore.jks")
	kafkaConfig.Security.SSL.Keystore.Password = getEnv("KAFKA_SSL_KEYSTORE_PASSWORD", "testpass")
	kafkaConfig.Security.SSL.Keystore.KeyPassword = getEnv("KAFKA_SSL_KEY_PASSWORD", "testpass")

	bridgeConfig := getBridgeTestConfig()
	producer := kafka.NewProducer(kafkaConfig, bridgeConfig)

	// Test SSL connection
	if err := producer.Connect(); err != nil {
		t.Fatalf("Failed to connect with SSL: %v", err)
	}
	defer producer.Close()

	t.Log("Successfully connected to Kafka with SSL")
}

func TestKafkaAutoTopicCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	kafkaConfig := getKafkaTestConfig()
	bridgeConfig := getBridgeTestConfig()
	bridgeConfig.Kafka.AutoCreateTopics = true // Enable auto-creation
	producer := kafka.NewProducer(kafkaConfig, bridgeConfig)

	// Connect first
	if err := producer.Connect(); err != nil {
		t.Fatalf("Failed to connect to Kafka: %v", err)
	}
	defer producer.Close()

	// Test auto-creation with dynamic topic name
	testTopic := "gom2k-auto-" + strconv.FormatInt(time.Now().Unix(), 10)
	testMessage := &types.KafkaMessage{
		Key:   "auto-test",
		Value: []byte(`{"payload":"auto-created","timestamp":"2023-01-01T00:00:00Z"}`),
		Topic: testTopic,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// This should auto-create the topic and produce the message
	if err := producer.WriteMessage(ctx, testMessage); err != nil {
		t.Fatalf("Failed to auto-create topic and produce: %v", err)
	}

	t.Log("Successfully auto-created topic and produced message")
}

// Helper functions

func getKafkaTestConfig() *types.KafkaConfig {
	config := &types.KafkaConfig{}
	
	// Use environment variables or defaults for testing
	config.Brokers = []string{getEnv("KAFKA_BROKERS", "localhost:9092")}
	config.Security.Protocol = getEnv("KAFKA_PROTOCOL", "PLAINTEXT")
	
	// SSL settings (only used if protocol is SSL)
	config.Security.SSL.Truststore.Location = getEnv("KAFKA_SSL_TRUSTSTORE", "")
	config.Security.SSL.Truststore.Password = getEnv("KAFKA_SSL_TRUSTSTORE_PASSWORD", "")
	config.Security.SSL.Keystore.Location = getEnv("KAFKA_SSL_KEYSTORE", "")
	config.Security.SSL.Keystore.Password = getEnv("KAFKA_SSL_KEYSTORE_PASSWORD", "")
	
	return config
}

func getBridgeTestConfig() *types.BridgeConfig {
	return &types.BridgeConfig{
		Mapping: struct {
			KafkaPrefix    string `yaml:"kafka_prefix"`
			MaxTopicLevels int    `yaml:"max_topic_levels"`
		}{
			KafkaPrefix:    "gom2k-test",
			MaxTopicLevels: 3,
		},
		Kafka: struct {
			AutoCreateTopics  bool `yaml:"auto_create_topics"`
			DefaultPartitions int  `yaml:"default_partitions"`
			ReplicationFactor int  `yaml:"replication_factor"`
		}{
			AutoCreateTopics:  true,
			DefaultPartitions: 1,
			ReplicationFactor: 1,
		},
	}
}