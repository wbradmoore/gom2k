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

// MQTTToKafkaBridge handles MQTT -> Kafka message flow
type MQTTToKafkaBridge struct {
	mqttClient   *mqtt.Client
	kafkaProducer *kafka.Producer
	config       *types.Config
}

// NewMQTTToKafkaBridge creates a new MQTT to Kafka bridge
func NewMQTTToKafkaBridge(config *types.Config) *MQTTToKafkaBridge {
	return &MQTTToKafkaBridge{
		config: config,
	}
}

// Start initializes and starts the bridge
func (b *MQTTToKafkaBridge) Start(ctx context.Context) error {
	// Initialize MQTT client
	b.mqttClient = mqtt.NewClient(&b.config.MQTT)
	b.mqttClient.SetMessageHandler(b.handleMQTTMessage)
	
	if err := b.mqttClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect MQTT client: %w", err)
	}
	
	// Initialize Kafka producer
	b.kafkaProducer = kafka.NewProducer(&b.config.Kafka, &b.config.Bridge)
	if err := b.kafkaProducer.Connect(); err != nil {
		return fmt.Errorf("failed to connect Kafka producer: %w", err)
	}
	
	// Subscribe to MQTT topics
	if err := b.mqttClient.Subscribe(); err != nil {
		return fmt.Errorf("failed to subscribe to MQTT topics: %w", err)
	}
	
	log.Println("MQTT to Kafka bridge started successfully")
	return nil
}

// Stop gracefully shuts down the bridge
func (b *MQTTToKafkaBridge) Stop() error {
	log.Println("Stopping MQTT to Kafka bridge")
	
	if b.mqttClient != nil {
		b.mqttClient.Disconnect()
	}
	
	if b.kafkaProducer != nil {
		return b.kafkaProducer.Close()
	}
	
	return nil
}

// Handle incoming MQTT messages
func (b *MQTTToKafkaBridge) handleMQTTMessage(mqttMsg *types.MQTTMessage) {
	// Map MQTT topic to Kafka topic
	kafkaTopic := b.mapMQTTToKafkaTopic(mqttMsg.Topic)
	
	// Convert message
	kafkaMsg, err := kafka.ConvertMQTTMessage(mqttMsg, kafkaTopic)
	if err != nil {
		log.Printf("Error converting MQTT message: %v", err)
		return
	}
	
	// Send to Kafka
	ctx := context.Background()
	if err := b.kafkaProducer.WriteMessage(ctx, kafkaMsg); err != nil {
		log.Printf("Error sending message to Kafka: %v", err)
		return
	}
	
	log.Printf("✓ Forwarded MQTT message: %s -> %s", mqttMsg.Topic, kafkaTopic)
}

// Map MQTT topic to Kafka topic using configured rules
func (b *MQTTToKafkaBridge) mapMQTTToKafkaTopic(mqttTopic string) string {
	// Split topic into levels
	levels := strings.Split(mqttTopic, "/")
	
	// Apply max topic levels limit
	maxLevels := b.config.Bridge.Mapping.MaxTopicLevels
	if len(levels) > maxLevels {
		levels = levels[:maxLevels]
	}
	
	// Build Kafka topic with prefix
	prefix := b.config.Bridge.Mapping.KafkaPrefix
	kafkaTopicParts := append([]string{prefix}, levels...)
	
	// Join with dots for Kafka topic format
	kafkaTopic := strings.Join(kafkaTopicParts, ".")
	
	return kafkaTopic
}