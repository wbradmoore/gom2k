# GOM2K MQTT-Kafka Bridge

A high-performance bidirectional bridge between MQTT and Apache Kafka, designed for IoT and real-time data streaming applications.

## Features

- **Bidirectional messaging** - (but intended for MQTT source and sink with Kafka in the middle)
- **SSL/TLS support** - Both MQTT and Kafka
- **Topic mapping** - Configurable MQTT to Kafka topic transformation
- **Auto-topic creation** - Automatically creates Kafka topics as needed
- **Message integrity** - Preserves QoS, retain flags, and timestamps
- **Consumer groups** - Scalable Kafka consumption with configurable groups

## Quick Start

### 0. Test
```bash
cd test
./tesh.sh
```

### 1. Configure
```bash
cp configs/sample-config.yaml configs/config.yaml
# Edit configs/config.yaml with your MQTT/Kafka settings
```

### 2. Run
```bash
./gom2k
```

## Configuration

Example minimal configuration:

```yaml
mqtt:
  broker:
    host: "mqtt.example.com"
    port: 1883
  topics:
    subscribe: ["sensor/#"]

kafka:
  brokers: ["kafka.example.com:9092"]

bridge:
  mapping:
    kafka_prefix: "iot"
    max_topic_levels: 3
  features:
    mqtt_to_kafka: true
    kafka_to_mqtt: true
```

See `configs/sample-config.yaml` for more.

## Topic Mapping

MQTT topics are mapped to Kafka topics with configurable prefixes and level limits:

```
MQTT Topic                    → Kafka Topic
sensor/temperature           → iot.sensor.temperature
home/kitchen/sensor/temp     → iot.home.kitchen (truncated at 3 levels)
homeassistant/switch/state   → iot.homeassistant.switch
```

## Message Format

Messages include original MQTT metadata:

```json
{
  "payload": "23.5",
  "timestamp": "2024-01-01T12:00:00Z",
  "qos": 0,
  "retained": false,
  "mqtt_topic": "sensor/temperature"
}
```

## Testing

The project includes comprehensive test suites:

```bash
cd test && ./test.sh                    # Full integration tests
cd test && ./test.sh unit               # Unit tests only  
cd test && ./test.sh ssl                # SSL/TLS tests
cd test && ./test.sh cleanup            # Clean up test environment
```

## Building

```bash
go build -o gom2k ./cmd/gom2k
```

## Requirements

- Go 1.19+
- MQTT broker (Mosquitto, etc.)
- Apache Kafka 2.8+

## License

MIT License - see LICENSE file for details.