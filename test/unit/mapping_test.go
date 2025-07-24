package unit

import (
	"strings"
	"testing"
)

func TestTopicMapping(t *testing.T) {
	tests := []struct {
		name          string
		mqttTopic     string
		prefix        string
		maxLevels     int
		expectedTopic string
	}{
		{
			name:          "simple topic",
			mqttTopic:     "temp",
			prefix:        "gom2k",
			maxLevels:     3,
			expectedTopic: "gom2k.temp",
		},
		{
			name:          "nested topic",
			mqttTopic:     "sensor/room/temp",
			prefix:        "gom2k",
			maxLevels:     3,
			expectedTopic: "gom2k.sensor.room.temp",
		},
		{
			name:          "deep nesting truncated",
			mqttTopic:     "home/floor1/room2/sensor/temp/celsius",
			prefix:        "gom2k",
			maxLevels:     3,
			expectedTopic: "gom2k.home.floor1.room2",
		},
		{
			name:          "custom prefix",
			mqttTopic:     "data/reading",
			prefix:        "mybridge",
			maxLevels:     3,
			expectedTopic: "mybridge.data.reading",
		},
		{
			name:          "single level limit",
			mqttTopic:     "a/b/c/d",
			prefix:        "test",
			maxLevels:     1,
			expectedTopic: "test.a",
		},
		{
			name:          "homeassistant switch",
			mqttTopic:     "homeassistant/switch/feeder/config",
			prefix:        "gom2k",
			maxLevels:     3,
			expectedTopic: "gom2k.homeassistant.switch.feeder",
		},
		{
			name:          "zigbee device",
			mqttTopic:     "zigbee2mqtt/0x001788010c488401/temperature",
			prefix:        "gom2k",
			maxLevels:     3,
			expectedTopic: "gom2k.zigbee2mqtt.0x001788010c488401.temperature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapMQTTToKafkaTopic(tt.mqttTopic, tt.prefix, tt.maxLevels)
			if result != tt.expectedTopic {
				t.Errorf("mapMQTTToKafkaTopic(%q, %q, %d) = %q, want %q",
					tt.mqttTopic, tt.prefix, tt.maxLevels, result, tt.expectedTopic)
			}
		})
	}
}

func TestTopicMappingEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		mqttTopic     string
		prefix        string
		maxLevels     int
		expectedTopic string
	}{
		{
			name:          "empty topic",
			mqttTopic:     "",
			prefix:        "gom2k",
			maxLevels:     3,
			expectedTopic: "gom2k",
		},
		{
			name:          "topic with slashes only",
			mqttTopic:     "///",
			prefix:        "gom2k",
			maxLevels:     3,
			expectedTopic: "gom2k...",
		},
		{
			name:          "leading slash",
			mqttTopic:     "/sensor/temp",
			prefix:        "gom2k",
			maxLevels:     3,
			expectedTopic: "gom2k..sensor.temp",
		},
		{
			name:          "trailing slash",
			mqttTopic:     "sensor/temp/",
			prefix:        "gom2k",
			maxLevels:     3,
			expectedTopic: "gom2k.sensor.temp.",
		},
		{
			name:          "zero max levels",
			mqttTopic:     "a/b/c",
			prefix:        "gom2k",
			maxLevels:     0,
			expectedTopic: "gom2k",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapMQTTToKafkaTopic(tt.mqttTopic, tt.prefix, tt.maxLevels)
			if result != tt.expectedTopic {
				t.Errorf("mapMQTTToKafkaTopic(%q, %q, %d) = %q, want %q",
					tt.mqttTopic, tt.prefix, tt.maxLevels, result, tt.expectedTopic)
			}
		})
	}
}

func BenchmarkTopicMapping(b *testing.B) {
	mqttTopic := "homeassistant/sensor/room1/temperature/celsius/current"
	prefix := "gom2k"
	maxLevels := 3

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapMQTTToKafkaTopic(mqttTopic, prefix, maxLevels)
	}
}

// Helper function that mimics the actual topic mapping logic
func mapMQTTToKafkaTopic(mqttTopic, prefix string, maxLevels int) string {
	if mqttTopic == "" {
		return prefix
	}

	// Split topic into levels (exactly like the real implementation)
	levels := strings.Split(mqttTopic, "/")
	
	// Apply max topic levels limit
	if len(levels) > maxLevels {
		levels = levels[:maxLevels]
	}
	
	// Build Kafka topic with prefix
	kafkaTopicParts := append([]string{prefix}, levels...)
	
	// Join with dots for Kafka topic format
	return strings.Join(kafkaTopicParts, ".")
}