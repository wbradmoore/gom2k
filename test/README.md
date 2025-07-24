# GOM2K Testing Guide

Complete testing suite for the GOM2K MQTT-Kafka Bridge.

## Quick Start - One Command Testing

### **Option 1: Complete Integration Tests (Recommended)**
```bash
cd test && ./test.sh
```
This runs everything: unit tests, Docker setup, connectivity tests, bridge tests, and cleanup.

### **Option 2: Unit Tests Only**
```bash
cd test && ./test.sh unit
```
Just the unit tests, no Docker required.

### **Option 3: SSL/TLS Tests**
```bash
cd test && ./test.sh ssl
```
Complete SSL test suite with automatic certificate generation, encrypted connections, and cleanup.

### **Option 4: Cleanup**
```bash
cd test && ./test.sh cleanup
```
Stop all Docker services and cleanup (if tests were interrupted).

---

## What Each Test Does

### **Unit Tests** (`./test/unit/`)
- Configuration loading and validation
- Topic mapping logic (MQTT ↔ Kafka)
- Message conversion and round-trip integrity
- Edge cases and error handling

### **Integration Tests** (Docker-based)
- MQTT connectivity to test broker (plaintext)
- Kafka connectivity with auto-topic creation (plaintext)
- Bidirectional message flow
- Real-time bridge operation

### **SSL/TLS Tests** (Docker-based with automatic certificates)
- MQTT TLS connectivity with auto-generated test certificates
- Kafka SSL connectivity with JKS keystores
- Bidirectional encrypted message flow
- Automatic certificate generation and caching (30-day reuse)

---

## Manual Testing (For Development)

If you need to test manually, start the Docker services and bridge separately:

```bash
# Start plaintext services (from test directory)
docker compose --profile plaintext up -d

# Or start SSL services  
docker compose --profile ssl up -d

# Start bridge (from test directory)
CONFIGS_DIR="." ../gom2k &

# Send test message
mosquitto_pub -h localhost -p 1883 -t "test/sensor/temp" -m '{"value": 25.5}'

# Check Kafka
docker exec gom2k-test-kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic gom2k.test.sensor --from-beginning

# Cleanup when done (from test directory)
./test.sh cleanup
```

---

## Test Configuration

### **Test Configs**:
- `config.yaml` - Plaintext test configuration (MQTT port 1883, Kafka port 9092)
- `config-ssl.yaml` - SSL test configuration (MQTT port 8883, Kafka port 9093)
- `ssl/certs/` - Auto-generated test certificates with 30-day caching (never use in production)
- `ssl/README-CERTS.md` - Certificate generation details and troubleshooting

---

## Troubleshooting

**Tests fail with "command not found":**
```bash
# Install required tools
sudo apt install mosquitto-clients  # For mosquitto_pub/sub
# Ensure Docker and Go are installed
```

**Docker services won't start:**
```bash
# Check ports aren't in use
sudo netstat -tlnp | grep -E "(1883|9092|8080)"

# Force cleanup and restart
docker system prune -f
./test.sh cleanup
./test.sh
```

**Bridge won't connect:**
```bash
# Check Docker services are running
docker ps

# Check logs
docker logs gom2k-test-kafka
docker logs gom2k-test-mosquitto
```

---

## For CI/CD

```bash
# Single command for automated testing (run from test directory)
cd test && ./test.sh

# Exit codes:
# 0 = All tests passed
# 1 = Tests failed
```

---

## Alternative Testing Methods

If you prefer the traditional approach, you can still use:

### **Makefile Commands**
```bash
make test-all        # Complete test suite with Docker
make test-unit       # Unit tests only
make docker-up       # Start Docker services
make docker-down     # Stop Docker services
```

### **Manual Docker Setup**
```bash
cd test/
docker compose --profile plaintext up -d
# ... run tests manually ...
docker compose --profile plaintext down
```

### **Go Test Commands**
```bash
go test ./test/unit/... -v                    # Unit tests
go test ./test/integration/... -v             # Integration tests
CONFIGS_DIR="./test" go test ./...
```

This streamlined approach gets you from **zero to fully tested** in one command!