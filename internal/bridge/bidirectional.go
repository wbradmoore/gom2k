// Package bridge provides bidirectional message forwarding between MQTT and Kafka systems.
// It includes components for MQTT→Kafka, Kafka→MQTT, and bidirectional bridges,
// handling message transformation, topic mapping, and error propagation.
package bridge

import (
	"context"
	"fmt"
	"log"
	"sync"

	"gom2k/pkg/types"
)

// BidirectionalBridge orchestrates both MQTT→Kafka and Kafka→MQTT message flows.
// It manages the lifecycle of both directional bridges and ensures proper
// initialization, operation, and shutdown of the complete bidirectional system.
type BidirectionalBridge struct {
	mqttToKafka *MQTTToKafkaBridge
	kafkaToMQTT *KafkaToMQTTBridge
	config      *types.Config
	wg          sync.WaitGroup
}

// NewBidirectionalBridge creates a new bidirectional bridge with the provided configuration.
// The bridge will handle message flows based on the feature flags in the configuration:
// - If MQTTToKafka is enabled, messages from MQTT topics will be forwarded to Kafka
// - If KafkaToMQTT is enabled, messages from Kafka topics will be forwarded to MQTT
// At least one direction must be enabled for the bridge to function.
func NewBidirectionalBridge(config *types.Config) *BidirectionalBridge {
	return &BidirectionalBridge{
		mqttToKafka: NewMQTTToKafkaBridge(config),
		kafkaToMQTT: NewKafkaToMQTTBridge(config),
		config:      config,
	}
}

// Start initializes and starts the bidirectional bridge components based on configuration.
// It launches goroutines for each enabled direction (MQTT→Kafka and/or Kafka→MQTT) and
// monitors their operation. The method blocks until the context is cancelled or an error
// occurs. At least one bridge direction must be enabled in the configuration.
func (b *BidirectionalBridge) Start(ctx context.Context) error {
	log.Println("Starting bidirectional MQTT-Kafka bridge...")

	// Start MQTT→Kafka bridge if enabled
	if b.config.Bridge.Features.MQTTToKafka {
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			if err := b.mqttToKafka.Start(ctx); err != nil {
				log.Printf("Error in MQTT→Kafka bridge: %v", err)
			}
		}()
		log.Println("✓ MQTT→Kafka bridge enabled")
	} else {
		log.Println("⚠ MQTT→Kafka bridge disabled")
	}

	// Start Kafka→MQTT bridge if enabled
	if b.config.Bridge.Features.KafkaToMQTT {
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			if err := b.kafkaToMQTT.Start(ctx); err != nil {
				log.Printf("Error in Kafka→MQTT bridge: %v", err)
			}
		}()
		log.Println("✓ Kafka→MQTT bridge enabled")
	} else {
		log.Println("⚠ Kafka→MQTT bridge disabled")
	}

	// Check if at least one direction is enabled
	if !b.config.Bridge.Features.MQTTToKafka && !b.config.Bridge.Features.KafkaToMQTT {
		return fmt.Errorf("no bridge directions enabled - check configuration")
	}

	log.Println("🚀 Bidirectional bridge started successfully")
	return nil
}

// Stop gracefully shuts down both bridge directions
func (b *BidirectionalBridge) Stop() error {
	log.Println("Stopping bidirectional bridge...")

	// Stop both bridges
	var err1, err2 error
	
	if b.mqttToKafka != nil {
		err1 = b.mqttToKafka.Stop()
		if err1 != nil {
			log.Printf("Error stopping MQTT→Kafka bridge: %v", err1)
		}
	}
	
	if b.kafkaToMQTT != nil {
		err2 = b.kafkaToMQTT.Stop()
		if err2 != nil {
			log.Printf("Error stopping Kafka→MQTT bridge: %v", err2)
		}
	}

	// Wait for all goroutines to finish
	b.wg.Wait()
	
	log.Println("✓ Bidirectional bridge stopped")

	// Return first error encountered
	if err1 != nil {
		return err1
	}
	return err2
}

// GetStatus returns the current operational status of both bridge directions.
// This includes whether each direction is enabled in configuration and the overall
// running state of the bridge system.
func (b *BidirectionalBridge) GetStatus() BridgeStatus {
	return BridgeStatus{
		MQTTToKafkaEnabled: b.config.Bridge.Features.MQTTToKafka,
		KafkaToMQTTEnabled: b.config.Bridge.Features.KafkaToMQTT,
		IsRunning:          true, // TODO: Add actual health checks
	}
}

// BridgeStatus represents the current operational status of the bidirectional bridge.
// It provides information about which bridge directions are enabled and whether
// the bridge system is currently running.
type BridgeStatus struct {
	MQTTToKafkaEnabled bool `json:"mqtt_to_kafka_enabled"` // Whether MQTT→Kafka flow is enabled
	KafkaToMQTTEnabled bool `json:"kafka_to_mqtt_enabled"` // Whether Kafka→MQTT flow is enabled
	IsRunning          bool `json:"is_running"`            // Overall bridge running status
}