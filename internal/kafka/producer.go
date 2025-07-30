// Package kafka provides Kafka client implementations for both producing and consuming messages.
// It includes SSL/TLS support with PKCS12 keystores, automatic topic creation, and message
// transformation between MQTT and Kafka formats. The package handles connection management,
// error recovery, and topic lifecycle operations.
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
	"strings"
	"sync"
	"time"

	"gom2k/pkg/types"

	"software.sslmate.com/src/go-pkcs12"
	"github.com/segmentio/kafka-go"
)

// Producer handles sending messages to Kafka topics with SSL support and automatic topic creation.
// It maintains a connection pool, tracks created topics to avoid duplicate creation attempts,
// and provides thread-safe operations for concurrent message publishing.
type Producer struct {
	config        *types.KafkaConfig     // Kafka connection and security configuration
	bridgeConfig  *types.BridgeConfig    // Bridge-specific settings like topic creation parameters
	writer        *kafka.Writer          // Underlying Kafka writer for message production
	createdTopics map[string]bool        // Cache of topics already created by this producer
	topicMutex    sync.RWMutex          // Protects the createdTopics map from concurrent access
}

// NewProducer creates a new Kafka producer with the provided configuration.
// The producer supports SSL/TLS connections using PKCS12 keystores and can automatically
// create topics based on the bridge configuration settings.
func NewProducer(config *types.KafkaConfig, bridgeConfig *types.BridgeConfig) *Producer {
	return &Producer{
		config:        config,
		bridgeConfig:  bridgeConfig,
		createdTopics: make(map[string]bool),
	}
}

// Connect establishes a connection to the Kafka cluster and initializes the producer.
// It configures SSL/TLS settings if specified in the configuration and sets up
// hash-based partitioning for consistent message distribution based on message keys.
func (p *Producer) Connect() error {
	// Create writer configuration
	writerConfig := kafka.WriterConfig{
		Brokers: p.config.Brokers,
		Balancer: &kafka.Hash{}, // Use hash balancer for key-based partitioning
	}
	
	// Configure SSL/TLS if specified
	if strings.ToUpper(p.config.Security.Protocol) == "SSL" {
		tlsConfig, err := p.createTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to create TLS config: %w", err)
		}
		
		// Create dialer with TLS
		dialer := &kafka.Dialer{
			TLS: tlsConfig,
		}
		writerConfig.Dialer = dialer
	}
	
	p.writer = kafka.NewWriter(writerConfig)
	
	log.Printf("Kafka producer initialized with brokers: %v", p.config.Brokers)
	return nil
}

// WriteMessage sends a message to Kafka
func (p *Producer) WriteMessage(ctx context.Context, msg *types.KafkaMessage) error {
	kafkaMsg := kafka.Message{
		Topic: msg.Topic,
		Key:   []byte(msg.Key),
		Value: msg.Value,
	}
	
	err := p.writer.WriteMessages(ctx, kafkaMsg)
	if err != nil {
		// If auto-creation is enabled, try to create the topic and retry once
		if p.bridgeConfig.Kafka.AutoCreateTopics {
			if createErr := p.createTopicIfNeeded(ctx, msg.Topic); createErr != nil {
				return fmt.Errorf("failed to create topic %s: %w", msg.Topic, createErr)
			}
			
			// Single retry after topic creation with brief delay
			time.Sleep(500 * time.Millisecond)
			if retryErr := p.writer.WriteMessages(ctx, kafkaMsg); retryErr == nil {
				return nil
			}
		}
		// Return original error - Kafka client gives clear error messages
		return fmt.Errorf("failed to write message to Kafka: %w", err)
	}
	
	return nil
}

// WriteMessages sends multiple messages to Kafka
func (p *Producer) WriteMessages(ctx context.Context, messages []*types.KafkaMessage) error {
	kafkaMessages := make([]kafka.Message, len(messages))
	
	for i, msg := range messages {
		kafkaMessages[i] = kafka.Message{
			Topic: msg.Topic,
			Key:   []byte(msg.Key),
			Value: msg.Value,
		}
	}
	
	err := p.writer.WriteMessages(ctx, kafkaMessages...)
	if err != nil {
		return fmt.Errorf("failed to write messages to Kafka: %w", err)
	}
	
	return nil
}

// Close closes the producer
func (p *Producer) Close() error {
	if p.writer != nil {
		log.Println("Closing Kafka producer")
		return p.writer.Close()
	}
	return nil
}

// Helper function to create TLS configuration for SSL
func (p *Producer) createTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{}
	
	// Load truststore (CA certificates)
	if p.config.Security.SSL.Truststore.Location != "" {
		if _, err := os.Stat(p.config.Security.SSL.Truststore.Location); os.IsNotExist(err) {
			return nil, fmt.Errorf("truststore file not found: %s", p.config.Security.SSL.Truststore.Location)
		}
		
		log.Printf("Loading truststore: %s", p.config.Security.SSL.Truststore.Location)
		
		caCerts, err := p.loadTruststorePKCS12(p.config.Security.SSL.Truststore.Location, p.config.Security.SSL.Truststore.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to load truststore: %w", err)
		}
		
		tlsConfig.RootCAs = caCerts
		log.Printf("Loaded CA certificates from truststore")
	}
	
	// Load client keystore (client certificate for mutual TLS)
	if p.config.Security.SSL.Keystore.Location != "" {
		if _, err := os.Stat(p.config.Security.SSL.Keystore.Location); os.IsNotExist(err) {
			return nil, fmt.Errorf("keystore file not found: %s", p.config.Security.SSL.Keystore.Location)
		}
		
		log.Printf("Loading keystore: %s", p.config.Security.SSL.Keystore.Location)
		
		clientCert, err := p.loadKeystorePKCS12(
			p.config.Security.SSL.Keystore.Location,
			p.config.Security.SSL.Keystore.Password,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load keystore: %w", err)
		}
		
		tlsConfig.Certificates = []tls.Certificate{clientCert}
		log.Println("Loaded client certificate from keystore")
	}
	
	return tlsConfig, nil
}

// loadTruststorePKCS12 loads CA certificates from a PKCS#12 truststore
func (p *Producer) loadTruststorePKCS12(filename, password string) (*x509.CertPool, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	
	log.Printf("Attempting to load PKCS#12 with password length: %d", len(password))
	
	// Parse PKCS#12 truststore data
	certs, err := pkcs12.DecodeTrustStore(data, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PKCS#12 truststore (check password): %w", err)
	}
	
	certPool := x509.NewCertPool()
	
	// Add all certificates from truststore
	for _, cert := range certs {
		certPool.AddCert(cert)
		log.Printf("Added CA certificate: %s", cert.Subject.CommonName)
	}
	
	if len(certs) == 0 {
		log.Println("Warning: no certificates found in truststore")
	}
	
	return certPool, nil
}

// loadKeystorePKCS12 loads client certificate and private key from a PKCS#12 keystore
func (p *Producer) loadKeystorePKCS12(filename, password string) (tls.Certificate, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return tls.Certificate{}, err
	}
	
	log.Printf("Attempting to load PKCS#12 keystore with password length: %d", len(password))
	
	// Parse PKCS#12 data
	privateKey, cert, err := pkcs12.Decode(data, password)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to decode PKCS#12 keystore (check password): %w", err)
	}
	
	if privateKey == nil || cert == nil {
		return tls.Certificate{}, fmt.Errorf("no private key or certificate found in keystore")
	}
	
	// Create certificate chain
	var certChain [][]byte
	certChain = append(certChain, cert.Raw)
	
	// Create TLS certificate
	tlsCert := tls.Certificate{
		Certificate: certChain,
		PrivateKey:  privateKey,
	}
	
	log.Printf("Loaded client certificate: %s", cert.Subject.CommonName)
	return tlsCert, nil
}

// createTopicIfNeeded creates a Kafka topic if it doesn't exist and hasn't been created by this producer.
// This function is thread-safe and caches created topics to avoid duplicate operations.
func (p *Producer) createTopicIfNeeded(ctx context.Context, topicName string) error {
	p.topicMutex.Lock()
	defer p.topicMutex.Unlock()
	
	if p.isTopicAlreadyCreated(topicName) {
		return nil
	}
	
	conn, err := p.createKafkaConn()
	if err != nil {
		return fmt.Errorf("failed to create Kafka connection: %w", err)
	}
	defer conn.Close()
	
	if err := p.createTopicWithConfig(conn, topicName); err != nil {
		return err
	}
	
	p.markTopicAsCreated(topicName)
	p.waitForTopicPropagation()
	
	return nil
}

// isTopicAlreadyCreated checks if we've already attempted to create this topic locally.
func (p *Producer) isTopicAlreadyCreated(topicName string) bool {
	return p.createdTopics[topicName]
}

// createTopicWithConfig handles the actual topic creation with proper configuration.
func (p *Producer) createTopicWithConfig(conn *kafka.Conn, topicName string) error {
	config := p.buildTopicConfig(topicName)
	
	log.Printf("Creating Kafka topic: %s (partitions: %d, replication: %d)", 
		topicName, config.NumPartitions, config.ReplicationFactor)
	
	err := conn.CreateTopics(config)
	return p.handleTopicCreationResult(err, topicName)
}

// buildTopicConfig constructs the topic configuration based on bridge settings.
func (p *Producer) buildTopicConfig(topicName string) kafka.TopicConfig {
	return kafka.TopicConfig{
		Topic:             topicName,
		NumPartitions:     p.bridgeConfig.Kafka.DefaultPartitions,
		ReplicationFactor: p.bridgeConfig.Kafka.ReplicationFactor,
	}
}

// handleTopicCreationResult processes the result of topic creation.
func (p *Producer) handleTopicCreationResult(err error, topicName string) error {
	if err != nil {
		// Just log the error and continue - topic might already exist or have other issues
		// The original write error will be returned to user if the topic truly doesn't work
		log.Printf("Topic creation attempted for %s: %v", topicName, err)
		return nil
	}
	
	log.Printf("✓ Successfully created Kafka topic: %s", topicName)
	return nil
}

// markTopicAsCreated updates the local cache to indicate this topic has been processed.
func (p *Producer) markTopicAsCreated(topicName string) {
	p.createdTopics[topicName] = true
}

// waitForTopicPropagation allows time for the topic to propagate across the Kafka cluster.
func (p *Producer) waitForTopicPropagation() {
	time.Sleep(500 * time.Millisecond)
}

// createKafkaConn creates a connection to Kafka for admin operations
func (p *Producer) createKafkaConn() (*kafka.Conn, error) {
	var dialer *kafka.Dialer
	
	// Configure SSL/TLS if specified
	if strings.ToUpper(p.config.Security.Protocol) == "SSL" {
		tlsConfig, err := p.createTLSConfig()
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
	if len(p.config.Brokers) == 0 {
		return nil, fmt.Errorf("no Kafka brokers configured")
	}
	
	broker := p.config.Brokers[0]
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

// ConvertMQTTMessage converts an MQTT message to Kafka format
func ConvertMQTTMessage(mqttMsg *types.MQTTMessage, kafkaTopic string) (*types.KafkaMessage, error) {
	// Create JSON payload with metadata
	payload := map[string]interface{}{
		"payload":    string(mqttMsg.Payload),
		"timestamp":  mqttMsg.Timestamp,
		"qos":        mqttMsg.QoS,
		"retained":   mqttMsg.Retained,
		"mqtt_topic": mqttMsg.Topic,
	}
	
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MQTT message to JSON: %w", err)
	}
	
	return &types.KafkaMessage{
		Key:   mqttMsg.Topic, // Use MQTT topic as Kafka key for partitioning
		Value: jsonPayload,
		Topic: kafkaTopic,
	}, nil
}