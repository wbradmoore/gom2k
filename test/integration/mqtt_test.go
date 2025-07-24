//go:build integration

package integration

import (
	"os"
	"strconv"
	"testing"
	"time"

	"gom2k/internal/mqtt"
	"gom2k/pkg/types"
)

func TestMQTTConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	
	config := getMQTTTestConfig()
	client := mqtt.NewClient(config)
	
	// Test connection
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to MQTT broker: %v", err)
	}
	defer client.Disconnect()
	
	t.Log("Successfully connected to MQTT broker")
}

func TestMQTTSubscribePublish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	
	config := getMQTTTestConfig()
	
	// Override topics for testing
	config.Topics.Subscribe = []string{"gom2k/test/+"}
	config.Topics.RetainOnly = false
	
	client := mqtt.NewClient(config)
	
	// Set up message handler
	var receivedMessages []*types.MQTTMessage
	client.SetMessageHandler(func(msg *types.MQTTMessage) {
		receivedMessages = append(receivedMessages, msg)
	})
	
	// Connect and subscribe
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Disconnect()
	
	if err := client.Subscribe(); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	
	// Wait a moment for subscription to be active
	time.Sleep(100 * time.Millisecond)
	
	// Publish test message
	testTopic := "gom2k/test/temperature"
	testPayload := []byte("23.5")
	
	if err := client.Publish(testTopic, testPayload, 0, false); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}
	
	// Wait for message to arrive
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for published message")
		case <-ticker.C:
			if len(receivedMessages) > 0 {
				msg := receivedMessages[0]
				if msg.Topic != testTopic {
					t.Errorf("Expected topic %q, got %q", testTopic, msg.Topic)
				}
				if string(msg.Payload) != string(testPayload) {
					t.Errorf("Expected payload %q, got %q", string(testPayload), string(msg.Payload))
				}
				t.Log("Successfully received published message")
				return
			}
		}
	}
}

func TestMQTTTLSConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	
	// Skip if no TLS broker configured
	if os.Getenv("MQTT_TLS_HOST") == "" {
		t.Skip("No TLS MQTT broker configured (set MQTT_TLS_HOST)")
	}
	
	config := &types.MQTTConfig{}
	config.Broker.Host = getEnv("MQTT_TLS_HOST", "localhost")
	config.Broker.Port = getEnvInt("MQTT_TLS_PORT", 8883)
	config.Broker.UseTLS = true
	config.Broker.UseOSCerts = true
	config.Auth.Username = getEnv("MQTT_TLS_USERNAME", "")
	config.Auth.Password = getEnv("MQTT_TLS_PASSWORD", "")
	config.Client.ClientID = "gom2k-tls-test"
	config.Client.QoS = 0
	config.Topics.Subscribe = []string{"test/+"}
	config.Topics.RetainOnly = false
	
	client := mqtt.NewClient(config)
	
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect with TLS: %v", err)
	}
	defer client.Disconnect()
	
	t.Log("Successfully connected to MQTT broker with TLS")
}

// Helper functions

func getMQTTTestConfig() *types.MQTTConfig {
	config := &types.MQTTConfig{}
	
	// Use environment variables or defaults
	config.Broker.Host = getEnv("MQTT_HOST", "localhost")
	config.Broker.Port = getEnvInt("MQTT_PORT", 1883)
	config.Broker.UseTLS = getEnvBool("MQTT_TLS", false)
	config.Broker.UseOSCerts = getEnvBool("MQTT_USE_OS_CERTS", false)
	config.Auth.Username = getEnv("MQTT_USERNAME", "")
	config.Auth.Password = getEnv("MQTT_PASSWORD", "")
	config.Client.ClientID = "gom2k-test"
	config.Client.QoS = 0
	config.Topics.Subscribe = []string{"test/+"}
	config.Topics.RetainOnly = false
	
	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}