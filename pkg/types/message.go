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

// FailedMessage represents a message that failed processing and should be sent to dead letter queue
type FailedMessage struct {
	OriginalMessage interface{} `json:"original_message"` // The original MQTT or Kafka message
	FailureReason   string      `json:"failure_reason"`   // Why the message failed
	AttemptCount    int         `json:"attempt_count"`    // Number of processing attempts
	FirstFailure    time.Time   `json:"first_failure"`    // When the message first failed
	LastAttempt     time.Time   `json:"last_attempt"`     // When the last attempt was made
	Direction       string      `json:"direction"`        // "mqtt-to-kafka" or "kafka-to-mqtt"
	OriginalTopic   string      `json:"original_topic"`   // The topic where message originated
	TargetTopic     string      `json:"target_topic"`     // The topic where message was being sent
}