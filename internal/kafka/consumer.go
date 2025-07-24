package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"software.sslmate.com/src/go-pkcs12"
	"time"

	"github.com/segmentio/kafka-go"
	"gom2k/pkg/types"
)

// Consumer handles Kafka message consumption with SSL support
type Consumer struct {
	reader *kafka.Reader
	config *types.KafkaConfig
	topics []string
}

// NewConsumer creates a new Kafka consumer with SSL configuration
func NewConsumer(kafkaConfig *types.KafkaConfig, bridgeConfig *types.BridgeConfig) *Consumer {
	return &Consumer{
		config: kafkaConfig,
		topics: generateKafkaTopics(bridgeConfig),
	}
}

// generateKafkaTopics creates list of Kafka topics to consume from based on prefix
func generateKafkaTopics(bridgeConfig *types.BridgeConfig) []string {
	// For now, we'll consume from topics matching our prefix pattern
	// In a production system, you might want to discover topics dynamically
	prefix := bridgeConfig.Mapping.KafkaPrefix
	
	// Common MQTT topic patterns we expect to see in Kafka
	patterns := []string{
		fmt.Sprintf("%s.homeassistant", prefix),
		fmt.Sprintf("%s.zigbee2mqtt", prefix),
		fmt.Sprintf("%s.sensor", prefix),
		fmt.Sprintf("%s.device", prefix),
		fmt.Sprintf("%s.switch", prefix),
		fmt.Sprintf("%s.light", prefix),
		fmt.Sprintf("%s.temp", prefix),
		fmt.Sprintf("%s.humidity", prefix),
	}
	
	return patterns
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
	
	c.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers: c.config.Brokers,
		GroupID: c.config.Consumer.GroupID,
		Topic:   "gom2k.homeassistant.sensor.temp", // Start with a specific topic for testing
		
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

	privateKey, cert, err := pkcs12.Decode(keystoreData, ssl.Keystore.KeyPassword)
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
		switch v := qosVal.(type) {
		case float64:
			qos = byte(v)
		case int:
			qos = byte(v)
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

// createCertPoolFromCerts creates a certificate pool from x509 certificates
func createCertPoolFromCerts(certs []*x509.Certificate) *x509.CertPool {
	pool := x509.NewCertPool()
	for _, cert := range certs {
		pool.AddCert(cert)
	}
	return pool
}