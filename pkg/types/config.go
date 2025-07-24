package types

import "time"

// Config represents the complete bridge configuration
type Config struct {
	MQTT   MQTTConfig   `yaml:"mqtt"`
	Kafka  KafkaConfig  `yaml:"kafka"`
	Bridge BridgeConfig `yaml:"bridge"`
}

// MQTTConfig holds MQTT connection settings
type MQTTConfig struct {
	Broker struct {
		Host       string `yaml:"host"`
		Port       int    `yaml:"port"`
		UseTLS     bool   `yaml:"use_tls"`
		UseOSCerts bool   `yaml:"use_os_certs"`
	} `yaml:"broker"`
	Auth struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"auth"`
	Client struct {
		ClientID string `yaml:"client_id"`
		QoS      byte   `yaml:"qos"`
	} `yaml:"client"`
	Topics struct {
		Subscribe  []string `yaml:"subscribe"`
		RetainOnly bool     `yaml:"retain_only"`
	} `yaml:"topics"`
}

// KafkaConfig holds Kafka connection settings
type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`
	Security struct {
		Protocol string `yaml:"protocol"`
		SSL struct {
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
	} `yaml:"security"`
	Consumer struct {
		GroupID string `yaml:"group_id"`
	} `yaml:"consumer"`
	Partitioning string `yaml:"partitioning"`
}

// BridgeConfig holds bridge behavior settings
type BridgeConfig struct {
	Mapping struct {
		KafkaPrefix     string `yaml:"kafka_prefix"`
		MaxTopicLevels  int    `yaml:"max_topic_levels"`
	} `yaml:"mapping"`
	Retry struct {
		ConnectionTimeout time.Duration `yaml:"connection_timeout"`
	} `yaml:"retry"`
	Logging struct {
		Level string `yaml:"level"`
	} `yaml:"logging"`
	Features struct {
		MQTTToKafka bool `yaml:"mqtt_to_kafka"`
		KafkaToMQTT bool `yaml:"kafka_to_mqtt"`
	} `yaml:"features"`
	Kafka struct {
		AutoCreateTopics  bool `yaml:"auto_create_topics"`
		DefaultPartitions int  `yaml:"default_partitions"`
		ReplicationFactor int  `yaml:"replication_factor"`
	} `yaml:"kafka"`
}