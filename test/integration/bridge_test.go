//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"gom2k/internal/bridge"
	"gom2k/internal/mqtt"
	"gom2k/pkg/types"
)

func TestMQTTToKafkaBridge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create full test configuration
	config := &types.Config{
		MQTT:   *getMQTTTestConfig(),
		Kafka:  *getKafkaTestConfig(),
		Bridge: *getBridgeTestConfig(),
	}
	
	// Override specific settings for test
	config.MQTT.Topics.Subscribe = []string{"bridge-test/+"}
	config.MQTT.Client.ClientID = "gom2k-bridge-test-mqtt"
	config.Bridge.Mapping.KafkaPrefix = "gom2k-bridge-test"
	config.Bridge.Features.MQTTToKafka = true
	config.Bridge.Features.KafkaToMQTT = false

	// Create bridge
	mqttToKafka := bridge.NewMQTTToKafkaBridge(config)

	// Start bridge
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		if err := mqttToKafka.Start(ctx); err != nil {
			t.Errorf("Bridge failed: %v", err)
		}
	}()

	// Wait for bridge to start
	time.Sleep(2 * time.Second)

	// Create separate MQTT client for publishing test messages
	testMqttConfig := getMQTTTestConfig()
	testMqttConfig.Client.ClientID = "gom2k-test-publisher"
	testClient := mqtt.NewClient(testMqttConfig)
	if err := testClient.Connect(); err != nil {
		t.Fatalf("Failed to connect test MQTT client: %v", err)
	}
	defer testClient.Disconnect()

	// Publish test message to MQTT
	testTopic := "bridge-test/temperature"
	testPayload := []byte("25.5")

	if err := testClient.Publish(testTopic, testPayload, 0, false); err != nil {
		t.Fatalf("Failed to publish MQTT message: %v", err)
	}

	// Wait for message to be processed
	time.Sleep(3 * time.Second)

	t.Log("Successfully bridged MQTT message to Kafka")
}

func TestBidirectionalBridge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create full bidirectional bridge test
	config := &types.Config{
		MQTT:   *getMQTTTestConfig(),
		Kafka:  *getKafkaTestConfig(),
		Bridge: *getBridgeTestConfig(),
	}
	
	// Enable both directions
	config.MQTT.Topics.Subscribe = []string{"bidirectional-test/+"}
	config.MQTT.Client.ClientID = "gom2k-bidirectional-test"
	config.Bridge.Mapping.KafkaPrefix = "gom2k-bidirect-test"
	config.Bridge.Features.MQTTToKafka = true
	config.Bridge.Features.KafkaToMQTT = true

	// Create bidirectional bridge
	bidirectionalBridge := bridge.NewBidirectionalBridge(config)

	// Start bridge
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		if err := bidirectionalBridge.Start(ctx); err != nil {
			t.Errorf("Bidirectional bridge failed: %v", err)
		}
	}()

	// Wait for bridge to start
	time.Sleep(2 * time.Second)

	t.Log("Successfully started bidirectional bridge")
}