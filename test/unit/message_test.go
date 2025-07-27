package unit

import (
	"encoding/json"
	"testing"
	"time"

	"gom2k/internal/kafka"
	"gom2k/pkg/types"
)

func TestMQTTToKafkaMessageConversion(t *testing.T) {
	timestamp := time.Now()
	
	mqttMsg := &types.MQTTMessage{
		Topic:     "sensor/room1/temp",
		Payload:   []byte("23.5"),
		QoS:       1,
		Retained:  true,
		Timestamp: timestamp,
	}

	kafkaTopic := "gom2k.sensor.room1"
	
	kafkaMsg, err := convertMQTTMessage(mqttMsg, kafkaTopic)
	if err != nil {
		t.Fatalf("Failed to convert MQTT message: %v", err)
	}

	// Verify Kafka message structure
	if kafkaMsg.Key != mqttMsg.Topic {
		t.Errorf("Expected key %q, got %q", mqttMsg.Topic, kafkaMsg.Key)
	}

	if kafkaMsg.Topic != kafkaTopic {
		t.Errorf("Expected topic %q, got %q", kafkaTopic, kafkaMsg.Topic)
	}

	// Parse JSON payload
	var payload map[string]interface{}
	if err := json.Unmarshal(kafkaMsg.Value, &payload); err != nil {
		t.Fatalf("Failed to parse JSON payload: %v", err)
	}

	// Verify payload content
	if payload["payload"] != "23.5" {
		t.Errorf("Expected payload '23.5', got %v", payload["payload"])
	}

	if payload["qos"] != float64(1) { // JSON unmarshals numbers as float64
		t.Errorf("Expected QoS 1, got %v", payload["qos"])
	}

	if payload["retained"] != true {
		t.Errorf("Expected retained true, got %v", payload["retained"])
	}

	if payload["mqtt_topic"] != mqttMsg.Topic {
		t.Errorf("Expected mqtt_topic %q, got %v", mqttMsg.Topic, payload["mqtt_topic"])
	}

	// Verify timestamp is present
	if _, ok := payload["timestamp"]; !ok {
		t.Error("Expected timestamp in payload")
	}
}

func TestKafkaToMQTTMessageConversion(t *testing.T) {
	// Create a Kafka message that represents an MQTT message
	originalTopic := "sensor/room1/temp"
	kafkaTopic := "gom2k.sensor.room1"
	
	payload := map[string]interface{}{
		"payload":    "23.5",
		"timestamp":  time.Now().Format(time.RFC3339),
		"qos":        1,
		"retained":   true,
		"mqtt_topic": originalTopic,
	}
	
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal test payload: %v", err)
	}
	
	kafkaMsg := &types.KafkaMessage{
		Key:   originalTopic,
		Value: jsonPayload,
		Topic: kafkaTopic,
	}
	
	// Convert back to MQTT message
	mqttMsg, err := kafka.ConvertKafkaMessage(kafkaMsg)
	if err != nil {
		t.Fatalf("Failed to convert Kafka message: %v", err)
	}
	
	// Verify MQTT message
	if mqttMsg.Topic != originalTopic {
		t.Errorf("Expected topic %q, got %q", originalTopic, mqttMsg.Topic)
	}
	
	if string(mqttMsg.Payload) != "23.5" {
		t.Errorf("Expected payload '23.5', got %q", string(mqttMsg.Payload))
	}
	
	if mqttMsg.QoS != 1 {
		t.Errorf("Expected QoS 1, got %d", mqttMsg.QoS)
	}
	
	if mqttMsg.Retained != true {
		t.Errorf("Expected retained true, got %v", mqttMsg.Retained)
	}
}

func TestMessageConversionRoundTrip(t *testing.T) {
	// Original MQTT message
	original := &types.MQTTMessage{
		Topic:     "homeassistant/switch/feeder/state",
		Payload:   []byte("ON"),
		QoS:       0,
		Retained:  false,
		Timestamp: time.Now(),
	}
	
	kafkaTopic := "gom2k.homeassistant.switch"
	
	// Convert MQTT -> Kafka
	kafkaMsg, err := convertMQTTMessage(original, kafkaTopic)
	if err != nil {
		t.Fatalf("MQTT->Kafka conversion failed: %v", err)
	}
	
	// Convert Kafka -> MQTT
	restored, err := kafka.ConvertKafkaMessage(kafkaMsg)
	if err != nil {
		t.Fatalf("Kafka->MQTT conversion failed: %v", err)
	}
	
	// Verify round-trip preservation
	if restored.Topic != original.Topic {
		t.Errorf("Topic mismatch: %q != %q", restored.Topic, original.Topic)
	}
	
	if string(restored.Payload) != string(original.Payload) {
		t.Errorf("Payload mismatch: %q != %q", string(restored.Payload), string(original.Payload))
	}
	
	if restored.QoS != original.QoS {
		t.Errorf("QoS mismatch: %d != %d", restored.QoS, original.QoS)
	}
	
	if restored.Retained != original.Retained {
		t.Errorf("Retained mismatch: %v != %v", restored.Retained, original.Retained)
	}
}

func BenchmarkMQTTToKafkaConversion(b *testing.B) {
	mqttMsg := &types.MQTTMessage{
		Topic:     "sensor/room1/temperature",
		Payload:   []byte("23.5"),
		QoS:       1,
		Retained:  true,
		Timestamp: time.Now(),
	}
	
	kafkaTopic := "gom2k.sensor.room1"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := convertMQTTMessage(mqttMsg, kafkaTopic)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper functions that mimic the actual conversion logic

func convertMQTTMessage(mqttMsg *types.MQTTMessage, kafkaTopic string) (*types.KafkaMessage, error) {
	// Create JSON payload with metadata
	payload := map[string]interface{}{
		"payload":    string(mqttMsg.Payload),
		"timestamp":  mqttMsg.Timestamp.Format(time.RFC3339),
		"qos":        mqttMsg.QoS,
		"retained":   mqttMsg.Retained,
		"mqtt_topic": mqttMsg.Topic,
	}
	
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	
	return &types.KafkaMessage{
		Key:   mqttMsg.Topic, // Use MQTT topic as Kafka key for partitioning
		Value: jsonPayload,
		Topic: kafkaTopic,
	}, nil
}

// TestKafkaToMQTTTopicReconstruction tests that MQTT topics are always reconstructed from JSON payload
func TestKafkaToMQTTTopicReconstruction(t *testing.T) {
	tests := []struct {
		name          string
		originalTopic string
		kafkaTopic    string
		kafkaKey      string
		expectError   bool
	}{
		{
			name:          "Standard topic reconstruction",
			originalTopic: "sensors/room1/temperature",
			kafkaTopic:    "gom2k.sensors.room1",
			kafkaKey:      "sensors/room1/temperature",
			expectError:   false,
		},
		{
			name:          "Very long topic that would exceed Kafka limits",
			originalTopic: "azeroth/eastern-kingdoms/stormwind/elwynn-forest/deadmines/instance-42/van-cleef-hideout/defias-brotherhood/edwin-vancleef/loot-table/rare-drops/cruel-barb/stats/damage/min-max/enchantments/current",
			kafkaTopic:    "gom2k.azeroth.eastern-kingdoms.stormwind", // Truncated
			kafkaKey:      "wrong-key", // Deliberately wrong to ensure JSON is used
			expectError:   false,
		},
		{
			name:          "Topic with special characters",
			originalTopic: "home/kitchen/sensor#1/temp-reading",
			kafkaTopic:    "gom2k.home.kitchen",
			kafkaKey:      "", // Empty key to verify JSON is used
			expectError:   false,
		},
		{
			name:          "Missing mqtt_topic in payload",
			originalTopic: "",
			kafkaTopic:    "gom2k.test",
			kafkaKey:      "test/topic",
			expectError:   true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create payload with or without mqtt_topic based on test case
			payload := map[string]interface{}{
				"payload":   "test-data",
				"timestamp": time.Now().Format(time.RFC3339),
				"qos":       0,
				"retained":  false,
			}
			
			if tt.originalTopic != "" {
				payload["mqtt_topic"] = tt.originalTopic
			}
			
			jsonPayload, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("Failed to marshal test payload: %v", err)
			}
			
			kafkaMsg := &types.KafkaMessage{
				Key:   tt.kafkaKey,
				Value: jsonPayload,
				Topic: tt.kafkaTopic,
			}
			
			// Convert back to MQTT message
			mqttMsg, err := kafka.ConvertKafkaMessage(kafkaMsg)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			// Verify topic is reconstructed from JSON payload, not from key
			if mqttMsg.Topic != tt.originalTopic {
				t.Errorf("Expected topic %q, got %q", tt.originalTopic, mqttMsg.Topic)
			}
		})
	}
}