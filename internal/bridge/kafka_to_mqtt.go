package bridge

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"gom2k/internal/kafka"
	"gom2k/internal/mqtt"
	"gom2k/pkg/types"
)

// KafkaToMQTTBridge handles Kafka -> MQTT message flow
type KafkaToMQTTBridge struct {
	kafkaConsumer *kafka.Consumer
	mqttClient    *mqtt.Client
	config        *types.Config
	wg            sync.WaitGroup // For goroutine lifecycle management
	cancel        context.CancelFunc // To signal goroutine shutdown
	errorChan     chan error      // Channel to receive errors from goroutine
	deadLetterQueue *DeadLetterQueue // Dead letter queue for failed messages
}

// NewKafkaToMQTTBridge creates a new Kafka to MQTT bridge
func NewKafkaToMQTTBridge(config *types.Config) *KafkaToMQTTBridge {
	return &KafkaToMQTTBridge{
		config:    config,
		errorChan: make(chan error, 10), // Buffered channel for async error reporting
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

	// Initialize Kafka producer for dead letter queue (if DLQ is enabled)
	var kafkaProducer *kafka.Producer
	if b.config.Bridge.DeadLetter.Enabled && b.config.Bridge.DeadLetter.KafkaTopic != "" {
		kafkaProducer = kafka.NewProducer(&b.config.Kafka, &b.config.Bridge)
		if err := kafkaProducer.Connect(); err != nil {
			return fmt.Errorf("failed to connect Kafka producer for DLQ: %w", err)
		}
	}

	// Initialize dead letter queue  
	b.deadLetterQueue = NewDeadLetterQueue(&b.config.Bridge, kafkaProducer, b.mqttClient)
	if b.deadLetterQueue != nil {
		if err := b.deadLetterQueue.Start(); err != nil {
			return fmt.Errorf("failed to start dead letter queue: %w", err)
		}
	}
	
	log.Println("Kafka to MQTT bridge started successfully")
	
	// Create cancellable context for goroutine management
	goroutineCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	
	// Start consuming messages with proper lifecycle management
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.consumeMessages(goroutineCtx)
	}()
	
	return nil
}

// Stop gracefully shuts down the bridge
func (b *KafkaToMQTTBridge) Stop() error {
	log.Println("Stopping Kafka to MQTT bridge")
	
	// Signal goroutine to stop
	if b.cancel != nil {
		b.cancel()
	}
	
	// Wait for goroutine to finish
	b.wg.Wait()
	log.Println("Kafka consumer goroutine stopped")

	// Stop dead letter queue
	if b.deadLetterQueue != nil {
		if err := b.deadLetterQueue.Stop(); err != nil {
			log.Printf("Error stopping dead letter queue: %v", err)
		}
	}
	
	if b.kafkaConsumer != nil {
		if err := b.kafkaConsumer.Close(); err != nil {
			log.Printf("Error closing Kafka consumer: %v", err)
		}
	}
	
	if b.mqttClient != nil {
		b.mqttClient.Disconnect()
	}
	
	// Close error channel
	close(b.errorChan)
	
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
				b.reportError(fmt.Errorf("error reading from Kafka: %w", err))
				continue
			}
			
			// Convert and forward to MQTT
			if err := b.handleKafkaMessage(kafkaMsg); err != nil {
				b.reportError(fmt.Errorf("error handling Kafka message: %w", err))
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
		errorMsg := fmt.Errorf("failed to convert Kafka message: %w", err)
		if b.deadLetterQueue != nil {
			b.deadLetterQueue.HandleFailedMessage(kafkaMsg, errorMsg.Error(), "kafka-to-mqtt", kafkaMsg.Topic, "")
		}
		return errorMsg
	}
	
	// Validate the MQTT topic
	if mqttMsg.Topic == "" {
		errorMsg := fmt.Errorf("empty MQTT topic from Kafka message")
		if b.deadLetterQueue != nil {
			b.deadLetterQueue.HandleFailedMessage(kafkaMsg, errorMsg.Error(), "kafka-to-mqtt", kafkaMsg.Topic, "")
		}
		return errorMsg
	}
	
	// Check if this topic should be republished (avoid loops)
	if b.shouldSkipTopic(mqttMsg.Topic) {
		log.Printf("Skipping topic to prevent loop: %s", mqttMsg.Topic)
		return nil
	}
	
	// Publish to MQTT
	if err := b.mqttClient.Publish(mqttMsg.Topic, mqttMsg.Payload, mqttMsg.QoS, mqttMsg.Retained); err != nil {
		errorMsg := fmt.Errorf("failed to publish to MQTT: %w", err)
		if b.deadLetterQueue != nil {
			b.deadLetterQueue.HandleFailedMessage(kafkaMsg, errorMsg.Error(), "kafka-to-mqtt", kafkaMsg.Topic, mqttMsg.Topic)
		}
		return errorMsg
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

// reportError sends error to error channel for monitoring
func (b *KafkaToMQTTBridge) reportError(err error) {
	log.Printf("Kafka to MQTT bridge error: %v", err)
	
	// Try to send to error channel (non-blocking)
	select {
	case b.errorChan <- err:
	default:
		// Channel full or closed, log additional warning
		log.Printf("Warning: Error channel unavailable, dropping error report")
	}
}

// GetErrorChan returns the error channel for external monitoring
func (b *KafkaToMQTTBridge) GetErrorChan() <-chan error {
	return b.errorChan
}

