package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"software.sslmate.com/src/go-pkcs12"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"gom2k/pkg/types"
)

// Consumer handles Kafka message consumption with SSL support
type Consumer struct {
	reader       *kafka.Reader
	config       *types.KafkaConfig
	bridgeConfig *types.BridgeConfig
	topics       []string
}

// NewConsumer creates a new Kafka consumer with SSL configuration
func NewConsumer(kafkaConfig *types.KafkaConfig, bridgeConfig *types.BridgeConfig) *Consumer {
	return &Consumer{
		config:       kafkaConfig,
		bridgeConfig: bridgeConfig,
		topics:       generateKafkaTopics(bridgeConfig),
	}
}

// generateKafkaTopics creates list of Kafka topics to consume from based on prefix
func generateKafkaTopics(bridgeConfig *types.BridgeConfig) []string {
	// Return placeholder topics - actual discovery happens in Connect()
	// This is needed for initialization but will be replaced by dynamic discovery
	prefix := bridgeConfig.Mapping.KafkaPrefix
	return []string{fmt.Sprintf("%s.placeholder", prefix)}
}

// Connect establishes connection to Kafka with SSL
func (c *Consumer) Connect() error {
	log.Printf("Connecting to Kafka consumer with brokers: %v", c.config.Brokers)
	
	// Load PKCS#12 certificate
	tlsConfig, err := c.loadTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to load TLS config: %w", err)
	}

	// Configure Kafka reader
	// Note: kafka-go Reader can only consume from one topic at a time
	// For multiple topics, we'll need to create multiple readers or use a different approach
	// For now, let's start with consuming from a specific topic for testing
	
	// Discover existing Kafka topics dynamically
	discoveredTopics, err := c.discoverKafkaTopics()
	if err != nil {
		return fmt.Errorf("failed to discover Kafka topics: %w", err)
	}
	
	if len(discoveredTopics) == 0 {
		prefix := c.getBridgePrefix()
		log.Printf("Warning: No existing Kafka topics found with prefix '%s'", prefix)
		log.Printf("This is normal for a new deployment. Topics will be created when MQTT messages arrive.")
		// Create a default topic to start consuming from
		defaultTopic := fmt.Sprintf("%s.sensor", prefix)
		discoveredTopics = []string{defaultTopic}
		log.Printf("Using default topic: %s", defaultTopic)
	}
	
	// Use the first discovered topic for single-topic reader
	// TODO: Implement proper multi-topic consumption
	topicToConsume := discoveredTopics[0]
	c.topics = discoveredTopics // Update our topic list
	
	log.Printf("Discovered %d Kafka topics with prefix, consuming from: %s", len(discoveredTopics), topicToConsume)
	
	c.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers: c.config.Brokers,
		GroupID: c.config.Consumer.GroupID,
		Topic:   topicToConsume,
		
		// SSL configuration
		Dialer: &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
			TLS:       tlsConfig,
		},
		
		// Consumer configuration
		MinBytes:    1,    // Wait for at least 1 byte
		MaxBytes:    10e6, // Max 10MB per batch
		MaxWait:     1 * time.Second,
		StartOffset: kafka.LastOffset, // Start from latest messages
	})

	log.Println("✓ Kafka consumer connected successfully")
	return nil
}

// loadTLSConfig loads TLS configuration from PKCS#12 files
func (c *Consumer) loadTLSConfig() (*tls.Config, error) {
	if c.config.Security.Protocol != "SSL" {
		return nil, nil
	}

	ssl := c.config.Security.SSL
	
	// Load keystore (client certificate)
	keystoreData, err := os.ReadFile(ssl.Keystore.Location)
	if err != nil {
		return nil, fmt.Errorf("failed to read keystore: %w", err)
	}

	privateKey, cert, err := pkcs12.Decode(keystoreData, ssl.Keystore.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to decode keystore: %w", err)
	}

	// Load truststore (CA certificates)
	truststoreData, err := os.ReadFile(ssl.Truststore.Location)
	if err != nil {
		return nil, fmt.Errorf("failed to read truststore: %w", err)
	}

	_, caCert, err := pkcs12.Decode(truststoreData, ssl.Truststore.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to decode truststore: %w", err)
	}

	// Create TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{cert.Raw},
				PrivateKey:  privateKey,
			},
		},
		RootCAs: createCertPoolFromCerts([]*x509.Certificate{caCert}),
	}

	return tlsConfig, nil
}

// ReadMessage reads the next message from Kafka
func (c *Consumer) ReadMessage(ctx context.Context) (*types.KafkaMessage, error) {
	kafkaMsg, err := c.reader.ReadMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	// Convert to our internal message format
	msg := &types.KafkaMessage{
		Topic: kafkaMsg.Topic,
		Key:   string(kafkaMsg.Key),
		Value: kafkaMsg.Value,
	}

	return msg, nil
}

// Close gracefully shuts down the consumer
func (c *Consumer) Close() error {
	if c.reader != nil {
		log.Println("Closing Kafka consumer")
		return c.reader.Close()
	}
	return nil
}

// GetTopics returns the topics this consumer is subscribed to
func (c *Consumer) GetTopics() []string {
	return c.topics
}

// ConvertKafkaMessage converts a Kafka message back to MQTT format
func ConvertKafkaMessage(kafkaMsg *types.KafkaMessage) (*types.MQTTMessage, error) {
	// Parse JSON payload to extract original MQTT message
	var payload map[string]interface{}
	if err := json.Unmarshal(kafkaMsg.Value, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Kafka message: %w", err)
	}

	// Extract original MQTT topic from Kafka message key
	// The key should be the original MQTT topic
	mqttTopic := kafkaMsg.Key
	if mqttTopic == "" {
		// Fallback to extracting from JSON payload
		if topic, ok := payload["mqtt_topic"].(string); ok {
			mqttTopic = topic
		}
	}

	// Extract payload
	payloadStr, ok := payload["payload"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid payload format in Kafka message")
	}

	// Extract QoS (handle both int and float64 from JSON)
	var qos byte = 0
	if qosVal, ok := payload["qos"]; ok {
		switch qosValue := qosVal.(type) {
		case float64:
			qos = byte(qosValue)
		case int:
			qos = byte(qosValue)
		}
	}

	// Extract retained flag
	retained := false
	if retainedVal, ok := payload["retained"].(bool); ok {
		retained = retainedVal
	}

	// Extract timestamp
	timestamp := time.Now()
	if timestampVal, ok := payload["timestamp"].(string); ok {
		if parsedTime, err := time.Parse(time.RFC3339, timestampVal); err == nil {
			timestamp = parsedTime
		}
	}

	// Create MQTT message
	mqttMsg := &types.MQTTMessage{
		Topic:     mqttTopic,
		Payload:   []byte(payloadStr),
		QoS:       qos,
		Retained:  retained,
		Timestamp: timestamp,
	}

	return mqttMsg, nil
}

// discoverKafkaTopics dynamically discovers existing Kafka topics matching our prefix
func (c *Consumer) discoverKafkaTopics() ([]string, error) {
	// Create connection for topic discovery
	conn, err := c.createKafkaConn()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka connection for discovery: %w", err)
	}
	defer conn.Close()
	
	// Get all topics from Kafka cluster
	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, fmt.Errorf("failed to read Kafka partitions: %w", err)
	}
	
	// Extract unique topic names and filter by our prefix
	topicSet := make(map[string]bool)
	prefix := c.getBridgePrefix()
	
	for _, partition := range partitions {
		topicName := partition.Topic
		// Only include topics that start with our bridge prefix
		if strings.HasPrefix(topicName, prefix) {
			topicSet[topicName] = true
		}
	}
	
	// Convert set to slice
	var discoveredTopics []string
	for topic := range topicSet {
		discoveredTopics = append(discoveredTopics, topic)
	}
	
	log.Printf("Topic discovery: found %d topics with prefix '%s'", len(discoveredTopics), prefix)
	for _, topic := range discoveredTopics {
		log.Printf("  - %s", topic)
	}
	
	return discoveredTopics, nil
}

// createKafkaConn creates a connection to Kafka for admin operations
func (c *Consumer) createKafkaConn() (*kafka.Conn, error) {
	var dialer *kafka.Dialer
	
	// Configure SSL/TLS if specified
	if strings.ToUpper(c.config.Security.Protocol) == "SSL" {
		tlsConfig, err := c.loadTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		
		dialer = &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
			TLS:       tlsConfig,
		}
	} else {
		dialer = &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
		}
	}
	
	// Connect to the first broker
	if len(c.config.Brokers) == 0 {
		return nil, fmt.Errorf("no Kafka brokers configured")
	}
	
	broker := c.config.Brokers[0]
	host, port, err := net.SplitHostPort(broker)
	if err != nil {
		return nil, fmt.Errorf("invalid broker address %s: %w", broker, err)
	}
	
	conn, err := dialer.DialContext(context.Background(), "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kafka broker %s: %w", broker, err)
	}
	
	return conn, nil
}

// getBridgePrefix returns the Kafka topic prefix for this bridge instance
func (c *Consumer) getBridgePrefix() string {
	// Use the bridge config prefix directly
	if c.bridgeConfig != nil && c.bridgeConfig.Mapping.KafkaPrefix != "" {
		return c.bridgeConfig.Mapping.KafkaPrefix
	}
	
	// Fallback: derive from consumer group ID if bridge config unavailable
	groupID := c.config.Consumer.GroupID
	if groupID == "" {
		return "gom2k" // Default fallback
	}
	
	// Extract prefix from group ID (e.g., "gom2k-1" -> "gom2k")
	parts := strings.Split(groupID, "-")
	if len(parts) > 0 {
		return parts[0]
	}
	
	return "gom2k" // Default fallback
}

// createCertPoolFromCerts creates a certificate pool from x509 certificates
func createCertPoolFromCerts(certs []*x509.Certificate) *x509.CertPool {
	pool := x509.NewCertPool()
	for _, cert := range certs {
		pool.AddCert(cert)
	}
	return pool
}