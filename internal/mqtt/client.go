// Package mqtt provides MQTT client functionality with support for TLS connections,
// OS certificate stores, client ID templating with random suffixes, and configurable
// QoS levels. It handles connection management, message publishing/subscribing, and
// integrates with the bridge system for bidirectional message flow.
package mqtt

import (
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"time"

	"gom2k/pkg/types"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Client provides MQTT connectivity with support for TLS, authentication, and message handling.
// It wraps the Eclipse Paho MQTT client with additional features like client ID templating,
// OS certificate store integration, and structured message handling for bridge operations.
type Client struct {
	config         *types.MQTTConfig                // MQTT broker and connection configuration
	client         mqtt.Client                      // Underlying Paho MQTT client
	messageHandler func(*types.MQTTMessage)         // Callback function for received messages
}

// NewClient creates a new MQTT client with the provided configuration.
// The client supports TLS connections, OS certificate stores, and client ID templating
// with random suffixes to prevent ID conflicts in multi-instance deployments.
func NewClient(config *types.MQTTConfig) *Client {
	return &Client{
		config: config,
	}
}

// SetMessageHandler sets the callback for incoming messages
func (c *Client) SetMessageHandler(handler func(*types.MQTTMessage)) {
	c.messageHandler = handler
}

// Connect establishes connection to MQTT broker
func (c *Client) Connect() error {
	opts := mqtt.NewClientOptions()
	
	// Build broker URL
	scheme := "tcp"
	if c.config.Broker.UseTLS {
		scheme = "ssl"  // Try ssl scheme
	}
	brokerURL := fmt.Sprintf("%s://%s:%d", scheme, c.config.Broker.Host, c.config.Broker.Port)
	opts.AddBroker(brokerURL)
	
	// Client ID with random suffix support
	clientID := c.config.Client.ClientID
	if strings.Contains(clientID, "{random}") {
		clientID = strings.ReplaceAll(clientID, "{random}", fmt.Sprintf("%d", time.Now().UnixNano()%10000))
	}
	opts.SetClientID(clientID)
	
	// Authentication
	if c.config.Auth.Username != "" {
		opts.SetUsername(c.config.Auth.Username)
		opts.SetPassword(c.config.Auth.Password)
	}
	
	// TLS Configuration
	if c.config.Broker.UseTLS {
		tlsConfig := &tls.Config{
			ServerName: c.config.Broker.Host, // Ensure SNI is set correctly
		}
		
		if c.config.Broker.UseOSCerts {
			// Use system certificate store (equivalent to --tls-use-os-certs)
			tlsConfig.InsecureSkipVerify = false
		} else {
			// If not using OS certs, might need to skip verification for testing
			tlsConfig.InsecureSkipVerify = false
		}
		
		opts.SetTLSConfig(tlsConfig)
		log.Printf("TLS enabled with SNI: %s", c.config.Broker.Host)
	}
	
	// Connection settings
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)
	opts.SetConnectTimeout(30 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	
	// Connection handlers
	opts.SetConnectionLostHandler(c.onConnectionLost)
	opts.SetOnConnectHandler(c.onConnect)
	
	// Default message handler
	opts.SetDefaultPublishHandler(c.onMessage)
	
	c.client = mqtt.NewClient(opts)
	
	log.Printf("Connecting to MQTT broker: %s (TLS: %v)", brokerURL, c.config.Broker.UseTLS)
	token := c.client.Connect()
	token.Wait()
	
	if token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}
	
	log.Println("Successfully connected to MQTT broker")
	return nil
}

// Subscribe subscribes to specified topics
func (c *Client) Subscribe() error {
	for _, topic := range c.config.Topics.Subscribe {
		log.Printf("Subscribing to MQTT topic: %s", topic)
		
		token := c.client.Subscribe(topic, c.config.Client.QoS, nil)
		token.Wait()
		
		if token.Error() != nil {
			return fmt.Errorf("failed to subscribe to topic %s: %w", topic, token.Error())
		}
		
		log.Printf("Successfully subscribed to: %s", topic)
	}
	
	return nil
}

// Publish publishes a message to MQTT
func (c *Client) Publish(topic string, payload []byte, qos byte, retained bool) error {
	token := c.client.Publish(topic, qos, retained, payload)
	token.Wait()
	
	if token.Error() != nil {
		return fmt.Errorf("failed to publish to topic %s: %w", topic, token.Error())
	}
	
	return nil
}

// Disconnect closes the MQTT connection
func (c *Client) Disconnect() {
	if c.client != nil && c.client.IsConnected() {
		log.Println("Disconnecting from MQTT broker")
		c.client.Disconnect(250)
	}
}

// Connection event handlers
func (c *Client) onConnect(client mqtt.Client) {
	log.Println("MQTT client connected")
}

func (c *Client) onConnectionLost(client mqtt.Client, err error) {
	log.Printf("MQTT connection lost: %v", err)
}

// Message handler
func (c *Client) onMessage(client mqtt.Client, msg mqtt.Message) {
	// Skip retained messages if configured
	if c.config.Topics.RetainOnly && !msg.Retained() {
		return
	}
	
	mqttMsg := &types.MQTTMessage{
		Topic:     msg.Topic(),
		Payload:   msg.Payload(),
		QoS:       msg.Qos(),
		Retained:  msg.Retained(),
		Timestamp: time.Now(),
	}
	
	if c.messageHandler != nil {
		c.messageHandler(mqttMsg)
	}
}