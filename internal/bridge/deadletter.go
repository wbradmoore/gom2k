// Package bridge implements the core message forwarding logic between MQTT and Kafka systems.
// This file contains the dead letter queue implementation for handling messages that fail processing.
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"gom2k/internal/kafka"
	"gom2k/internal/mqtt"
	"gom2k/pkg/types"
)

// DeadLetterQueue handles messages that fail processing after retries
type DeadLetterQueue struct {
	config        *types.BridgeConfig
	kafkaProducer *kafka.Producer
	mqttClient    *mqtt.Client
	
	// Message tracking for retries
	failedMessages map[string]*types.FailedMessage
	messageMutex   sync.RWMutex
	
	// Retry processing
	retryTicker *time.Ticker
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// NewDeadLetterQueue creates a new dead letter queue handler
func NewDeadLetterQueue(config *types.BridgeConfig, kafkaProducer *kafka.Producer, mqttClient *mqtt.Client) *DeadLetterQueue {
	if !config.DeadLetter.Enabled {
		return nil
	}
	
	return &DeadLetterQueue{
		config:         config,
		kafkaProducer:  kafkaProducer,
		mqttClient:     mqttClient,
		failedMessages: make(map[string]*types.FailedMessage),
		stopChan:       make(chan struct{}),
	}
}

// Start begins the dead letter queue processing
func (dlq *DeadLetterQueue) Start() error {
	if dlq == nil || !dlq.config.DeadLetter.Enabled {
		return nil
	}
	
	log.Printf("Starting dead letter queue with retry interval: %v", dlq.config.DeadLetter.RetryInterval)
	
	// Start retry processing goroutine
	dlq.retryTicker = time.NewTicker(dlq.config.DeadLetter.RetryInterval)
	dlq.wg.Add(1)
	go dlq.processRetries()
	
	return nil
}

// Stop stops the dead letter queue processing
func (dlq *DeadLetterQueue) Stop() error {
	if dlq == nil || dlq.retryTicker == nil {
		return nil
	}
	
	log.Println("Stopping dead letter queue")
	
	// Stop retry processing
	close(dlq.stopChan)
	dlq.retryTicker.Stop()
	dlq.wg.Wait()
	
	return nil
}

// HandleFailedMessage processes a message that failed and determines if it should be retried or sent to DLQ
func (dlq *DeadLetterQueue) HandleFailedMessage(originalMsg interface{}, failureReason string, direction string, originalTopic string, targetTopic string) {
	if dlq == nil || !dlq.config.DeadLetter.Enabled {
		// Just log the error if DLQ is disabled
		log.Printf("Message failed processing (DLQ disabled): %s -> %s: %s", originalTopic, targetTopic, failureReason)
		return
	}
	
	// Create unique key for message tracking
	messageKey := dlq.createMessageKey(originalMsg, direction, originalTopic)
	
	dlq.messageMutex.Lock()
	defer dlq.messageMutex.Unlock()
	
	failedMsg, exists := dlq.failedMessages[messageKey]
	if !exists {
		// First failure - create new failed message record
		failedMsg = &types.FailedMessage{
			OriginalMessage: originalMsg,
			FailureReason:   failureReason,
			AttemptCount:    1,
			FirstFailure:    time.Now(),
			LastAttempt:     time.Now(),
			Direction:       direction,
			OriginalTopic:   originalTopic,
			TargetTopic:     targetTopic,
		}
		dlq.failedMessages[messageKey] = failedMsg
		log.Printf("Added message to retry queue (attempt 1/%d): %s", dlq.config.DeadLetter.MaxRetries, failureReason)
	} else {
		// Subsequent failure - update existing record
		failedMsg.AttemptCount++
		failedMsg.LastAttempt = time.Now()
		failedMsg.FailureReason = failureReason // Update with latest error
		log.Printf("Message retry failed (attempt %d/%d): %s", failedMsg.AttemptCount, dlq.config.DeadLetter.MaxRetries, failureReason)
	}
	
	// Check if we've exceeded max retries
	if failedMsg.AttemptCount >= dlq.config.DeadLetter.MaxRetries {
		log.Printf("Message exceeded max retries, sending to dead letter queue: %s", failureReason)
		dlq.sendToDeadLetterQueue(failedMsg)
		delete(dlq.failedMessages, messageKey)
	}
}

// processRetries periodically attempts to reprocess failed messages
func (dlq *DeadLetterQueue) processRetries() {
	defer dlq.wg.Done()
	
	for {
		select {
		case <-dlq.stopChan:
			return
		case <-dlq.retryTicker.C:
			dlq.retryFailedMessages()
		}
	}
}

// retryFailedMessages attempts to reprocess all failed messages
func (dlq *DeadLetterQueue) retryFailedMessages() {
	dlq.messageMutex.Lock()
	messagesToRetry := make([]*types.FailedMessage, 0, len(dlq.failedMessages))
	for _, msg := range dlq.failedMessages {
		messagesToRetry = append(messagesToRetry, msg)
	}
	dlq.messageMutex.Unlock()
	
	for _, failedMsg := range messagesToRetry {
		// Only retry if enough time has passed since last attempt
		if time.Since(failedMsg.LastAttempt) >= dlq.config.DeadLetter.RetryInterval {
			dlq.retryMessage(failedMsg)
		}
	}
}

// retryMessage attempts to reprocess a single failed message
func (dlq *DeadLetterQueue) retryMessage(failedMsg *types.FailedMessage) {
	log.Printf("Retrying failed message (attempt %d): %s -> %s", failedMsg.AttemptCount+1, failedMsg.OriginalTopic, failedMsg.TargetTopic)
	
	var err error
	switch failedMsg.Direction {
	case "mqtt-to-kafka":
		err = dlq.retryMQTTToKafka(failedMsg)
	case "kafka-to-mqtt":
		err = dlq.retryKafkaToMQTT(failedMsg)
	default:
		err = fmt.Errorf("unknown direction: %s", failedMsg.Direction)
	}
	
	if err != nil {
		// Retry failed, update failure info
		dlq.HandleFailedMessage(failedMsg.OriginalMessage, err.Error(), failedMsg.Direction, failedMsg.OriginalTopic, failedMsg.TargetTopic)
	} else {
		// Retry succeeded, remove from failed messages
		messageKey := dlq.createMessageKey(failedMsg.OriginalMessage, failedMsg.Direction, failedMsg.OriginalTopic)
		dlq.messageMutex.Lock()
		delete(dlq.failedMessages, messageKey)
		dlq.messageMutex.Unlock()
		log.Printf("✓ Retry successful: %s -> %s", failedMsg.OriginalTopic, failedMsg.TargetTopic)
	}
}

// retryMQTTToKafka retries sending an MQTT message to Kafka
func (dlq *DeadLetterQueue) retryMQTTToKafka(failedMsg *types.FailedMessage) error {
	mqttMsg, ok := failedMsg.OriginalMessage.(*types.MQTTMessage)
	if !ok {
		return fmt.Errorf("invalid MQTT message type for retry")
	}
	
	// Convert and send to Kafka
	kafkaMsg, err := kafka.ConvertMQTTMessage(mqttMsg, failedMsg.TargetTopic)
	if err != nil {
		return fmt.Errorf("retry: failed to convert MQTT message: %w", err)
	}
	
	ctx := context.Background()
	if err := dlq.kafkaProducer.WriteMessage(ctx, kafkaMsg); err != nil {
		return fmt.Errorf("retry: failed to send to Kafka: %w", err)
	}
	
	return nil
}

// retryKafkaToMQTT retries sending a Kafka message to MQTT
func (dlq *DeadLetterQueue) retryKafkaToMQTT(failedMsg *types.FailedMessage) error {
	kafkaMsg, ok := failedMsg.OriginalMessage.(*types.KafkaMessage)
	if !ok {
		return fmt.Errorf("invalid Kafka message type for retry")
	}
	
	// Convert and send to MQTT
	mqttMsg, err := kafka.ConvertKafkaMessage(kafkaMsg)
	if err != nil {
		return fmt.Errorf("retry: failed to convert Kafka message: %w", err)
	}
	
	if err := dlq.mqttClient.Publish(mqttMsg.Topic, mqttMsg.Payload, mqttMsg.QoS, mqttMsg.Retained); err != nil {
		return fmt.Errorf("retry: failed to publish to MQTT: %w", err)
	}
	
	return nil
}

// sendToDeadLetterQueue sends a failed message to the configured dead letter topics
func (dlq *DeadLetterQueue) sendToDeadLetterQueue(failedMsg *types.FailedMessage) {
	// Serialize the failed message
	dlqPayload, err := json.Marshal(failedMsg)
	if err != nil {
		log.Printf("Error serializing failed message for DLQ: %v", err)
		return
	}
	
	// Send to Kafka dead letter topic if configured and producer is available
	if dlq.config.DeadLetter.KafkaTopic != "" && dlq.kafkaProducer != nil {
		kafkaMsg := &types.KafkaMessage{
			Key:   fmt.Sprintf("dlq-%s-%d", failedMsg.Direction, time.Now().Unix()),
			Value: dlqPayload,
			Topic: dlq.config.DeadLetter.KafkaTopic,
		}
		
		ctx := context.Background()
		if err := dlq.kafkaProducer.WriteMessage(ctx, kafkaMsg); err != nil {
			log.Printf("Error sending failed message to Kafka DLQ: %v", err)
		} else {
			log.Printf("✓ Sent failed message to Kafka DLQ: %s", dlq.config.DeadLetter.KafkaTopic)
		}
	}
	
	// Send to MQTT dead letter topic if configured and client is available
	if dlq.config.DeadLetter.MQTTTopic != "" && dlq.mqttClient != nil {
		if err := dlq.mqttClient.Publish(dlq.config.DeadLetter.MQTTTopic, dlqPayload, 1, false); err != nil {
			log.Printf("Error sending failed message to MQTT DLQ: %v", err)
		} else {
			log.Printf("✓ Sent failed message to MQTT DLQ: %s", dlq.config.DeadLetter.MQTTTopic)
		}
	}
}

// createMessageKey creates a unique key for tracking failed messages
func (dlq *DeadLetterQueue) createMessageKey(originalMsg interface{}, direction string, originalTopic string) string {
	switch msg := originalMsg.(type) {
	case *types.MQTTMessage:
		return fmt.Sprintf("%s-%s-%d", direction, originalTopic, msg.Timestamp.Unix())
	case *types.KafkaMessage:
		return fmt.Sprintf("%s-%s-%s", direction, originalTopic, msg.Key)
	default:
		return fmt.Sprintf("%s-%s-%d", direction, originalTopic, time.Now().Unix())
	}
}

// GetFailedMessageCount returns the number of messages currently in retry queue
func (dlq *DeadLetterQueue) GetFailedMessageCount() int {
	if dlq == nil {
		return 0
	}
	
	dlq.messageMutex.RLock()
	defer dlq.messageMutex.RUnlock()
	return len(dlq.failedMessages)
}