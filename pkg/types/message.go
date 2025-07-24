package types

import "time"

// MQTTMessage represents an MQTT message with metadata
type MQTTMessage struct {
	Topic     string    `json:"mqtt_topic"`
	Payload   []byte    `json:"payload"`
	QoS       byte      `json:"qos"`
	Retained  bool      `json:"retained"`
	Timestamp time.Time `json:"timestamp"`
}

// KafkaMessage represents a Kafka message
type KafkaMessage struct {
	Key   string
	Value []byte
	Topic string
}