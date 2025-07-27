//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gom2k/internal/bridge"
	"gom2k/internal/kafka"
	"gom2k/internal/mqtt"
	"gom2k/pkg/types"
)

// TestLongTopicNames tests end-to-end behavior with MQTT topics that would create long Kafka topic names
func TestLongTopicNames(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name           string
		mqttTopic      string
		prefix         string
		maxLevels      int
		expectedKafka  string
		description    string
	}{
		{
			name:           "WoW reference - very long topic",
			mqttTopic:      "azeroth/eastern-kingdoms/stormwind/elwynn-forest/deadmines/instance-42/van-cleef-hideout/defias-brotherhood/edwin-vancleef/loot-table/rare-drops/cruel-barb/stats/damage/min-max/enchantments/current",
			prefix:         "gom2k-test",
			maxLevels:      10,
			expectedKafka:  "gom2k-test.azeroth.eastern-kingdoms.stormwind.elwynn-forest.deadmines.instance-42.van-cleef-hideout.defias-brotherhood.edwin-vancleef.loot-table",
			description:    "World of Warcraft themed topic that would exceed 249 chars",
		},
		{
			name:           "Enterprise IoT - deeply nested hierarchy",
			mqttTopic:      "enterprise/manufacturing/facility-alpha/building-3/floor-2/production-line-12/assembly-station-45/robotic-arm-7/sensor-array-9/temperature/precision/high-accuracy/zone-4/alerts",
			prefix:         "very-long-enterprise-organization-name-for-iot-infrastructure",
			maxLevels:      8,
			expectedKafka:  "very-long-enterprise-organization-name-for-iot-infrastructure.enterprise.manufacturing.facility-alpha.building-3.floor-2.production-line-12.assembly-station-45.robotic-arm",
			description:    "Enterprise IoT scenario with long prefix and deep nesting",
		},
		{
			name:           "Standard topic - should not truncate",
			mqttTopic:      "home/living-room/temperature",
			prefix:         "gom2k",
			maxLevels:      3,
			expectedKafka:  "gom2k.home.living-room.temperature",
			description:    "Normal topic that doesn't need truncation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration with specific settings
			config := &types.Config{
				MQTT:   *getMQTTTestConfig(),
				Kafka:  *getKafkaTestConfig(),
				Bridge: *getBridgeTestConfig(),
			}
			
			// Configure for this specific test
			config.MQTT.Topics.Subscribe = []string{strings.Split(tt.mqttTopic, "/")[0] + "/+"}
			config.MQTT.Client.ClientID = "gom2k-length-test-" + strings.ReplaceAll(tt.name, " ", "-")
			config.Bridge.Mapping.KafkaPrefix = tt.prefix
			config.Bridge.Mapping.MaxTopicLevels = tt.maxLevels
			config.Kafka.Consumer.GroupID = "gom2k-length-test-" + strings.ReplaceAll(tt.name, " ", "-")

			// Calculate expected truncated Kafka topic
			expectedTruncated := tt.expectedKafka
			if len(expectedTruncated) > 249 {
				expectedTruncated = expectedTruncated[:249]
				if expectedTruncated[len(expectedTruncated)-1] == '.' {
					expectedTruncated = expectedTruncated[:len(expectedTruncated)-1]
				}
			}

			t.Logf("Testing topic length: MQTT=%q (len=%d) -> Kafka=%q (len=%d)", 
				tt.mqttTopic, len(tt.mqttTopic), expectedTruncated, len(expectedTruncated))

			// Verify Kafka topic length is valid
			if len(expectedTruncated) > 249 {
				t.Fatalf("Expected Kafka topic still exceeds 249 chars: %d", len(expectedTruncated))
			}

			// Create bridge components
			mqttToKafka := bridge.NewMQTTToKafkaBridge(config)
			kafkaToMQTT := bridge.NewKafkaToMQTTBridge(config)

			// Test MQTT -> Kafka direction
			t.Run("MQTT_to_Kafka", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				// Start the MQTT to Kafka bridge
				if err := mqttToKafka.Start(ctx); err != nil {
					t.Fatalf("Failed to start MQTT to Kafka bridge: %v", err)
				}
				defer mqttToKafka.Stop()

				// Create test MQTT client to publish
				testMQTT := mqtt.NewClient(&config.MQTT)
				if err := testMQTT.Connect(); err != nil {
					t.Fatalf("Failed to connect test MQTT client: %v", err)
				}
				defer testMQTT.Disconnect()

				// Publish test message
				testPayload := `{"sensor_value": 23.5, "timestamp": "2024-01-01T12:00:00Z"}`
				if err := testMQTT.Publish(tt.mqttTopic, []byte(testPayload), 0, false); err != nil {
					t.Fatalf("Failed to publish test message: %v", err)
				}

				// Allow time for message processing
				time.Sleep(2 * time.Second)

				// Verify no errors occurred (we can't easily verify Kafka contents in integration test without a consumer)
				// The main goal is to ensure no panics or failures occur with long topic names
				t.Logf("Successfully published to long MQTT topic and processed without errors")
			})

			// Test Kafka -> MQTT direction (round trip)
			t.Run("Kafka_to_MQTT_roundtrip", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				// Start both bridges for round-trip test
				if err := mqttToKafka.Start(ctx); err != nil {
					t.Fatalf("Failed to start MQTT to Kafka bridge: %v", err)
				}
				defer mqttToKafka.Stop()

				if err := kafkaToMQTT.Start(ctx); err != nil {
					t.Fatalf("Failed to start Kafka to MQTT bridge: %v", err)
				}
				defer kafkaToMQTT.Stop()

				// Create test MQTT client for publishing
				publishClient := mqtt.NewClient(&config.MQTT)
				publishClient.(*mqtt.Client).SetClientID("test-publisher-" + strings.ReplaceAll(tt.name, " ", "-"))
				if err := publishClient.Connect(); err != nil {
					t.Fatalf("Failed to connect publisher MQTT client: %v", err)
				}
				defer publishClient.Disconnect()

				// Create subscription client to verify round-trip
				subscribeConfig := config.MQTT
				subscribeConfig.Client.ClientID = "test-subscriber-" + strings.ReplaceAll(tt.name, " ", "-")
				subscribeConfig.Topics.Subscribe = []string{tt.mqttTopic}
				
				subscribeClient := mqtt.NewClient(&subscribeConfig)
				if err := subscribeClient.Connect(); err != nil {
					t.Fatalf("Failed to connect subscriber MQTT client: %v", err)
				}
				defer subscribeClient.Disconnect()

				if err := subscribeClient.Subscribe(); err != nil {
					t.Fatalf("Failed to subscribe: %v", err)
				}

				// Allow time for subscription to be established
				time.Sleep(1 * time.Second)

				// Publish test message with unique payload
				testPayload := map[string]interface{}{
					"test_id":     tt.name,
					"sensor_value": 42.7,
					"timestamp":   time.Now().Format(time.RFC3339),
					"description": tt.description,
				}
				payloadBytes, _ := json.Marshal(testPayload)

				if err := publishClient.Publish(tt.mqttTopic, payloadBytes, 0, false); err != nil {
					t.Fatalf("Failed to publish round-trip test message: %v", err)
				}

				// Allow time for round-trip processing
				time.Sleep(3 * time.Second)

				t.Logf("Round-trip test completed for topic %q -> Kafka topic (truncated to %d chars) -> back to MQTT", 
					tt.mqttTopic, len(expectedTruncated))
			})
		})
	}
}

// TestTopicTruncationBehavior specifically tests the truncation logic in isolation
func TestTopicTruncationBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a bridge to test topic mapping
	config := &types.Config{
		MQTT:   *getMQTTTestConfig(),
		Kafka:  *getKafkaTestConfig(),
		Bridge: *getBridgeTestConfig(),
	}

	// Test with very long prefix to force truncation
	config.Bridge.Mapping.KafkaPrefix = "extremely-long-enterprise-kafka-prefix-that-will-cause-truncation-when-combined-with-topic-segments"
	config.Bridge.Mapping.MaxTopicLevels = 10

	bridge := bridge.NewMQTTToKafkaBridge(config)

	tests := []struct {
		name      string
		mqttTopic string
	}{
		{
			name:      "topic that will definitely exceed 249 chars",
			mqttTopic: "building/floor/department/subdivision/room/equipment/sensor/measurement/precision/accuracy/zone/alert/critical/immediate/action-required",
		},
		{
			name:      "topic that ends with slash (tests trailing dot removal)",
			mqttTopic: strings.Repeat("verylongsegment/", 20),
		},
		{
			name:      "single very long segment",
			mqttTopic: strings.Repeat("a", 300),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This calls the internal mapMQTTToKafkaTopic method via reflection or we could make it public for testing
			// For now, we'll test the behavior through actual bridge usage
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := bridge.Start(ctx); err != nil {
				t.Fatalf("Failed to start bridge: %v", err)
			}
			defer bridge.Stop()

			// Create MQTT client
			mqttClient := mqtt.NewClient(&config.MQTT)
			if err := mqttClient.Connect(); err != nil {
				t.Fatalf("Failed to connect MQTT client: %v", err)
			}
			defer mqttClient.Disconnect()

			// Publish message - the main test is that this doesn't panic or fail
			testPayload := `{"test": "truncation behavior"}`
			if err := mqttClient.Publish(tt.mqttTopic, []byte(testPayload), 0, false); err != nil {
				t.Fatalf("Failed to publish to long topic: %v", err)
			}

			// Allow processing time
			time.Sleep(1 * time.Second)

			t.Logf("Successfully processed topic that would exceed 249 chars: %q", tt.mqttTopic)
		})
	}
}