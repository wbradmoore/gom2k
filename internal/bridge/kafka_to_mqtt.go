package bridge

import (
	"context"
	"fmt"
	"log"
	"strings"

	"gom2k/internal/kafka"
	"gom2k/internal/mqtt"
	"gom2k/pkg/types"
)

// KafkaToMQTTBridge handles Kafka -> MQTT message flow
type KafkaToMQTTBridge struct {
	kafkaConsumer *kafka.Consumer
	mqttClient    *mqtt.Client
	config        *types.Config
}

// NewKafkaToMQTTBridge creates a new Kafka to MQTT bridge
func NewKafkaToMQTTBridge(config *types.Config) *KafkaToMQTTBridge {
	return &KafkaToMQTTBridge{
		config: config,
	}
}

// Start initializes and starts the bridge
func (b *KafkaToMQTTBridge) Start(ctx context.Context) error {
	// Initialize Kafka consumer
	b.kafkaConsumer = kafka.NewConsumer(&b.config.Kafka, &b.config.Bridge)
	if err := b.kafkaConsumer.Connect(); err != nil {
		return fmt.Errorf("failed to connect Kafka consumer: %w", err)
	}
	
	// Initialize MQTT client
	b.mqttClient = mqtt.NewClient(&b.config.MQTT)
	if err := b.mqttClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect MQTT client: %w", err)
	}
	
	log.Println("Kafka to MQTT bridge started successfully")
	
	// Start consuming messages
	go b.consumeMessages(ctx)
	
	return nil
}

// Stop gracefully shuts down the bridge
func (b *KafkaToMQTTBridge) Stop() error {
	log.Println("Stopping Kafka to MQTT bridge")
	
	if b.kafkaConsumer != nil {
		if err := b.kafkaConsumer.Close(); err != nil {
			log.Printf("Error closing Kafka consumer: %v", err)
		}
	}
	
	if b.mqttClient != nil {
		b.mqttClient.Disconnect()
	}
	
	return nil
}

// consumeMessages continuously consumes messages from Kafka and forwards to MQTT
func (b *KafkaToMQTTBridge) consumeMessages(ctx context.Context) {
	log.Println("Starting Kafka message consumption...")
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Kafka consumer stopping due to context cancellation")
			return
		default:
			// Read message from Kafka
			kafkaMsg, err := b.kafkaConsumer.ReadMessage(ctx)
			if err != nil {
				log.Printf("Error reading from Kafka: %v", err)
				continue
			}
			
			// Convert and forward to MQTT
			if err := b.handleKafkaMessage(kafkaMsg); err != nil {
				log.Printf("Error handling Kafka message: %v", err)
				continue
			}
		}
	}
}

// handleKafkaMessage processes a Kafka message and forwards it to MQTT
func (b *KafkaToMQTTBridge) handleKafkaMessage(kafkaMsg *types.KafkaMessage) error {
	// Convert Kafka message back to MQTT format
	mqttMsg, err := kafka.ConvertKafkaMessage(kafkaMsg)
	if err != nil {
		return fmt.Errorf("failed to convert Kafka message: %w", err)
	}
	
	// Validate the MQTT topic
	if mqttMsg.Topic == "" {
		return fmt.Errorf("empty MQTT topic from Kafka message")
	}
	
	// Check if this topic should be republished (avoid loops)
	if b.shouldSkipTopic(mqttMsg.Topic) {
		log.Printf("Skipping topic to prevent loop: %s", mqttMsg.Topic)
		return nil
	}
	
	// Publish to MQTT
	if err := b.mqttClient.Publish(mqttMsg.Topic, mqttMsg.Payload, mqttMsg.QoS, mqttMsg.Retained); err != nil {
		return fmt.Errorf("failed to publish to MQTT: %w", err)
	}
	
	log.Printf("✓ Forwarded Kafka message: %s -> %s", kafkaMsg.Topic, mqttMsg.Topic)
	return nil
}

// shouldSkipTopic determines if a topic should be skipped to prevent message loops
func (b *KafkaToMQTTBridge) shouldSkipTopic(mqttTopic string) bool {
	// Skip certain system topics that might cause loops
	skipPrefixes := []string{
		"$SYS/",
		"gom2k/",  // Skip our own bridge topics
	}
	
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(mqttTopic, prefix) {
			return true
		}
	}
	
	return false
}

