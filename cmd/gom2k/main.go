package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gom2k/internal/bridge"
	"gom2k/internal/config"
	"gom2k/internal/kafka"
	"gom2k/internal/mqtt"
	"gom2k/pkg/types"
)

func main() {
	fmt.Println("GOM2K MQTT-Kafka Bridge")
	fmt.Println("Version: 0.1.0")
	
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		os.Exit(0)
	}
	
	// Test mode to verify MQTT connectivity
	if len(os.Args) > 1 && os.Args[1] == "--test-mqtt" {
		testMQTTConnectivity()
		return
	}
	
	// Test mode to verify Kafka connectivity
	if len(os.Args) > 1 && os.Args[1] == "--test-kafka" {
		testKafkaConnectivity()
		return
	}
	
	// Test mode for auto-topic creation
	if len(os.Args) > 1 && os.Args[1] == "--test-topics" {
		testTopicCreation()
		return
	}
	
	// Load configuration
	configPath := config.GetConfigPath()
	log.Printf("Loading configuration from: %s", configPath)
	bridgeConfig, err := config.LoadFromFile(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	log.Printf("Configuration loaded. MQTT: %s:%d, Kafka: %v", 
		bridgeConfig.MQTT.Broker.Host, bridgeConfig.MQTT.Broker.Port, bridgeConfig.Kafka.Brokers)
	log.Printf("Bridge features: MQTT→Kafka=%v, Kafka→MQTT=%v", 
		bridgeConfig.Bridge.Features.MQTTToKafka, bridgeConfig.Bridge.Features.KafkaToMQTT)
	
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Initialize bidirectional bridge
	log.Println("Initializing bidirectional MQTT-Kafka bridge...")
	bridgeInstance := bridge.NewBidirectionalBridge(bridgeConfig)
	
	if err := bridgeInstance.Start(ctx); err != nil {
		log.Fatalf("Failed to start bridge: %v", err)
	}
	
	log.Println("Bridge started successfully")
	
	// Wait for interrupt signal
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	
	<-signalCh
	log.Println("Received shutdown signal")
	
	// Graceful shutdown
	if err := bridgeInstance.Stop(); err != nil {
		log.Printf("Error stopping bridge: %v", err)
	}
	
	log.Println("Bridge stopped")
}

func testMQTTConnectivity() {
	log.Println("Testing MQTT connectivity...")
	
	// Load configuration for testing (skips validation)
	configPath := config.GetConfigPath()
	mqttConfig, err := config.LoadForTesting(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	log.Printf("Debug - MQTT config: Host=%s, Port=%d, TLS=%v, Auth=%s", 
		mqttConfig.MQTT.Broker.Host, mqttConfig.MQTT.Broker.Port, mqttConfig.MQTT.Broker.UseTLS, mqttConfig.MQTT.Auth.Username)
	
	messageCount := 0
	maxMessages := 3
	
	// Create MQTT client
	client := mqtt.NewClient(&mqttConfig.MQTT)
	client.SetMessageHandler(func(msg *types.MQTTMessage) {
		messageCount++
		fmt.Printf("Message %d:\n", messageCount)
		fmt.Printf("  Topic: %s\n", msg.Topic)
		fmt.Printf("  Payload: %s\n", string(msg.Payload))
		fmt.Printf("  QoS: %d, Retained: %t\n", msg.QoS, msg.Retained)
		fmt.Printf("  Timestamp: %s\n", msg.Timestamp.Format(time.RFC3339))
		fmt.Println("---")
		
		if messageCount >= maxMessages {
			log.Printf("Received %d messages, disconnecting...", maxMessages)
			client.Disconnect()
		}
	})
	
	// Connect
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect to MQTT: %v", err)
	}
	
	// Subscribe
	if err := client.Subscribe(); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	
	log.Printf("Connected! Waiting for %d messages (30 second timeout)...", maxMessages)
	
	// Wait for messages or timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			log.Println("Timeout reached, no messages received")
			client.Disconnect()
			return
		case <-ticker.C:
			if messageCount >= maxMessages {
				return
			}
		}
	}
}

func testKafkaConnectivity() {
	log.Println("Testing Kafka connectivity...")
	
	// Load configuration for testing (skips validation)
	configPath := config.GetConfigPath()
	kafkaConfig, err := config.LoadForTesting(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	log.Printf("Kafka brokers: %v", kafkaConfig.Kafka.Brokers)
	log.Printf("Security protocol: %s", kafkaConfig.Kafka.Security.Protocol)
	
	// Create Kafka producer  
	producer := kafka.NewProducer(&kafkaConfig.Kafka, &kafkaConfig.Bridge)
	if err := producer.Connect(); err != nil {
		log.Fatalf("Failed to connect to Kafka: %v", err)
	}
	defer producer.Close()
	
	log.Println("Successfully connected to Kafka broker")
	
	// Create a copy of bridge config for testing to avoid mutation
	testBridgeConfig := kafkaConfig.Bridge
	testBridgeConfig.Kafka = kafkaConfig.Bridge.Kafka  // Copy Kafka struct
	testBridgeConfig.Kafka.AutoCreateTopics = true
	
	// Recreate producer with test config to avoid modifying original
	producer.Close()
	producer = kafka.NewProducer(&kafkaConfig.Kafka, &testBridgeConfig)
	if err := producer.Connect(); err != nil {
		log.Fatalf("Failed to reconnect Kafka producer with test config: %v", err)
	}
	defer producer.Close()
	
	// Test message
	testTopic := "gom2k.test"
	testMessage := &types.KafkaMessage{
		Key:   "test-key",
		Value: []byte(`{"test": "message", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`),
		Topic: testTopic,
	}
	
	log.Printf("Sending test message to topic: %s", testTopic)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := producer.WriteMessage(ctx, testMessage); err != nil {
		log.Fatalf("Failed to send test message: %v", err)
	}
	
	log.Println("✓ Successfully sent test message to Kafka!")
	log.Printf("Message: key=%s, topic=%s", testMessage.Key, testMessage.Topic)
	log.Printf("Payload: %s", string(testMessage.Value))
}

func testTopicCreation() {
	log.Println("Testing auto-topic creation...")
	
	// Load configuration for testing (skips validation)
	configPath := config.GetConfigPath()
	topicConfig, err := config.LoadForTesting(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	// Create a copy of bridge config for testing to avoid mutation
	testBridgeConfig := topicConfig.Bridge
	testBridgeConfig.Kafka = topicConfig.Bridge.Kafka  // Copy Kafka struct
	testBridgeConfig.Kafka.AutoCreateTopics = true
	testBridgeConfig.Kafka.DefaultPartitions = 3
	testBridgeConfig.Kafka.ReplicationFactor = 1
	
	producer := kafka.NewProducer(&topicConfig.Kafka, &testBridgeConfig)
	if err := producer.Connect(); err != nil {
		log.Fatalf("Failed to connect to Kafka: %v", err)
	}
	defer producer.Close()
	
	// Test message to a topic that doesn't exist
	testTopic := "gom2k.test.autocreate"
	testMessage := &types.KafkaMessage{
		Key:   "test-key",
		Value: []byte(`{"test": "auto-topic-creation", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`),
		Topic: testTopic,
	}
	
	log.Printf("Sending test message to new topic: %s", testTopic)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := producer.WriteMessage(ctx, testMessage); err != nil {
		log.Fatalf("Failed to send test message with auto-create: %v", err)
	}
	
	log.Println("✓ Successfully sent message with auto-topic creation!")
	log.Printf("Topic: %s should now exist with 3 partitions", testTopic)
}