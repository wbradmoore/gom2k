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
	errorChan    chan error  // Channel to propagate errors from message handler
	errorCount   int         // Counter for failed messages
	deadLetterQueue *DeadLetterQueue // Dead letter queue for failed messages
}

// NewMQTTToKafkaBridge creates a new MQTT to Kafka bridge
func NewMQTTToKafkaBridge(config *types.Config) *MQTTToKafkaBridge {
	return &MQTTToKafkaBridge{
		config:    config,
		errorChan: make(chan error, 100), // Buffered channel for async error handling
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

	// Initialize dead letter queue
	b.deadLetterQueue = NewDeadLetterQueue(&b.config.Bridge, b.kafkaProducer, b.mqttClient)
	if b.deadLetterQueue != nil {
		if err := b.deadLetterQueue.Start(); err != nil {
			return fmt.Errorf("failed to start dead letter queue: %w", err)
		}
	}
	
	// Subscribe to MQTT topics
	if err := b.mqttClient.Subscribe(); err != nil {
		return fmt.Errorf("failed to subscribe to MQTT topics: %w", err)
	}
	
	// Start error monitoring goroutine
	go b.monitorErrors(ctx)
	
	log.Println("MQTT to Kafka bridge started successfully")
	return nil
}

// Stop gracefully shuts down the bridge
func (b *MQTTToKafkaBridge) Stop() error {
	log.Println("Stopping MQTT to Kafka bridge")
	
	// Stop dead letter queue first
	if b.deadLetterQueue != nil {
		if err := b.deadLetterQueue.Stop(); err != nil {
			log.Printf("Error stopping dead letter queue: %v", err)
		}
	}
	
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
		b.reportError(fmt.Errorf("failed to convert MQTT message from topic %s: %w", mqttMsg.Topic, err))
		if b.deadLetterQueue != nil {
			b.deadLetterQueue.HandleFailedMessage(mqttMsg, err.Error(), "mqtt-to-kafka", mqttMsg.Topic, kafkaTopic)
		}
		return
	}
	
	// Send to Kafka
	ctx := context.Background()
	if err := b.kafkaProducer.WriteMessage(ctx, kafkaMsg); err != nil {
		errorMsg := fmt.Errorf("failed to send message to Kafka topic %s: %w", kafkaTopic, err)
		b.reportError(errorMsg)
		if b.deadLetterQueue != nil {
			b.deadLetterQueue.HandleFailedMessage(mqttMsg, errorMsg.Error(), "mqtt-to-kafka", mqttMsg.Topic, kafkaTopic)
		}
		return
	}
	
	log.Printf("✓ Forwarded MQTT message: %s -> %s", mqttMsg.Topic, kafkaTopic)
}

// Map MQTT topic to Kafka topic using configured rules
func (b *MQTTToKafkaBridge) mapMQTTToKafkaTopic(mqttTopic string) string {
	// Use strings.Builder for efficient string concatenation
	var builder strings.Builder
	
	// Pre-allocate capacity (estimate: prefix + topic + separators)
	builder.Grow(len(b.config.Bridge.Mapping.KafkaPrefix) + len(mqttTopic) + 10)
	
	// Add prefix
	builder.WriteString(b.config.Bridge.Mapping.KafkaPrefix)
	
	// Process topic levels directly without creating intermediate slices
	maxLevels := b.config.Bridge.Mapping.MaxTopicLevels
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

// reportError sends error to error channel for monitoring
func (b *MQTTToKafkaBridge) reportError(err error) {
	b.errorCount++
	log.Printf("Bridge error #%d: %v", b.errorCount, err)
	
	// Try to send to error channel (non-blocking)
	select {
	case b.errorChan <- err:
	default:
		// Channel full, log additional warning
		log.Printf("Warning: Error channel full, dropping error report")
	}
}

// monitorErrors monitors the error channel and can trigger alerts or shutdown
func (b *MQTTToKafkaBridge) monitorErrors(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-b.errorChan:
			// For now, we just ensure errors are properly logged
			// In production, this could trigger alerts, circuit breakers, etc.
			log.Printf("Error monitoring: %v", err)
			
			// If error rate is too high, we could implement circuit breaker logic here
			if b.errorCount > 100 {
				log.Printf("WARNING: High error count (%d), consider investigating", b.errorCount)
			}
		}
	}
}

// GetErrorCount returns the current error count for monitoring
func (b *MQTTToKafkaBridge) GetErrorCount() int {
	return b.errorCount
}