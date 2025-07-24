#!/bin/bash
# Streamlined test certificate generation for GOM2K
set -e

CERTS_DIR="./certs"
mkdir -p "$CERTS_DIR"

# Skip if certificates already exist and are recent (less than 30 days old)
if [ -f "$CERTS_DIR/kafka.keystore.jks" ] && [ -f "$CERTS_DIR/mqtt-server.crt" ]; then
    if [ $(($(date +%s) - $(stat -c %Y "$CERTS_DIR/kafka.keystore.jks"))) -lt 2592000 ]; then
        echo "Using existing test certificates (generated within 30 days)"
        exit 0
    fi
fi

echo "Generating test certificates..."

# Clean up any old certificates
rm -f "$CERTS_DIR"/*

# Generate all-in-one: CA, Kafka keystores, and MQTT certificates
keytool -genkeypair -alias ca -keystore "$CERTS_DIR/ca.keystore.jks" -storepass testpass -keypass testpass -dname 'CN=Test CA,O=GOM2K Test,C=US' -keyalg RSA -keysize 2048 -validity 365 -ext BC=ca:true >/dev/null 2>&1
keytool -exportcert -alias ca -keystore "$CERTS_DIR/ca.keystore.jks" -storepass testpass -file "$CERTS_DIR/ca.crt" >/dev/null 2>&1

# Generate Kafka keystore with SAN for localhost
keytool -genkeypair -alias kafka-server -keystore "$CERTS_DIR/kafka.keystore.jks" -storepass testpass -keypass testpass -dname 'CN=localhost,O=GOM2K Test,C=US' -keyalg RSA -keysize 2048 -validity 365 -ext SAN=dns:localhost,ip:127.0.0.1 >/dev/null 2>&1

# Create Kafka truststore
keytool -importcert -alias ca -keystore "$CERTS_DIR/kafka.truststore.jks" -storepass testpass -file "$CERTS_DIR/ca.crt" -noprompt >/dev/null 2>&1

# Generate MQTT certificates (simpler approach)
openssl req -new -x509 -days 365 -nodes -out "$CERTS_DIR/mqtt-ca.crt" -keyout "$CERTS_DIR/mqtt-ca.key" -subj '/CN=MQTT Test CA/O=GOM2K Test/C=US' >/dev/null 2>&1
openssl req -new -x509 -days 365 -nodes -out "$CERTS_DIR/mqtt-server.crt" -keyout "$CERTS_DIR/mqtt-server.key" -subj '/CN=localhost/O=GOM2K Test/C=US' -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1' >/dev/null 2>&1

# Set consistent permissions
chmod 644 "$CERTS_DIR"/*

echo "Test certificates generated successfully"