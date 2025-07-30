#!/bin/bash
# Simplified test certificate generation using mkcert
set -e

CERTS_DIR="./certs"
mkdir -p "$CERTS_DIR"

# Check if mkcert is installed
if ! command -v mkcert &> /dev/null; then
    echo "Error: mkcert is required but not installed"
    echo ""
    echo "Install mkcert:"
    echo "  macOS:     brew install mkcert"
    echo "  Linux:     https://github.com/FiloSottile/mkcert#linux"
    echo "  Manual:    https://github.com/FiloSottile/mkcert#installation"
    echo ""
    exit 1
fi

# Skip if certificates already exist and are recent (less than 30 days old)
if [ -f "$CERTS_DIR/localhost+1.pem" ] && [ -f "$CERTS_DIR/kafka.keystore.jks" ]; then
    if [ $(($(date +%s) - $(stat -c %Y "$CERTS_DIR/localhost+1.pem" 2>/dev/null || stat -f %m "$CERTS_DIR/localhost+1.pem"))) -lt 2592000 ]; then
        echo "Using existing test certificates (generated within 30 days)"
        exit 0
    fi
fi

echo "Generating test certificates with mkcert..."

# Clean up any old certificates
rm -f "$CERTS_DIR"/*

# Install local CA if not already installed (prompts user if needed)
mkcert -install

# Generate certificate for localhost and 127.0.0.1
cd "$CERTS_DIR"
mkcert localhost 127.0.0.1

# Get the CA certificate location
CA_CERT=$(mkcert -CAROOT)/rootCA.pem

# Copy CA certificate for reference
cp "$CA_CERT" ca.crt

# Convert to Java KeyStore format for Kafka
# First convert to PKCS12
openssl pkcs12 -export -in localhost+1.pem -inkey localhost+1-key.pem -out server.p12 -name kafka-server -password pass:testpass

# Convert PKCS12 to JKS for Kafka
keytool -importkeystore -srckeystore server.p12 -srcstoretype PKCS12 -srcstorepass testpass -destkeystore kafka.keystore.jks -deststoretype JKS -deststorepass testpass -destkeypass testpass >/dev/null 2>&1

# Create truststore with CA certificate
keytool -importcert -alias ca -keystore kafka.truststore.jks -storepass testpass -file ca.crt -noprompt >/dev/null 2>&1

# Create symlinks for MQTT (use the same certs)
ln -sf localhost+1.pem mqtt-server.crt
ln -sf localhost+1-key.pem mqtt-server.key
ln -sf ca.crt mqtt-ca.crt

# Clean up intermediate files
rm -f server.p12

# Set consistent permissions
chmod 644 *

echo "✅ Test certificates generated successfully with mkcert"
echo "📁 CA certificate: $(mkcert -CAROOT)/rootCA.pem"
echo "🔐 Certificates are trusted by your system"