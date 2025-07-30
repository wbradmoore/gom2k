package unit

import (
	"testing"
	"time"

	"gom2k/internal/bridge"
	"gom2k/pkg/types"
)

func TestNewDeadLetterQueue(t *testing.T) {
	// Test with DLQ disabled
	config := &types.BridgeConfig{
		DeadLetter: struct {
			Enabled       bool   `yaml:"enabled"`
			KafkaTopic    string `yaml:"kafka_topic"`
			MQTTTopic     string `yaml:"mqtt_topic"`
			MaxRetries    int    `yaml:"max_retries"`
			RetryInterval time.Duration `yaml:"retry_interval"`
		}{
			Enabled: false,
		},
	}

	dlq := bridge.NewDeadLetterQueue(config, nil, nil)
	if dlq != nil {
		t.Error("Expected nil DLQ when disabled")
	}

	// Test with DLQ enabled
	config.DeadLetter.Enabled = true
	config.DeadLetter.MaxRetries = 3
	config.DeadLetter.RetryInterval = 30 * time.Second
	config.DeadLetter.KafkaTopic = "test-dlq"

	dlq = bridge.NewDeadLetterQueue(config, nil, nil)
	if dlq == nil {
		t.Error("Expected non-nil DLQ when enabled")
	}

	// Can only test that the DLQ was created successfully
	// Internal configuration details are not accessible
}

func TestDeadLetterQueueStartStop(t *testing.T) {
	config := &types.BridgeConfig{
		DeadLetter: struct {
			Enabled       bool   `yaml:"enabled"`
			KafkaTopic    string `yaml:"kafka_topic"`
			MQTTTopic     string `yaml:"mqtt_topic"`
			MaxRetries    int    `yaml:"max_retries"`
			RetryInterval time.Duration `yaml:"retry_interval"`
		}{
			Enabled:       true,
			MaxRetries:    2,
			RetryInterval: 100 * time.Millisecond, // Short interval for testing
		},
	}

	dlq := bridge.NewDeadLetterQueue(config, nil, nil)
	if dlq == nil {
		t.Fatal("Failed to create DLQ")
	}

	// Test start
	if err := dlq.Start(); err != nil {
		t.Errorf("Failed to start DLQ: %v", err)
	}

	// Test stop
	if err := dlq.Stop(); err != nil {
		t.Errorf("Failed to stop DLQ: %v", err)
	}
}

func TestHandleFailedMessage(t *testing.T) {
	config := &types.BridgeConfig{
		DeadLetter: struct {
			Enabled       bool   `yaml:"enabled"`
			KafkaTopic    string `yaml:"kafka_topic"`
			MQTTTopic     string `yaml:"mqtt_topic"`
			MaxRetries    int    `yaml:"max_retries"`
			RetryInterval time.Duration `yaml:"retry_interval"`
		}{
			Enabled:       true,
			MaxRetries:    2,
			RetryInterval: 50 * time.Millisecond,
			KafkaTopic:    "test-dlq",
		},
	}

	dlq := bridge.NewDeadLetterQueue(config, nil, nil)
	if dlq == nil {
		t.Fatal("Failed to create DLQ")
	}

	// Create a test message
	testMsg := &types.MQTTMessage{
		Topic:     "test/topic",
		Payload:   []byte("test payload"),
		QoS:       0,
		Retained:  false,
		Timestamp: time.Now(),
	}

	// First failure should add to retry queue
	dlq.HandleFailedMessage(testMsg, "test error", "mqtt-to-kafka", "test/topic", "gom2k.test.topic")
	
	if dlq.GetFailedMessageCount() != 1 {
		t.Errorf("Expected 1 failed message, got %d", dlq.GetFailedMessageCount())
	}

	// Second failure should exceed max retries (MaxRetries=2) and remove from queue
	dlq.HandleFailedMessage(testMsg, "test error 2", "mqtt-to-kafka", "test/topic", "gom2k.test.topic")
	
	if dlq.GetFailedMessageCount() != 0 {
		t.Errorf("Expected 0 failed messages after exceeding max retries, got %d", dlq.GetFailedMessageCount())
	}
}

// TestCreateMessageKey removed - createMessageKey is not exported
// The functionality is tested indirectly through other tests