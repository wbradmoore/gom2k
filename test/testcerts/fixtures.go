package testcerts

import (
	"testing"

	"gom2k/pkg/types"
)

// Fixtures provides pre-configured test scenarios for SSL certificate testing
type Fixtures struct {
	certs *TestCertificates
	t     *testing.T
}

// NewFixtures creates a new fixtures instance with generated test certificates
func NewFixtures(t *testing.T) (*Fixtures, error) {
	certs, err := CreateTestCertificates(t, DefaultOptions())
	if err != nil {
		return nil, err
	}

	return &Fixtures{
		certs: certs,
		t:     t,
	}, nil
}

// CompleteValidConfig returns a complete, valid configuration for testing
func (f *Fixtures) CompleteValidConfig() *types.Config {
	return &types.Config{
		MQTT:   *f.ValidMQTTTLSConfig(),
		Kafka:  *f.ValidKafkaSSLConfig(),
		Bridge: *f.ValidBridgeConfig(),
	}
}

// CompleteInvalidSSLConfig returns a configuration with invalid SSL certificate paths
func (f *Fixtures) CompleteInvalidSSLConfig() *types.Config {
	return &types.Config{
		MQTT:   *f.ValidMQTTTLSConfig(),
		Kafka:  *f.InvalidKafkaSSLConfig(),
		Bridge: *f.ValidBridgeConfig(),
	}
}

// ValidMQTTTLSConfig returns a valid MQTT configuration with TLS
func (f *Fixtures) ValidMQTTTLSConfig() *types.MQTTConfig {
	return &types.MQTTConfig{
		Broker: struct {
			Host       string `yaml:"host"`
			Port       int    `yaml:"port"`
			UseTLS     bool   `yaml:"use_tls"`
			UseOSCerts bool   `yaml:"use_os_certs"`
		}{
			Host:       "localhost",
			Port:       8883,
			UseTLS:     true,
			UseOSCerts: true,
		},
		Auth: struct {
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		}{
			Username: "testuser",
			Password: "testpass",
		},
		Client: struct {
			ClientID string `yaml:"client_id"`
			QoS      byte   `yaml:"qos"`
		}{
			ClientID: "test-client",
			QoS:      0,
		},
		Topics: struct {
			Subscribe  []string `yaml:"subscribe"`
			RetainOnly bool     `yaml:"retain_only"`
		}{
			Subscribe:  []string{"test/#"},
			RetainOnly: false,
		},
	}
}

// ValidKafkaSSLConfig returns a valid Kafka configuration with SSL
func (f *Fixtures) ValidKafkaSSLConfig() *types.KafkaConfig {
	return &types.KafkaConfig{
		Brokers: []string{"localhost:9093"},
		Security: struct {
			Protocol string `yaml:"protocol"`
			SSL      struct {
				Truststore struct {
					Location string `yaml:"location"`
					Password string `yaml:"password"`
				} `yaml:"truststore"`
				Keystore struct {
					Location    string `yaml:"location"`
					Password    string `yaml:"password"`
					KeyPassword string `yaml:"key_password"`
				} `yaml:"keystore"`
			} `yaml:"ssl"`
		}{
			Protocol: "SSL",
			SSL: struct {
				Truststore struct {
					Location string `yaml:"location"`
					Password string `yaml:"password"`
				} `yaml:"truststore"`
				Keystore struct {
					Location    string `yaml:"location"`
					Password    string `yaml:"password"`
					KeyPassword string `yaml:"key_password"`
				} `yaml:"keystore"`
			}{
				Truststore: struct {
					Location string `yaml:"location"`
					Password string `yaml:"password"`
				}{
					Location: f.certs.GetTruststorePath(),
					Password: f.certs.GetPassword(),
				},
				Keystore: struct {
					Location    string `yaml:"location"`
					Password    string `yaml:"password"`
					KeyPassword string `yaml:"key_password"`
				}{
					Location:    f.certs.GetKeystorePath(),
					Password:    f.certs.GetPassword(),
					KeyPassword: f.certs.GetPassword(),
				},
			},
		},
		Consumer: struct {
			GroupID string `yaml:"group_id"`
		}{
			GroupID: "test-group",
		},
	}
}

// InvalidKafkaSSLConfig returns a Kafka configuration with invalid SSL certificate paths
func (f *Fixtures) InvalidKafkaSSLConfig() *types.KafkaConfig {
	config := f.ValidKafkaSSLConfig()
	
	// Set invalid certificate paths
	config.Security.SSL.Keystore.Location = "/nonexistent/keystore.jks"
	config.Security.SSL.Truststore.Location = "/nonexistent/truststore.jks"
	
	return config
}

// DisallowedDirectoryConfig returns a Kafka configuration with certificates in disallowed directories
func (f *Fixtures) DisallowedDirectoryConfig() *types.KafkaConfig {
	config := f.ValidKafkaSSLConfig()
	
	// Set certificate paths to disallowed directories (system paths)
	config.Security.SSL.Keystore.Location = "/etc/passwd"
	config.Security.SSL.Truststore.Location = "/etc/hosts"
	
	return config
}

// ValidBridgeConfig returns a valid bridge configuration
func (f *Fixtures) ValidBridgeConfig() *types.BridgeConfig {
	return &types.BridgeConfig{
		Mapping: struct {
			KafkaPrefix    string `yaml:"kafka_prefix"`
			MaxTopicLevels int    `yaml:"max_topic_levels"`
		}{
			KafkaPrefix:    "test",
			MaxTopicLevels: 3,
		},
		Features: struct {
			MQTTToKafka bool `yaml:"mqtt_to_kafka"`
			KafkaToMQTT bool `yaml:"kafka_to_mqtt"`
		}{
			MQTTToKafka: true,
			KafkaToMQTT: true,
		},
		Kafka: struct {
			AutoCreateTopics  bool `yaml:"auto_create_topics"`
			DefaultPartitions int  `yaml:"default_partitions"`
			ReplicationFactor int  `yaml:"replication_factor"`
		}{
			AutoCreateTopics:  true,
			DefaultPartitions: 1,
			ReplicationFactor: 1,
		},
	}
}

// GetCertificates returns the underlying TestCertificates instance
func (f *Fixtures) GetCertificates() *TestCertificates {
	return f.certs
}