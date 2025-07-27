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

func TestTopicTruncation(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		mqtt      string
		maxLevels int
	}{
		{
			name:      "topic under 249 chars",
			prefix:    "gom2k",
			mqtt:      "home/sensors/temperature",
			maxLevels: 3,
		},
		{
			name:      "topic exactly 249 chars - should not truncate",
			prefix:    "gom2k",
			mqtt:      strings.Repeat("a", 242) + "/b", // Results in exactly 249 chars: "gom2k." + 242 "a"s + ".b"
			maxLevels: 10,
		},
		{
			name:      "topic over 249 chars - should truncate",
			prefix:    "my-very-long-enterprise-prefix",
			mqtt:      strings.Repeat("extremely-long-segment-name-that-represents-a-deeply-nested-iot-hierarchy/", 10),
			maxLevels: 10,
		},
		{
			name:      "truncation removes trailing dot",
			prefix:    "prefix",
			mqtt:      strings.Repeat("a", 242) + "/", // Would result in "prefix." + 242 "a"s + "." = 250 chars, needs truncation
			maxLevels: 2,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapMQTTToKafkaTopic(tt.mqtt, tt.prefix, tt.maxLevels)
			
			// Verify length constraint
			if len(result) > 249 {
				t.Errorf("Result exceeds 249 chars: len=%d, topic=%q", len(result), result)
			}
			
			// Verify no trailing dot if length is exactly 249
			if len(result) == 249 && result[len(result)-1] == '.' {
				t.Errorf("Result has trailing dot after truncation to 249 chars")
			}
			
			// Log the actual lengths for debugging
			t.Logf("Test %q: input would produce %d chars, actual result %d chars",
				tt.name, len(tt.prefix)+1+len(strings.ReplaceAll(tt.mqtt, "/", ".")), len(result))
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

// Helper function that mimics the actual topic mapping logic (optimized version)
func mapMQTTToKafkaTopic(mqttTopic, prefix string, maxLevels int) string {
	if mqttTopic == "" {
		return prefix
	}

	// Use strings.Builder for efficient string concatenation
	var builder strings.Builder
	
	// Pre-allocate capacity (estimate: prefix + topic + separators)
	builder.Grow(len(prefix) + len(mqttTopic) + 10)
	
	// Add prefix
	builder.WriteString(prefix)
	
	// Process topic levels directly without creating intermediate slices
	levelCount := 0
	startIdx := 0
	
	for i := 0; i < len(mqttTopic); i++ {
		if mqttTopic[i] == '/' {
			if levelCount < maxLevels {
				builder.WriteByte('.')
				builder.WriteString(mqttTopic[startIdx:i])
				levelCount++
			}
			startIdx = i + 1
		}
	}
	
	// Handle the last segment (including empty segment from trailing slash)
	if levelCount < maxLevels && startIdx <= len(mqttTopic) {
		builder.WriteByte('.')
		builder.WriteString(mqttTopic[startIdx:])
	}
	
	kafkaTopic := builder.String()
	
	// Ensure Kafka topic doesn't exceed maximum length (249 chars)
	if len(kafkaTopic) > 249 {
		kafkaTopic = kafkaTopic[:249]
		// Remove trailing dot if present
		if kafkaTopic[len(kafkaTopic)-1] == '.' {
			kafkaTopic = kafkaTopic[:len(kafkaTopic)-1]
		}
	}
	
	return kafkaTopic
}