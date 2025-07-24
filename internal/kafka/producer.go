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

type Producer struct {
	config        *types.KafkaConfig
	bridgeConfig  *types.BridgeConfig
	writer        *kafka.Writer
	createdTopics map[string]bool
	topicMutex    sync.RWMutex
}

// NewProducer creates a new Kafka producer
func NewProducer(config *types.KafkaConfig, bridgeConfig *types.BridgeConfig) *Producer {
	return &Producer{
		config:        config,
		bridgeConfig:  bridgeConfig,
		createdTopics: make(map[string]bool),
	}
}

// Connect initializes the Kafka producer
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
		// Check if it's a "topic doesn't exist" error and auto-create if enabled
		if strings.Contains(err.Error(), "Unknown Topic Or Partition") && p.bridgeConfig.Kafka.AutoCreateTopics {
			if createErr := p.createTopicIfNeeded(ctx, msg.Topic); createErr != nil {
				return fmt.Errorf("failed to create topic %s: %w", msg.Topic, createErr)
			}
			
			// Retry the message after topic creation with backoff
			for i := 0; i < 3; i++ {
				time.Sleep(time.Duration(i*200) * time.Millisecond)
				err = p.writer.WriteMessages(ctx, kafkaMsg)
				if err == nil {
					return nil
				}
				if !strings.Contains(err.Error(), "Unknown Topic Or Partition") {
					break // Different error, don't retry
				}
				log.Printf("Retry %d: Topic still not available, waiting...", i+1)
			}
			return fmt.Errorf("failed to write message to Kafka after topic creation and retries: %w", err)
		}
		
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

// createTopicIfNeeded creates a Kafka topic if it doesn't exist and hasn't been created by this producer
func (p *Producer) createTopicIfNeeded(ctx context.Context, topicName string) error {
	// Check if we've already created this topic
	p.topicMutex.RLock()
	if p.createdTopics[topicName] {
		p.topicMutex.RUnlock()
		return nil
	}
	p.topicMutex.RUnlock()
	
	// Create connection for topic management
	conn, err := p.createKafkaConn()
	if err != nil {
		return fmt.Errorf("failed to create Kafka connection: %w", err)
	}
	defer conn.Close()
	
	log.Printf("Creating Kafka topic: %s (partitions: %d, replication: %d)", 
		topicName, p.bridgeConfig.Kafka.DefaultPartitions, p.bridgeConfig.Kafka.ReplicationFactor)
	
	// Create the topic
	topicConfig := kafka.TopicConfig{
		Topic:             topicName,
		NumPartitions:     p.bridgeConfig.Kafka.DefaultPartitions,
		ReplicationFactor: p.bridgeConfig.Kafka.ReplicationFactor,
	}
	
	err = conn.CreateTopics(topicConfig)
	if err != nil {
		// Check if topic already exists (could have been created by another process)
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "Topic already exists") {
			log.Printf("Topic %s already exists", topicName)
		} else {
			return fmt.Errorf("failed to create topic: %w", err)
		}
	} else {
		log.Printf("✓ Successfully created Kafka topic: %s", topicName)
	}
	
	// Mark as created to avoid future attempts
	p.topicMutex.Lock()
	p.createdTopics[topicName] = true
	p.topicMutex.Unlock()
	
	// Allow time for topic to propagate across Kafka cluster
	time.Sleep(500 * time.Millisecond)
	
	return nil
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