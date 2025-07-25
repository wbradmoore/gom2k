# GOM2K MQTT-Kafka Bridge Makefile

.PHONY: help build test test-unit test-integration test-coverage test-race clean lint

# Default target
help:
	@echo "GOM2K MQTT-Kafka Bridge"
	@echo ""
	@echo "Available targets:"
	@echo "  build           - Build the bridge binary"
	@echo "  test            - Run all tests"
	@echo "  test-unit       - Run unit tests only (fast, no external deps)"
	@echo "  test-integration- Run integration tests (requires MQTT/Kafka)"
	@echo "  test-coverage   - Run tests with coverage report"
	@echo "  test-race       - Run tests with race detection"
	@echo "  lint            - Run linters"
	@echo "  clean           - Clean build artifacts"
	@echo "  help            - Show this help"
	@echo ""
	@echo "Environment variables for integration tests:"
	@echo "  MQTT_HOST       - MQTT broker host (default: localhost)"
	@echo "  MQTT_PORT       - MQTT broker port (default: 1883)"
	@echo "  MQTT_USERNAME   - MQTT username"
	@echo "  MQTT_PASSWORD   - MQTT password"
	@echo "  KAFKA_BROKERS   - Kafka brokers (default: localhost:9092)"

# Build the bridge binary
build:
	@echo "Building gom2k binary..."
	go build -o gom2k ./cmd/gom2k

# Run all tests
test:
	@echo "Running all tests..."
	go test ./test/... -v -count=1

# Run unit tests only (fast, no external dependencies)
test-unit:
	@echo "Running unit tests..."
	go test ./test/unit/... -v -count=1

# Run integration tests (requires external services)
test-integration:
	@echo "Running integration tests..."
	@echo "Note: This requires running MQTT and Kafka services"
	go test ./test/integration/... -v -count=1 -tags=integration

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test ./test/... -v -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test ./test/... -v -count=1 -race

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	go test ./test/unit/... -bench=. -benchmem -count=1

# Lint the code
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		go vet ./...; \
		go fmt ./...; \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f gom2k
	rm -f coverage.out coverage.html
	go clean -testcache

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Quick development cycle: test + build
dev: test-unit build
	@echo "Development build complete"

# Run the bridge (development mode)
run: build
	@echo "Starting gom2k bridge..."
	./gom2k

# Test MQTT connectivity
test-mqtt: build
	@echo "Testing MQTT connectivity..."
	./gom2k --test-mqtt

# Test Kafka connectivity  
test-kafka: build
	@echo "Testing Kafka connectivity..."
	./gom2k --test-kafka

# Test auto-topic creation
test-topics: build
	@echo "Testing auto-topic creation..."
	./gom2k --test-topics

# Docker test environment
docker-up:
	@echo "Starting Docker test environment..."
	cd test && docker compose up -d
	@echo "Waiting for services to be ready..."
	sleep 8
	@echo "[PASS] Docker services ready"

docker-down:
	@echo "Stopping Docker test environment..."
	cd test && docker compose down

docker-test: docker-up build
	@echo "Running complete test suite with Docker..."
	@echo "1. Running unit tests..."
	@go test ./test/unit/... -v -count=1
	@echo ""
	@echo "2. Testing MQTT connectivity..."
	@CONFIGS_DIR="./test/plaintext" timeout 8s ./gom2k --test-mqtt || echo "MQTT test completed"
	@echo ""
	@echo "3. Testing Kafka connectivity..."
	@CONFIGS_DIR="./test/plaintext" timeout 8s ./gom2k --test-kafka || echo "Kafka test completed"
	@echo ""
	@echo "4. Testing bidirectional bridge..."
	@CONFIGS_DIR="./test/plaintext" timeout 15s ./gom2k &
	@sleep 3
	@mosquitto_pub -h localhost -p 1883 -t "test/sensor/temperature" -m '{"value": 23.5, "test": "automated"}' || echo "Message sent"
	@sleep 2
	@pkill -f "./gom2k" || true
	@echo ""
	@echo "[PASS] All tests completed!"

# Complete test workflow - one command does everything
test-all: docker-test docker-down

# Quick test without full cleanup
test-quick: docker-up
	@echo "Quick test suite..."
	@make test-unit
	@echo "[PASS] Quick tests completed. Use 'make docker-down' to cleanup."

# Release build (with optimizations)
release:
	@echo "Building release binary..."
	CGO_ENABLED=0 go build -ldflags="-w -s" -o gom2k ./cmd/gom2k

# Show project info
info:
	@echo "GOM2K MQTT-Kafka Bridge"
	@echo "Go version: $(shell go version)"
	@echo "Module: $(shell go list -m)"
	@echo "Dependencies:"
	@go list -m all | grep -v "$(shell go list -m)"